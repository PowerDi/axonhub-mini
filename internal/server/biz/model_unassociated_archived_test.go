package biz

import (
	"testing"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/stretchr/testify/require"
)

func TestFindUnassociatedChannels_IgnoresArchivedTargetForSameModel(t *testing.T) {
	channels := []*ent.Channel{
		{
			ID:              2,
			Type:            "openai",
			Name:            "backup",
			Status:          "enabled",
			SupportedModels: []string{"gpt-4"},
		},
	}

	associations := []*objects.ModelAssociation{
		{
			Type: "channel_model",
			ChannelModel: &objects.ChannelModelAssociation{
				ChannelID: 1, // archived channel is omitted from the input channel set
				ModelID:   "gpt-4",
			},
		},
	}

	require.Empty(t, findUnassociatedChannelsWithArchivedChannels(channels, associations, map[int]struct{}{1: {}}))
}
