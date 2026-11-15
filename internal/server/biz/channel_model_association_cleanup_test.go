package biz

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPruneDirectChannelAssociations(t *testing.T) {
	settings := &objects.ModelSettings{Associations: []*objects.ModelAssociation{
		{
			Type: "channel_model",
			ChannelModel: &objects.ChannelModelAssociation{
				ChannelID: 10,
				ModelID:   "gpt-4o",
			},
		},
		{
			Type: "channel_regex",
			ChannelRegex: &objects.ChannelRegexAssociation{
				ChannelID: 11,
				Pattern:   "^gpt-4",
			},
		},
		{
			Type:    "model",
			ModelID: &objects.ModelIDAssociation{ModelID: "gpt-4o"},
		},
	}}

	updated, changed := pruneDirectChannelAssociations(settings, map[int]struct{}{10: {}})
	require.True(t, changed)
	require.Len(t, updated.Associations, 2)
	require.Equal(t, "channel_regex", updated.Associations[0].Type)
	require.Equal(t, "model", updated.Associations[1].Type)
	require.Len(t, settings.Associations, 3, "the source settings must not be mutated")
}

func TestPruneDirectChannelAssociationsKeepsUnrelatedRules(t *testing.T) {
	settings := &objects.ModelSettings{Associations: []*objects.ModelAssociation{
		{Type: "channel_model", ChannelModel: &objects.ChannelModelAssociation{ChannelID: 10, ModelID: "gpt-4o"}},
	}}

	updated, changed := pruneDirectChannelAssociations(settings, map[int]struct{}{11: {}})
	require.False(t, changed)
	require.Same(t, settings, updated)
}

type channelAssociationCleanupFixture struct {
	svc         *ChannelService
	ctx         context.Context
	client      *ent.Client
	channels    []*ent.Channel
	model       *ent.Model
	systemValue string
}

