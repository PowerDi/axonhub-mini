package biz

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// ValidateModelAPIFormatPolicies validates the structural shape of per-model
// endpoint policies: non-empty model names, known api_formats, and no
// duplicate model entries. It does not check routability —
// ValidateChannelModelRoutability does that.
func ValidateModelAPIFormatPolicies(settings *objects.ChannelSettings) error {
	if settings == nil || len(settings.ModelAPIFormatPolicies) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(settings.ModelAPIFormatPolicies))
	for i, policy := range settings.ModelAPIFormatPolicies {
		if policy.Model == "" {
			return fmt.Errorf("model api format policy[%d]: model is required", i)
		}

		if _, ok := seen[policy.Model]; ok {
			return fmt.Errorf("model api format policy[%d]: duplicate policy for model %q", i, policy.Model)
		}

		seen[policy.Model] = struct{}{}

		for _, field := range [...]struct {
			name   string
			values []string
		}{
			{"exclude", policy.Exclude},
			{"allow", policy.Allow},
		} {
			for _, format := range field.values {
				if _, ok := SupportedAPIFormats[format]; !ok {
					return fmt.Errorf("model api format policy for %q: unsupported api_format %q in %s", policy.Model, format, field.name)
				}
			}
		}
	}

	return nil
}

// modelRoutabilityIssue describes a model that loses its last usable
// endpoint combination on a channel after an edit.
type modelRoutabilityIssue struct {
	Model       string
	RequestType llm.RequestType
}

func (issue modelRoutabilityIssue) String() string {
	return fmt.Sprintf("%s (type %s)", issue.Model, issue.RequestType)
}

// entChannel aliases the raw ent channel entity so edit-time validation can
// run before any outbound construction.
type entChannel = ent.Channel

// policyFiltersAll reports whether the policy leaves no endpoint that can
// serve requestType. Policies absent for a model never filter.
func policyFiltersAll(endpoints []objects.ChannelEndpoint, policy *objects.ModelAPIFormatPolicy, requestType llm.RequestType) bool {
	if policy == nil {
		return false
	}

	capable := llm.CapableAPIFormats(requestType)
	if capable == nil {
		// Unknown request types rely on any endpoint (mirrors SelectAPIFormat's
		// fallback to the first endpoint), so only a policy that filters every
		// endpoint is fatal.
		for _, ep := range endpoints {
			if policy.AllowsAPIFormat(ep.APIFormat) {
				return false
			}
		}

		return len(endpoints) > 0
	}

	for _, ep := range endpoints {
		if _, ok := capable[ep.APIFormat]; !ok {
			continue
		}

		if policy.AllowsAPIFormat(ep.APIFormat) {
			return false
		}
	}

	return true
}

// validateUpdateModelRoutability computes the post-update effective state of
// the channel (input values win over stored ones) and runs the routability
// check against it. Skipped entirely when neither settings nor endpoints nor
// type is being touched, or when the resulting settings carry no policies.
func (svc *ChannelService) validateUpdateModelRoutability(ctx context.Context, id int, input *ent.UpdateChannelInput) error {
	if input.Settings == nil && input.Endpoints == nil && input.Type == nil {
		return nil
	}

	existing, err := svc.entFromContext(ctx).Channel.Query().Where(channel.IDEQ(id)).
		Select(channel.FieldType, channel.FieldStatus, channel.FieldEndpoints, channel.FieldSettings).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to load channel for routability validation: %w", err)
	}

	effectiveType := existing.Type
	if input.Type != nil {
		effectiveType = *input.Type
	}

	effectiveEndpoints := existing.Endpoints
	if input.Endpoints != nil {
		effectiveEndpoints = input.Endpoints
	}

	effectiveSettings := existing.Settings
	if input.Settings != nil {
		effectiveSettings = input.Settings
	}

	return svc.ValidateChannelModelRoutability(ctx, id, effectiveType, existing.Status, effectiveEndpoints, effectiveSettings)
}

