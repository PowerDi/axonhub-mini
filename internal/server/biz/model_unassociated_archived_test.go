package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
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

	require.Empty(t, findUnassociatedChannelsWithExcludedChannels(channels, associations, map[int]struct{}{1: {}}))
}

// TestFindUnassociatedChannels_IgnoresDisabledTargetForSameModel mirrors the
// archived case: a disabled channel is masked exactly like an archived one, so
// its channel_model rule still shields a sibling channel covering the same
// upstream model from being listed as unassociated.
func TestFindUnassociatedChannels_IgnoresDisabledTargetForSameModel(t *testing.T) {
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
				ChannelID: 1, // disabled channel is omitted from the input channel set
				ModelID:   "gpt-4",
			},
		},
	}

	require.Empty(t, findUnassociatedChannelsWithExcludedChannels(channels, associations, map[int]struct{}{1: {}}))
}

// TestQueryUnassociatedChannels_ExcludesDisabledChannelModels verifies the
// end-to-end behavior: a disabled channel's own added-but-unassociated models
// must NOT surface in the unassociated view, while an enabled channel's do. The
// disabled channel's models stay on the row untouched, so re-enabling it brings
// them back with no cleanup needed.
func TestQueryUnassociatedChannels_ExcludesDisabledChannelModels(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)
	svc := &ModelService{
		AbstractService: &AbstractService{
			db: client,
		},
	}

	// Enabled channel with a model that has no association -> should surface.
	_, err := client.Channel.Create().
		SetType("openai").
		SetName("Enabled Channel").
		SetStatus("enabled").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-enabled"}}).
		SetSupportedModels([]string{"gpt-4-enabled"}).
		SetDefaultTestModel("gpt-4-enabled").
		Save(ctx)
	require.NoError(t, err)

	// Disabled channel with a model that has no association -> should be masked.
	disabledChannel, err := client.Channel.Create().
		SetType("openai").
		SetName("Disabled Channel").
		SetStatus("disabled").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-disabled"}}).
		SetSupportedModels([]string{"gpt-4-disabled"}).
		SetDefaultTestModel("gpt-4-disabled").
		Save(ctx)
	require.NoError(t, err)

	result, err := svc.QueryUnassociatedChannels(ctx)
	require.NoError(t, err)

	// The disabled channel must not appear at all.
	for _, uc := range result {
		require.NotEqual(t, disabledChannel.ID, uc.Channel.ID,
			"disabled channel should not appear in the unassociated view")
	}

	// The enabled channel's unassociated model is still reported.
	require.Len(t, result, 1)
	require.Equal(t, []string{"gpt-4-enabled"}, result[0].Models)
}