func setupChannelAssociationCleanup(t *testing.T) channelAssociationCleanupFixture {
	t.Helper()
	svc, client := setupTestChannelService(t)
	t.Cleanup(func() { client.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	channels := []*ent.Channel{
		createImportChannel(t, ctx, client, "delete-first", []string{"upstream"}),
		createImportChannel(t, ctx, client, "delete-second", []string{"upstream"}),
		createImportChannel(t, ctx, client, "keep", []string{"upstream"}),
	}
	rules := []*objects.ModelAssociation{
		channelModelAssoc(channels[0].ID, "upstream", 0),
		{Type: "channel_regex", Disabled: true, ChannelRegex: &objects.ChannelRegexAssociation{ChannelID: channels[0].ID, Pattern: "^upstream$"}},
		channelModelAssoc(channels[1].ID, "upstream", 1),
		channelModelAssoc(channels[2].ID, "upstream", 2),
		{Type: "model", ModelID: &objects.ModelIDAssociation{ModelID: "upstream", Exclude: []*objects.ExcludeAssociation{{ChannelIds: []int{channels[0].ID}}}}},
		{Type: "regex", Regex: &objects.RegexAssociation{Pattern: "^upstream$"}},
		{Type: "channel_tags_model", Disabled: true, ChannelTagsModel: &objects.ChannelTagsModelAssociation{ChannelTags: []string{"shared"}, ModelID: "upstream"}},
		{Type: "channel_tags_regex", ChannelTagsRegex: &objects.ChannelTagsRegexAssociation{ChannelTags: []string{"shared"}, Pattern: "^upstream$"}},
	}
	m := createImportModel(t, ctx, client, "logical-model", model.StatusEnabled, rules)
	developers := []*DeveloperModelSettings{
		{Developer: "openai", Associations: []*objects.ModelAssociation{
			channelModelAssoc(channels[0].ID, "", 0),
			channelModelAssoc(channels[1].ID, "", 1),
			{Type: "channel_model", Disabled: true, Priority: 2, ChannelModel: &objects.ChannelModelAssociation{ChannelID: channels[2].ID}},
			{Type: "channel_tags_model", Priority: 3, ChannelTagsModel: &objects.ChannelTagsModelAssociation{ChannelTags: []string{"shared"}}},
		}},
		{Developer: "only-deleted", Associations: []*objects.ModelAssociation{channelModelAssoc(channels[0].ID, "", 0)}},
		nil,
	}
	value, err := json.Marshal(map[string]any{
		"developer_settings":       developers,
		"query_all_channel_models": true,
		"model_blacklist_regex":    "^private$",
		"other_setting":            map[string]any{"keep": []string{"value"}},
	})
	require.NoError(t, err)
	require.NoError(t, svc.SystemService.setSystemValue(ctx, SystemKeyModelSettings, string(value)))
	// Populate the pre-deletion cache as a normal request would.
	_, err = svc.SystemService.ModelSettings(ctx)
	require.NoError(t, err)
	return channelAssociationCleanupFixture{svc, ctx, client, channels, m, string(value)}
}

func TestChannelDeletionCleansModelAndDeveloperAssociations(t *testing.T) {
	for _, bulk := range []bool{false, true} {
		name := "single"
		if bulk {
			name = "bulk"
		}
		t.Run(name, func(t *testing.T) {
			f := setupChannelAssociationCleanup(t)
			models := []*ent.Model{f.model}
			for _, status := range []model.Status{model.StatusDisabled, model.StatusArchived} {
				models = append(models, createImportModel(t, f.ctx, f.client, string(status), status, f.model.Settings.Associations))
			}
			// Another request can cache the old committed row before our transaction commits.
			cacheRefilled := false
			f.client.Channel.Use(func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
					cacheRefilled = true
					require.NoError(t, f.svc.SystemService.Cache.Set(ctx, "system:"+SystemKeyModelSettings, ent.System{Value: f.systemValue}))
					return next.Mutate(ctx, mutation)
				})
			})
			firstKeptRule := 2
			firstKeptDeveloperRule := 1
			if bulk {
				require.NoError(t, f.svc.BulkDeleteChannels(f.ctx, []int{f.channels[0].ID, f.channels[1].ID}))
				firstKeptRule = 3
				firstKeptDeveloperRule = 2
			} else {
				require.NoError(t, f.svc.DeleteChannel(f.ctx, f.channels[0].ID))
			}
			require.True(t, cacheRefilled)
			for _, m := range models {
				updated, err := f.client.Model.Get(f.ctx, m.ID)
				require.NoError(t, err)
				expected := *m.Settings
				expected.Associations = m.Settings.Associations[firstKeptRule:]
				require.Equal(t, &expected, updated.Settings)
			}
			for i, ch := range f.channels {
				_, err := f.client.Channel.Get(f.ctx, ch.ID)
				if i == 0 || (bulk && i == 1) {
					require.True(t, ent.IsNotFound(err))
				} else {
					require.NoError(t, err)
				}
			}
			var original SystemModelSettings
			require.NoError(t, json.Unmarshal([]byte(f.systemValue), &original))
			settings, err := f.svc.SystemService.ModelSettings(f.ctx)
			require.NoError(t, err)
			require.Equal(t, original.DeveloperSettings[0].Associations[firstKeptDeveloperRule:], settings.DeveloperSettings[0].Associations)
			require.Equal(t, "only-deleted", settings.DeveloperSettings[1].Developer)
			require.Empty(t, settings.DeveloperSettings[1].Associations)
			require.Nil(t, settings.DeveloperSettings[2])
			value, err := f.svc.SystemService.getSystemValue(f.ctx, SystemKeyModelSettings)
			require.NoError(t, err)
			var before, after map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(f.systemValue), &before))
			require.NoError(t, json.Unmarshal([]byte(value), &after))
			delete(before, "developer_settings")
			delete(after, "developer_settings")
			require.Equal(t, before, after, "unrelated system settings must survive cleanup")
		})
	}
}

