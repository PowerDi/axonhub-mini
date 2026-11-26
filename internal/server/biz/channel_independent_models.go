// 职责: 生成渠道独立模型入口、显示名称，并校验独立入口命名空间的归属与冲突。
// 关联文件:
//
//	internal/objects/channel.go
//	internal/server/biz/channel.go
//	internal/server/biz/channel_settings.go
//	internal/server/biz/model.go
//	internal/server/orchestrator/candidates.go
//
// 遵循的 Rules:
//
//	.agents/rules/biz-services.md
//	.agents/rules/cache-compat.md
//
// 注意事项:
//
//	独立入口复用 GetModelEntries 的有效前缀映射，不修改其缓存或放开既有隐藏规则。
//	独立前缀保留整个命名空间，停用渠道仍保留归属；显示名称变化不改变调用 ID。
package biz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/scopes"
)

var ErrIndependentModelConflict = errors.New("independent channel model entry conflict")

// IndependentModelPrefix returns the reserved, request-side namespace. Only
// request IDs follow LowercaseModelID; upstream model IDs and labels never do.
func (ch *Channel) IndependentModelPrefix() string {
	if ch.Settings == nil || !ch.Settings.IndependentListing {
		return ""
	}
	prefix := ch.Settings.ExtraModelPrefix
	if ch.Settings.LowercaseModelID {
		prefix = strings.ToLower(prefix)
	}
	return prefix
}

// IndependentModelEntries returns a projection of the existing prefixed
// mappings. Bare, trimmed and user-mapped aliases are not independently listed.
func (ch *Channel) IndependentModelEntries() map[string]ChannelModelEntry {
	entries := make(map[string]ChannelModelEntry)
	prefix := ch.IndependentModelPrefix()
	if prefix == "" {
		return entries
	}
	for id, entry := range ch.GetModelEntries() {
		if entry.Source == "prefix" && independentNamespaceContains(prefix, id) {
			entries[id] = entry
		}
	}
	return entries
}

func (ch *Channel) IndependentModelDisplayName(entry ChannelModelEntry) string {
	label := strings.TrimSpace(ch.Settings.IndependentDisplayName)
	if label == "" {
		label = ch.Name
	}
	return entry.ActualModel + " · " + label
}

// ResolveIndependentModelEntry claims an entire independent namespace, even
// when its suffix is unsupported or hidden. A claimed request must never fall
// through to another channel or a configured aggregate with the same ID.
func ResolveIndependentModelEntry(channels []*Channel, id string) (*Channel, ChannelModelEntry, bool, error) {
	var owner *Channel
	for _, ch := range channels {
		prefix := ch.IndependentModelPrefix()
		if prefix == "" || !independentNamespaceContains(prefix, id) {
			continue
		}
		if owner != nil && owner.ID != ch.ID {
			return nil, ChannelModelEntry{}, true, fmt.Errorf("%w: model %q belongs to multiple channels", ErrIndependentModelConflict, id)
		}
		owner = ch
	}
	if owner == nil {
		return nil, ChannelModelEntry{}, false, nil
	}
	for _, ch := range channels {
		if ch.ID == owner.ID {
			continue
		}
		if _, exists := ch.GetModelEntries()[id]; exists {
			return nil, ChannelModelEntry{}, true, fmt.Errorf("%w: model %q is also exposed by channel %q", ErrIndependentModelConflict, id, ch.Name)
		}
	}
	entry := owner.GetModelEntries()[id]
	if entry.Source != "prefix" {
		entry = ChannelModelEntry{}
	}
	return owner, entry, true, nil
}

func independentNamespaceContains(prefix, id string) bool {
	return strings.HasPrefix(id, prefix+"/")
}

// RejectUnresolvedIndependentNamespace prevents disabled or not-yet-loaded
// owners from handing their IDs to legacy fallback. Bare IDs need no host read.
func (svc *ChannelService) RejectUnresolvedIndependentNamespace(ctx context.Context, id string) error {
	if !strings.Contains(id, "/") {
		return nil
	}
	if err := authz.RequireScope(ctx, scopes.ScopeReadChannels); err != nil {
		return err
	}
	prefixes, err := loadIndependentModelPrefixes(ctx, svc.entFromContext(ctx))
	if err != nil {
		return err
	}
	for _, prefix := range prefixes {
		if independentNamespaceContains(prefix, id) {
			return fmt.Errorf("%w: independent channel for %q is unavailable", ErrInvalidModel, id)
		}
	}
	return nil
}

func loadIndependentModelPrefixes(ctx context.Context, client *ent.Client) ([]string, error) {
	return authz.RunAsHost(ctx, client, "read-independent-model-namespaces", func(hostCtx context.Context) ([]string, error) {
		entities, err := client.Channel.Query().Select(channel.FieldID, channel.FieldSettings).All(hostCtx)
		if err != nil {
			return nil, fmt.Errorf("load independent channel namespaces: %w", err)
		}
		var prefixes []string
		for _, entity := range entities {
			if prefix := (&Channel{Channel: entity}).IndependentModelPrefix(); prefix != "" {
				prefixes = append(prefixes, prefix)
			}
		}
		return prefixes, nil
	})
}

