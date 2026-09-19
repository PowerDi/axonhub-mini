package biz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

// removeChannelAssociations removes associations that point directly at a
// channel. These references live inside Model.settings JSON, so Ent cannot
// cascade them when the channel is soft-deleted.
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
