package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

// An association to one channel must never hide another channel's same-name
// model, regardless of the target channel's status or the rule's disabled flag.
func TestQueryUnassociatedChannels_AssociationsAreChannelScoped(t *testing.T) {
	for _, status := range []channel.Status{channel.StatusEnabled, channel.StatusDisabled, channel.StatusArchived} {
		t.Run(string(status), func(t *testing.T) {
			for _, rule := range []struct {
				name     string
				disabled bool
			}{
				{name: "active_rule"},
				{name: "disabled_rule", disabled: true},
			} {
				t.Run(rule.name, func(t *testing.T) {
					client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
					defer client.Close()

					ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
					svc := &ModelService{AbstractService: &AbstractService{db: client}}

					target, err := client.Channel.Create().
						SetType("openai").
						SetName("target").
						SetStatus(status).
						SetCredentials(objects.ChannelCredentials{}).
						SetSupportedModels([]string{"gpt-6-astra", "target-only"}).
						SetManualModels([]string{"target-only"}).
						SetDefaultTestModel("gpt-6-astra").
						Save(ctx)
					require.NoError(t, err)

					backup, err := client.Channel.Create().
						SetType("openai").
						SetName("opus公益站").
						SetStatus(channel.StatusEnabled).
						SetCredentials(objects.ChannelCredentials{}).
						SetSupportedModels([]string{"gpt-6-astra", "gpt-6.1-sol"}).
						SetDefaultTestModel("gpt-6-astra").
						Save(ctx)
					require.NoError(t, err)

					_, err = client.Model.Create().
						SetDeveloper("openai").
						SetModelID("gpt-6-astra").
						SetName("GPT-6 Astra").
						SetGroup("gpt").
						SetIcon("icon").
						SetModelCard(&objects.ModelCard{}).
						SetStatus("enabled").
						SetSettings(&objects.ModelSettings{Associations: []*objects.ModelAssociation{
							{
								Type:     "channel_model",
								Disabled: rule.disabled,
								ChannelModel: &objects.ChannelModelAssociation{
									ChannelID: target.ID,
									ModelID:   "gpt-6-astra",
								},
							},
						}}).
						Save(ctx)
					require.NoError(t, err)

					result, err := svc.QueryUnassociatedChannels(ctx)
					require.NoError(t, err)
					byChannel := make(map[int][]string)
					for _, info := range result {
						byChannel[info.Channel.ID] = info.Models
					}
					require.ElementsMatch(t, []string{"gpt-6-astra", "gpt-6.1-sol"}, byChannel[backup.ID])
					if status == channel.StatusEnabled {
						expected := []string{"target-only"}
						if rule.disabled {
							expected = append(expected, "gpt-6-astra")
						}
						require.ElementsMatch(t, expected, byChannel[target.ID])
						require.Len(t, result, 2)
					} else {
						require.NotContains(t, byChannel, target.ID)
						require.Len(t, result, 1)
					}

					persisted, err := client.Channel.Get(ctx, target.ID)
					require.NoError(t, err)
					require.Equal(t, target.SupportedModels, persisted.SupportedModels)
					require.Equal(t, target.ManualModels, persisted.ManualModels)
				})
			}
		})
	}
}

// Request-side aliases sharing an upstream model remain separate association
// keys, and associating an alias on one channel does not cover another channel.
func TestFindUnassociatedChannels_RequestModelKey(t *testing.T) {
	channels := []*ent.Channel{
		{
			ID:              1,
			Type:            "openai",
			Name:            "primary",
			Status:          "enabled",
			SupportedModels: []string{"upstream-model"},
			Settings: &objects.ChannelSettings{
				ModelMappings: []objects.ModelMapping{
					{From: "alias-a", To: "upstream-model"},
					{From: "alias-b", To: "upstream-model"},
				},
				HideMappedModels: true,
			},
		},
		{
			ID:              2,
			Type:            "openai",
			Name:            "backup",
			Status:          "enabled",
			SupportedModels: []string{"alias-a", "alias-b"},
		},
	}
	associations := []*objects.ModelAssociation{
		{
			Type: "channel_model",
			ChannelModel: &objects.ChannelModelAssociation{
				ChannelID: 1,
				ModelID:   "alias-a",
			},
		},
	}

	result := findUnassociatedChannels(channels, associations)
	require.Len(t, result, 2)
	byChannel := make(map[int][]string)
	for _, info := range result {
		byChannel[info.Channel.ID] = info.Models
	}
	require.Equal(t, []string{"alias-b"}, byChannel[1])
	require.ElementsMatch(t, []string{"alias-a", "alias-b"}, byChannel[2])
}

// TestFindUnassociatedChannels_IgnoresDisabledRuleOnExcludedChannel verifies a
// disabled channel_model rule stays inert even when it points at a disabled or
// archived channel. Otherwise it masquerades as a global association and hides
// the model on unrelated enabled channels.
func TestFindUnassociatedChannels_IgnoresDisabledRuleOnExcludedChannel(t *testing.T) {
	channels := []*ent.Channel{
		{
			ID:              50,
			Type:            "openai",
			Name:            "unrelated",
			Status:          "enabled",
			SupportedModels: []string{"glm-5.3-flash"},
		},
	}

	associations := []*objects.ModelAssociation{
		{
			Type:     "channel_model",
			Disabled: true,
			ChannelModel: &objects.ChannelModelAssociation{
				ChannelID: 18,
				ModelID:   "glm-5.3-flash",
			},
		},
	}

	result := findUnassociatedChannels(channels, associations)
	require.Len(t, result, 1)
	require.Equal(t, []string{"glm-5.3-flash"}, result[0].Models)
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