// validateIndependentChannelModels also guards non-independent channels from
// creating aliases inside another channel's reserved namespace. These host
// reads include disabled/archived entities: enabling them must not steal IDs.
func (svc *ChannelService) validateIndependentChannelModels(ctx context.Context, proposed *ent.Channel) error {
	if err := authz.RequireScope(ctx, scopes.ScopeWriteChannels); err != nil {
		return err
	}
	candidate := &Channel{Channel: proposed}
	prefix := candidate.IndependentModelPrefix()
	if proposed.Settings != nil && proposed.Settings.IndependentListing {
		if prefix == "" || prefix != strings.TrimSpace(prefix) || strings.HasPrefix(prefix, "/") || strings.HasSuffix(prefix, "/") || strings.ContainsFunc(prefix, unicode.IsControl) {
			return fmt.Errorf("independent channel models require a non-empty extra model prefix without leading/trailing slashes or control characters")
		}
		for id, entry := range candidate.GetModelEntries() {
			if independentNamespaceContains(prefix, id) && entry.Source != "prefix" {
				return fmt.Errorf("%w: channel alias %q overlaps its independent prefix", ErrIndependentModelConflict, id)
			}
		}
	}

	client := svc.entFromContext(ctx)
	_, err := authz.RunAsHost(ctx, client, "validate-independent-channel-models", func(hostCtx context.Context) (bool, error) {
		others, err := client.Channel.Query().Where(channel.IDNEQ(proposed.ID)).
			Select(channel.FieldID, channel.FieldName, channel.FieldType, channel.FieldSettings, channel.FieldSupportedModels).
			All(hostCtx)
		if err != nil {
			return false, fmt.Errorf("load channel model namespaces: %w", err)
		}
		for _, other := range others {
			ch := &Channel{Channel: other}
			otherPrefix := ch.IndependentModelPrefix()
			if prefix != "" && otherPrefix != "" && (prefix == otherPrefix || independentNamespaceContains(prefix, otherPrefix) || independentNamespaceContains(otherPrefix, prefix)) {
				return false, fmt.Errorf("%w: prefix %q overlaps independent channel %q", ErrIndependentModelConflict, prefix, ch.Name)
			}
			if otherPrefix != "" {
				for id := range candidate.GetModelEntries() {
					if independentNamespaceContains(otherPrefix, id) {
						return false, fmt.Errorf("%w: model %q belongs to independent channel %q", ErrIndependentModelConflict, id, ch.Name)
					}
				}
			}
			if prefix != "" {
				for id := range ch.GetModelEntries() {
					if independentNamespaceContains(prefix, id) {
						return false, fmt.Errorf("%w: prefix %q overlaps channel %q model %q", ErrIndependentModelConflict, prefix, ch.Name, id)
					}
				}
			}
		}
		if prefix != "" {
			exists, err := client.Model.Query().Where(model.ModelIDHasPrefix(prefix + "/")).Exist(hostCtx)
			if err != nil {
				return false, fmt.Errorf("check configured model namespace: %w", err)
			}
			if exists {
				return false, fmt.Errorf("%w: prefix %q is already used by configured models", ErrIndependentModelConflict, prefix)
			}
		}
		return true, nil
	})
	return err
}

// validateIndependentModelIDs covers all Model-creation paths, including
// imports. Reserving the prefix, rather than today's supported-model set,
// prevents future channel model syncs from silently shadowing an aggregate.
func validateIndependentModelIDs(ctx context.Context, client *ent.Client, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := authz.RequireScope(ctx, scopes.ScopeWriteChannels); err != nil {
		return err
	}
	_, err := authz.RunAsHost(ctx, client, "validate-configured-model-namespaces", func(hostCtx context.Context) (bool, error) {
		channels, err := client.Channel.Query().Select(channel.FieldID, channel.FieldName, channel.FieldSettings).All(hostCtx)
		if err != nil {
			return false, fmt.Errorf("load independent channel namespaces: %w", err)
		}
		for _, entity := range channels {
			ch := &Channel{Channel: entity}
			prefix := ch.IndependentModelPrefix()
			if prefix == "" {
				continue
			}
			for _, id := range ids {
				if independentNamespaceContains(prefix, id) {
					return false, fmt.Errorf("%w: model %q belongs to independent channel %q", ErrIndependentModelConflict, id, ch.Name)
				}
			}
		}
		return true, nil
	})
	return err
}

// checkIndependentEntryConflicts fails closed if out-of-band writes or a
// concurrent edit left a published ID occupied by any configured Model.
func checkIndependentEntryConflicts(ctx context.Context, client *ent.Client, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := authz.RunAsHost(ctx, client, "check-independent-entry-conflicts", func(hostCtx context.Context) (bool, error) {
		exists, err := client.Model.Query().Where(model.ModelIDIn(ids...)).Exist(hostCtx)
		if err != nil {
			return false, fmt.Errorf("check independent model IDs: %w", err)
		}
		if exists {
			return false, fmt.Errorf("%w: an independent entry is already used by a configured model", ErrIndependentModelConflict)
		}
		return true, nil
	})
	return err
}