func TestChannelDeletionRollsBackAssociations(t *testing.T) {
	for _, failure := range []string{"developer-write", "channel-delete", "outer-transaction"} {
		t.Run(failure, func(t *testing.T) {
			f := setupChannelAssociationCleanup(t)
			failureErr := errors.New("injected cleanup failure")
			reject := func(ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(context.Context, ent.Mutation) (ent.Value, error) {
					return nil, failureErr
				})
			}
			var err error
			switch failure {
			case "developer-write":
				f.client.System.Use(reject)
				err = f.svc.DeleteChannel(f.ctx, f.channels[0].ID)
			case "channel-delete":
				f.client.Channel.Use(reject)
				err = f.svc.BulkDeleteChannels(f.ctx, []int{f.channels[0].ID, f.channels[1].ID})
			case "outer-transaction":
				err = f.svc.RunInTransaction(f.ctx, func(txCtx context.Context) error {
					require.NoError(t, f.svc.DeleteChannel(txCtx, f.channels[0].ID))
					settings, readErr := f.svc.SystemService.ModelSettings(txCtx)
					require.NoError(t, readErr)
					require.Len(t, settings.DeveloperSettings[0].Associations, 3)
					return failureErr
				})
			}
			require.ErrorIs(t, err, failureErr)
			for _, ch := range f.channels {
				_, err := f.client.Channel.Get(f.ctx, ch.ID)
				require.NoError(t, err)
			}
			m, err := f.client.Model.Get(f.ctx, f.model.ID)
			require.NoError(t, err)
			require.Equal(t, f.model.Settings, m.Settings)
			row, err := f.svc.SystemService.querySystemRow(f.ctx, SystemKeyModelSettings)
			require.NoError(t, err)
			require.JSONEq(t, f.systemValue, row.Value)
			value, err := f.svc.SystemService.getSystemValue(f.ctx, SystemKeyModelSettings)
			require.NoError(t, err)
			require.JSONEq(t, f.systemValue, value)
		})
	}
}

func TestChannelDeletionWithoutMatchingDeveloperRules(t *testing.T) {
	for _, value := range []string{"", `{"query_all_channel_models":true}`, `{"developer_settings":null}`, `{"developer_settings":[{"developer":"openai","associations":[{"type":"channel_tags_model","channelTagsModel":{"channelTags":["shared"]}}]}]}`} {
		t.Run(value, func(t *testing.T) {
			svc, client := setupTestChannelService(t)
			t.Cleanup(func() { client.Close() })
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			ch := createImportChannel(t, ctx, client, "unused", []string{"upstream"})
			if value != "" {
				require.NoError(t, svc.SystemService.setSystemValue(ctx, SystemKeyModelSettings, value))
			}
			client.System.Use(func(ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(context.Context, ent.Mutation) (ent.Value, error) {
					return nil, errors.New("unrelated developer settings must not be written")
				})
			})
			require.NoError(t, svc.DeleteChannel(ctx, ch.ID))
		})
	}
}

func TestChannelStatusPreservesAssociations(t *testing.T) {
	for _, bulk := range []bool{false, true} {
		f := setupChannelAssociationCleanup(t)
		for _, status := range []channel.Status{channel.StatusDisabled, channel.StatusArchived, channel.StatusEnabled} {
			if bulk {
				require.NoError(t, f.svc.bulkUpdateChannelStatus(f.ctx, []int{f.channels[0].ID}, status, "test", false))
			} else {
				_, err := f.svc.UpdateChannelStatus(f.ctx, f.channels[0].ID, status)
				require.NoError(t, err)
			}
			m, err := f.client.Model.Get(f.ctx, f.model.ID)
			require.NoError(t, err)
			require.Equal(t, f.model.Settings, m.Settings, "channel state must not rewrite rule switches")
			row, err := f.svc.SystemService.querySystemRow(f.ctx, SystemKeyModelSettings)
			require.NoError(t, err)
			require.JSONEq(t, f.systemValue, row.Value)
		}
	}
}

func TestChannelDeletionRejectsChangedDeveloperSettings(t *testing.T) {
	f := setupChannelAssociationCleanup(t)
	changed := false
	f.client.System.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			if !changed {
				changed = true
				// Change the row after the cleanup read but before its write. The
				// original snapshot must not overwrite this newer value.
				m := mutation.(*ent.SystemMutation)
				id, _ := m.ID()
				_, err := ent.FromContext(ctx).System.UpdateOneID(id).SetValue(`{"developer_settings":[]}`).Save(ctx)
				require.NoError(t, err)
			}
			return next.Mutate(ctx, mutation)
		})
	})
	err := f.svc.DeleteChannel(f.ctx, f.channels[0].ID)
	require.True(t, changed)
	require.True(t, ent.IsNotFound(err), "deletion must fail when its settings snapshot is stale: %v", err)
	_, err = f.client.Channel.Get(f.ctx, f.channels[0].ID)
	require.NoError(t, err)
	m, err := f.client.Model.Get(f.ctx, f.model.ID)
	require.NoError(t, err)
	require.Equal(t, f.model.Settings, m.Settings)
	row, err := f.svc.SystemService.querySystemRow(f.ctx, SystemKeyModelSettings)
	require.NoError(t, err)
	require.JSONEq(t, f.systemValue, row.Value, "all changes in the failed transaction must roll back")
}
