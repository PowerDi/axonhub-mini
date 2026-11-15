package biz

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/objects"
)

// removeChannelAssociations removes associations that point directly at a
// channel. These references live inside model and developer settings JSON, so
// Ent cannot cascade them when the channel is soft-deleted.
func (svc *ChannelService) removeChannelAssociations(ctx context.Context, channelIDs []int) error {
	if len(channelIDs) == 0 {
		return nil
	}

	removed := make(map[int]struct{}, len(channelIDs))
	for _, id := range channelIDs {
		removed[id] = struct{}{}
	}

	models, err := svc.entFromContext(ctx).Model.Query().
		Where(model.DeletedAtEQ(0)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query models while deleting channels: %w", err)
	}

	for _, m := range models {
		settings, changed := pruneDirectChannelAssociations(m.Settings, removed)
		if !changed {
			continue
		}

		if _, err := svc.entFromContext(ctx).Model.UpdateOneID(m.ID).
			SetSettings(settings).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to remove deleted channel associations from model %d: %w", m.ID, err)
		}
	}

	return svc.removeDeveloperChannelAssociations(ctx, removed)
}

func (svc *ChannelService) removeDeveloperChannelAssociations(ctx context.Context, removed map[int]struct{}) error {
	row, err := svc.SystemService.querySystemRow(ctx, SystemKeyModelSettings)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query developer settings while deleting channels: %w", err)
	}

	// Read from the deletion transaction and replace only developer_settings,
	// preserving other system settings without normalizing unrelated rules.
	var settings map[string]json.RawMessage
	if err := json.Unmarshal([]byte(row.Value), &settings); err != nil {
		return fmt.Errorf("failed to decode model settings while deleting channels: %w", err)
	}
	data, ok := settings["developer_settings"]
	if !ok {
		return nil
	}
	var developers []*DeveloperModelSettings
	if err := json.Unmarshal(data, &developers); err != nil {
		return fmt.Errorf("failed to decode developer settings while deleting channels: %w", err)
	}

	changed := false
	for _, developer := range developers {
		if developer == nil {
			continue
		}
		pruned, updated := pruneDirectChannelAssociations(&objects.ModelSettings{Associations: developer.Associations}, removed)
		if updated {
			developer.Associations = pruned.Associations
			changed = true
		}
	}
	if !changed {
		return nil
	}

	settings["developer_settings"], err = json.Marshal(developers)
	if err != nil {
		return fmt.Errorf("failed to encode developer settings while deleting channels: %w", err)
	}
	value, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to encode model settings while deleting channels: %w", err)
	}
	_, err = authz.RunAsHost(ctx, svc.entFromContext(ctx), "deleted-channel-associations-cleanup", func(hostCtx context.Context) (*ent.System, error) {
		// Abort deletion if a concurrent settings edit replaced the snapshot.
		return ent.FromContext(hostCtx).System.UpdateOneID(row.ID).
			Where(system.ValueEQ(row.Value)).
			SetValue(string(value)).
			Save(hostCtx)
	})
	if err != nil {
		return fmt.Errorf("failed to remove deleted channel associations from developer settings: %w", err)
	}

	// Register the commit hook on the original transaction context, which the
	// isolated host operation does not retain.
	svc.SystemService.invalidateSystemValueCache(ctx, SystemKeyModelSettings)
	return nil
}

func pruneDirectChannelAssociations(settings *objects.ModelSettings, removed map[int]struct{}) (*objects.ModelSettings, bool) {
	if settings == nil || len(settings.Associations) == 0 {
		return settings, false
	}

	associations := make([]*objects.ModelAssociation, 0, len(settings.Associations))
	changed := false
	for _, association := range settings.Associations {
		if association == nil {
			associations = append(associations, nil)
			continue
		}

		remove := false
		switch association.Type {
		case "channel_model":
			remove = association.ChannelModel != nil && hasRemovedChannel(association.ChannelModel.ChannelID, removed)
		case "channel_regex":
			remove = association.ChannelRegex != nil && hasRemovedChannel(association.ChannelRegex.ChannelID, removed)
		}

		if remove {
			changed = true
			continue
		}

		associations = append(associations, association)
	}

	if !changed {
		return settings, false
	}

	updated := *settings
	updated.Associations = associations

	return &updated, true
}

func hasRemovedChannel(channelID int, removed map[int]struct{}) bool {
	_, ok := removed[channelID]
	return ok
}