// ValidateChannelModelRoutability is the edit-time bidirectional check for
// per-model endpoint policies. It rejects a channel edit (settings and/or
// endpoints) when a model with a policy ends up with zero usable endpoints on
// this channel AND no other enabled channel can serve that model with a
// usable endpoint. Both directions share one code path because the failure
// condition is symmetric: an endpoint removal starves an existing policy just
// like a policy edit starves existing endpoints.
//
// effectiveStatus gates the check: a channel that is not enabled carries no
// traffic, so a starving configuration on it does not change current
// routability — edits to disabled channels are not blocked (cleanup must
// stay possible). Create paths pass StatusEnabled: a new channel is created
// with intent to serve, so it is validated as enabled even though channels
// start disabled in the schema.
//
// For a new channel (existingID == 0) other channels are checked as-is.
// For an update, the in-memory effective state (settings/endpoints) of the
// edited channel replaces its stored state.
func (svc *ChannelService) ValidateChannelModelRoutability(
	ctx context.Context,
	existingID int,
	effectiveType channel.Type,
	effectiveStatus channel.Status,
	effectiveEndpoints []objects.ChannelEndpoint,
	effectiveSettings *objects.ChannelSettings,
) error {
	if effectiveSettings == nil || len(effectiveSettings.ModelAPIFormatPolicies) == 0 {
		return nil
	}

	if effectiveStatus != channel.StatusEnabled {
		return nil
	}

	// Resolve this channel's effective endpoints.
	endpoints := mergeEndpoints(DefaultEndpointsForChannelType(effectiveType), effectiveEndpoints)

	// Model entities carry the request type; a model without an entity is
	// still validated with the permissive "" request type (any capable endpoint).
	modelTypes := svc.loadModelRequestTypes(ctx)

	otherChannels := svc.loadOtherEnabledChannels(ctx, existingID)

	var issues []string
	for _, policy := range effectiveSettings.ModelAPIFormatPolicies {
		if policyFiltersAll(endpoints, &policy, modelTypes[policy.Model]) {
			// Check if another enabled channel serves this model with a
			// usable endpoint. Policies on other channels count too: the
			// fallback needs a genuinely usable path, not a starved one.
			if !otherChannelServesModel(otherChannels, policy.Model, modelTypes[policy.Model]) {
				issues = append(issues, fmt.Sprintf("model %q has no usable endpoint left on this channel and no other enabled channel serves it", policy.Model))
			}
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("channel update would make models unroutable: %s", joinStrings(issues, "; "))
	}

	return nil
}

// loadModelRequestTypes returns modelID -> request type for every Model
// entity, so policies can be checked against the model's real request type.
func (svc *ChannelService) loadModelRequestTypes(ctx context.Context) map[string]llm.RequestType {
	models, err := svc.entFromContext(ctx).Model.Query().All(ctx)
	if err != nil {
		// A lookup failure must not block the edit; fall back to the
		// permissive request type.
		return nil
	}

	types := make(map[string]llm.RequestType, len(models))
	for _, m := range models {
		types[m.ModelID] = llm.RequestTypeForModelType(m.Type.String())
	}

	return types
}

// loadOtherEnabledChannels loads every enabled channel except existingID
// (0 keeps all). Returns raw ent entities; policies and endpoints are read
// without building outbounds.
func (svc *ChannelService) loadOtherEnabledChannels(ctx context.Context, existingID int) []*entChannel {
	channels, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusEQ(channel.StatusEnabled)).
		All(ctx)
	if err != nil {
		return nil
	}

	return slices.DeleteFunc(channels, func(ch *entChannel) bool {
		return ch.ID == existingID
	})
}

// otherChannelServesModel reports whether any channel serves the model with
// at least one usable endpoint under the channel's own per-model policy.
func otherChannelServesModel(channels []*entChannel, model string, requestType llm.RequestType) bool {
	for _, chEnt := range channels {
		ch := &Channel{Channel: chEnt}
		entries := ch.GetModelEntries()
		entry, ok := entries[model]
		if !ok && chEnt.Settings != nil && chEnt.Settings.LowercaseModelID {
			// A fallback channel with lowercase model matching exposes its
			// entries under lowercased keys; a mixed-case policy key must
			// still find them, otherwise a valid fallback is silently
			// discarded and the edit wrongly rejected.
			entry, ok = entries[strings.ToLower(model)]
		}
		if !ok {
			continue
		}

		endpoints := mergeEndpoints(DefaultEndpointsForChannelType(chEnt.Type), chEnt.Endpoints)
		if !policyFiltersAll(endpoints, entry.Policy, requestType) {
			return true
		}
	}

	return false
}

func joinStrings(values []string, sep string) string {
	return strings.Join(values, sep)
}
