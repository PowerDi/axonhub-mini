// 职责: 验证渠道独立入口的名称、命名空间保护、目录可见性和设置更新兼容性。
// 关联文件:
//
//	internal/server/biz/channel_independent_models.go
//	internal/server/biz/channel.go
//	internal/server/biz/channel_settings.go
//	internal/server/biz/model.go
//	internal/server/biz/model_import.go
//
// 注意事项:
//
//	部分夹具直接写入 Ent，专门验证旁路写入冲突时目录不会静默覆盖身份。
package biz

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

func setupIndependentModelsTest(t *testing.T) (context.Context, *ent.Client, *ChannelService, *ModelService, *SystemService) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	channelSvc := NewChannelServiceForTest(client)
	t.Cleanup(channelSvc.modelCache.Stop)
	systemSvc := channelSvc.SystemService
	modelSvc := NewModelService(ModelServiceParams{
		Ent: client, ChannelService: channelSvc, SystemService: systemSvc, ModelCache: channelSvc.modelCache,
	})
	return ctx, client, channelSvc, modelSvc, systemSvc
}

func createIndependentTestChannel(t *testing.T, ctx context.Context, client *ent.Client, name, prefix string, models []string) *ent.Channel {
	t.Helper()
	entity, err := client.Channel.Create().
		SetName(name).SetType(channel.TypeOpenai).SetBaseURL("https://example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels(models).SetDefaultTestModel(models[0]).
		SetSettings(&objects.ChannelSettings{ExtraModelPrefix: prefix, IndependentListing: prefix != ""}).
		SetStatus(channel.StatusEnabled).Save(ctx)
	require.NoError(t, err)
	return entity
}

func TestIndependentModelEntries(t *testing.T) {
	for _, tt := range []struct {
		name     string
		settings *objects.ChannelSettings
		want     map[string]string
	}{
		{name: "old channel with no settings"},
		{name: "old prefixed channel", settings: &objects.ChannelSettings{ExtraModelPrefix: "claude官池"}},
		{name: "enabled", settings: &objects.ChannelSettings{ExtraModelPrefix: "claude官池", IndependentListing: true}, want: map[string]string{"claude官池/Claude-Opus": "Claude-Opus"}},
		{name: "lowercase request only", settings: &objects.ChannelSettings{ExtraModelPrefix: "Pool", IndependentListing: true, LowercaseModelID: true}, want: map[string]string{"pool/claude-opus": "Claude-Opus"}},
		{name: "hide originals keeps prefix", settings: &objects.ChannelSettings{ExtraModelPrefix: "claude官池", IndependentListing: true, HideOriginalModels: true}, want: map[string]string{"claude官池/Claude-Opus": "Claude-Opus"}},
		{name: "hide mapped target also hides prefix", settings: &objects.ChannelSettings{ExtraModelPrefix: "claude官池", IndependentListing: true, HideMappedModels: true, ModelMappings: []objects.ModelMapping{{From: "alias", To: "Claude-Opus"}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := &Channel{Channel: &ent.Channel{Name: "claude官池", SupportedModels: []string{"Claude-Opus"}, Settings: tt.settings}}
			before := maps.Clone(ch.GetModelEntries())
			entries := ch.IndependentModelEntries()
			got := make(map[string]string)
			for id, entry := range entries {
				got[id] = entry.ActualModel
				require.Equal(t, id, entry.RequestModel)
				require.Equal(t, "Claude-Opus · claude官池", ch.IndependentModelDisplayName(entry))
			}
			if len(tt.want) == 0 {
				require.Empty(t, got)
			} else {
				require.Equal(t, tt.want, got)
			}
			require.Equal(t, before, ch.GetModelEntries(), "projection must not mutate cached entries")
		})
	}
}

func TestIndependentModelSettingsPatch(t *testing.T) {
	ctx, client, channelSvc, _, _ := setupIndependentModelsTest(t)
	entity := createIndependentTestChannel(t, ctx, client, "Internal Pool", "claude官池", []string{"claude-opus-5-5"})
	updated, err := channelSvc.UpdateChannelSettings(ctx, entity.ID, &UpdateChannelSettingsInput{
		IndependentDisplayName: lo.ToPtr("claude官池"),
	})
	require.NoError(t, err)
	require.True(t, updated.Settings.IndependentListing)
	require.Equal(t, "claude官池", updated.Settings.ExtraModelPrefix)
	require.Equal(t, "claude官池", updated.Settings.IndependentDisplayName)
	ch := &Channel{Channel: updated}
	entries := ch.IndependentModelEntries()
	require.Equal(t, "claude-opus-5-5 · claude官池", ch.IndependentModelDisplayName(entries["claude官池/claude-opus-5-5"]))

	updated, err = channelSvc.UpdateChannelSettings(ctx, entity.ID, &UpdateChannelSettingsInput{HideOriginalModels: lo.ToPtr(true)})
	require.NoError(t, err)
	require.True(t, updated.Settings.IndependentListing)
	require.Equal(t, "claude官池", updated.Settings.IndependentDisplayName)

	updated, err = channelSvc.UpdateChannel(ctx, entity.ID, &ent.UpdateChannelInput{Name: lo.ToPtr("Renamed internal channel")})
	require.NoError(t, err)
	require.Equal(t, "claude官池", updated.Settings.ExtraModelPrefix, "renaming the channel must not change calling IDs")

	updated, err = channelSvc.UpdateChannelSettings(ctx, entity.ID, &UpdateChannelSettingsInput{
		IndependentListing: lo.ToPtr(false), IndependentDisplayName: lo.ToPtr(""),
	})
	require.NoError(t, err)
	require.False(t, updated.Settings.IndependentListing)
	require.Empty(t, updated.Settings.IndependentDisplayName)
	require.Equal(t, "claude官池", updated.Settings.ExtraModelPrefix)
	require.Empty(t, (&Channel{Channel: updated}).IndependentModelEntries())
}

func TestIndependentChannelNamespaceValidation(t *testing.T) {
	t.Run("invalid prefixes", func(t *testing.T) {
		for _, prefix := range []string{"", " pool ", "/pool", "pool/", "pool\nname"} {
			t.Run(fmt.Sprintf("%q", prefix), func(t *testing.T) {
				ctx, _, channelSvc, _, _ := setupIndependentModelsTest(t)
				_, err := channelSvc.CreateChannel(ctx, ent.CreateChannelInput{
					Name: "Pool", Type: channel.TypeOpenai, SupportedModels: []string{"m"},
					Credentials: objects.ChannelCredentials{APIKey: "key"},
					Settings:    &objects.ChannelSettings{ExtraModelPrefix: prefix, IndependentListing: true},
				})
				require.ErrorContains(t, err, "extra model prefix")
			})
		}
	})

	for _, status := range []model.Status{model.StatusEnabled, model.StatusDisabled, model.StatusArchived} {
		t.Run("configured model "+status.String(), func(t *testing.T) {
			ctx, client, channelSvc, modelSvc, _ := setupIndependentModelsTest(t)
			m, err := modelSvc.CreateModel(ctx, ent.CreateModelInput{ModelID: "pool/future-model", Developer: "test", Name: "Conflict", Icon: "", Group: "", ModelCard: &objects.ModelCard{}, Settings: &objects.ModelSettings{}})
			require.NoError(t, err)
			_, err = client.Model.UpdateOne(m).SetStatus(status).Save(ctx)
			require.NoError(t, err)
			entity := createIndependentTestChannel(t, ctx, client, "Pool", "", []string{"m"})
			_, err = channelSvc.UpdateChannelSettings(ctx, entity.ID, &UpdateChannelSettingsInput{
				ExtraModelPrefix: lo.ToPtr("pool"), IndependentListing: lo.ToPtr(true),
			})
			require.ErrorIs(t, err, ErrIndependentModelConflict)
			stored, err := client.Channel.Get(ctx, entity.ID)
			require.NoError(t, err)
			require.False(t, stored.Settings.IndependentListing)
		})
	}

	t.Run("duplicate and nested prefixes remain reserved on disabled channels", func(t *testing.T) {
		ctx, client, channelSvc, _, _ := setupIndependentModelsTest(t)
		owner := createIndependentTestChannel(t, ctx, client, "Owner", "pool", []string{"m"})
		_, err := client.Channel.UpdateOne(owner).SetStatus(channel.StatusDisabled).Save(ctx)
		require.NoError(t, err)
		for _, prefix := range []string{"pool", "pool/nested"} {
			_, err := channelSvc.CreateChannel(ctx, ent.CreateChannelInput{
				Name: prefix, Type: channel.TypeOpenai, SupportedModels: []string{"another-model"},
				Credentials: objects.ChannelCredentials{APIKey: "key"},
				Settings:    &objects.ChannelSettings{ExtraModelPrefix: prefix, IndependentListing: true},
			})
			require.ErrorIs(t, err, ErrIndependentModelConflict)
		}
	})

	t.Run("legacy aliases cannot occupy a reserved namespace", func(t *testing.T) {
		ctx, client, channelSvc, _, _ := setupIndependentModelsTest(t)
		createIndependentTestChannel(t, ctx, client, "Owner", "pool", []string{"m"})
		_, err := channelSvc.CreateChannel(ctx, ent.CreateChannelInput{
			Name: "Legacy", Type: channel.TypeOpenai, SupportedModels: []string{"pool/future-model"},
			Credentials: objects.ChannelCredentials{APIKey: "key"},
		})
		require.ErrorIs(t, err, ErrIndependentModelConflict)
	})

	t.Run("existing legacy aliases block enabling an overlapping prefix", func(t *testing.T) {
		ctx, client, channelSvc, _, _ := setupIndependentModelsTest(t)
		createIndependentTestChannel(t, ctx, client, "Legacy", "", []string{"pool/future-model"})
		entity := createIndependentTestChannel(t, ctx, client, "Owner", "", []string{"m"})
		_, err := channelSvc.UpdateChannelSettings(ctx, entity.ID, &UpdateChannelSettingsInput{
			ExtraModelPrefix: lo.ToPtr("pool"), IndependentListing: lo.ToPtr(true),
		})
		require.ErrorIs(t, err, ErrIndependentModelConflict)
	})
}

func TestIndependentConfiguredModelWriteGuards(t *testing.T) {
	ctx, client, channelSvc, modelSvc, _ := setupIndependentModelsTest(t)
	owner := createIndependentTestChannel(t, ctx, client, "Owner", "pool", []string{"claude-opus-5-5"})
	input := ent.CreateModelInput{Developer: "anthropic", ModelID: "pool/future-model", Name: "Conflict", Group: "", Icon: "", ModelCard: &objects.ModelCard{}, Settings: &objects.ModelSettings{}}
	_, err := modelSvc.CreateModel(ctx, input)
	require.ErrorIs(t, err, ErrIndependentModelConflict)

	good := ent.CreateModelInput{Developer: "anthropic", ModelID: "claude-opus-5-5", Name: "Aggregate", Group: "", Icon: "", ModelCard: &objects.ModelCard{}, Settings: &objects.ModelSettings{}}
	m, err := modelSvc.CreateModel(ctx, good)
	require.NoError(t, err, "bare aggregate IDs must remain available")
	_, err = modelSvc.UpdateModel(ctx, m.ID, &ent.UpdateModelInput{ModelID: lo.ToPtr(input.ModelID)})
	require.ErrorIs(t, err, ErrIndependentModelConflict)
	stored, err := client.Model.Get(ctx, m.ID)
	require.NoError(t, err)
	require.Equal(t, good.ModelID, stored.ModelID)

	other := good
	other.ModelID = "another-aggregate"
	_, err = modelSvc.BulkCreateModels(ctx, []*ent.CreateModelInput{&other, &input})
	require.ErrorIs(t, err, ErrIndependentModelConflict)
	_, err = modelSvc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{{
		Metadata: &ImportModelMetadata{Developer: "anthropic", ModelID: "pool/claude-opus-5-5", Name: "Conflict"},
		Sources:  []*ImportModelSource{{ChannelID: owner.ID, UpstreamModelID: "claude-opus-5-5"}},
	}})
	require.ErrorIs(t, err, ErrIndependentModelConflict)

	_, err = channelSvc.UpdateChannelSettings(ctx, owner.ID, &UpdateChannelSettingsInput{IndependentListing: lo.ToPtr(false)})
	require.NoError(t, err)
	_, err = modelSvc.CreateModel(ctx, input)
	require.NoError(t, err, "switching off independent entries releases their namespace")
}

func TestIndependentModelList(t *testing.T) {
	ctx, client, channelSvc, modelSvc, systemSvc := setupIndependentModelsTest(t)
	pool := createIndependentTestChannel(t, ctx, client, "claude官池", "claude官池", []string{"claude-opus-5-5", "claude-sonnet-5-5"})
	public := createIndependentTestChannel(t, ctx, client, "林夕", "", []string{"claude-opus-5-5"})
	createConfiguredModel(t, ctx, client, "claude-opus-5-5", model.TypeChat, &objects.ModelAssociation{
		Type: "channel_model", ChannelModel: &objects.ChannelModelAssociation{ChannelID: public.ID, ModelID: "claude-opus-5-5"},
	})
	reloadEnabledChannels(t, ctx, client, channelSvc)

	for _, queryAll := range []bool{false, true} {
		t.Run(fmt.Sprintf("global query=%t", queryAll), func(t *testing.T) {
			require.NoError(t, systemSvc.SetModelSettings(ctx, SystemModelSettings{QueryAllChannelModels: queryAll}))
			models, err := modelSvc.ListEnabledModels(ctx)
			require.NoError(t, err)
			ids := lo.Map(models, func(m ModelFacade, _ int) string { return m.ID })
			require.Contains(t, ids, "claude-opus-5-5")
			require.Contains(t, ids, "claude官池/claude-opus-5-5")
			require.Contains(t, ids, "claude官池/claude-sonnet-5-5")
			require.Equal(t, len(ids), len(lo.Uniq(ids)))
			if !queryAll {
				require.Len(t, models, 3)
			}
			entry, found := lo.Find(models, func(m ModelFacade) bool { return m.ID == "claude官池/claude-opus-5-5" })
			require.True(t, found)
			require.Equal(t, "claude-opus-5-5 · claude官池", entry.DisplayName)
		})
	}

	require.NoError(t, systemSvc.SetModelSettings(ctx, SystemModelSettings{QueryAllChannelModels: false}))
	for _, tt := range []struct {
		name       string
		profile    objects.APIKeyProfile
		projectIDs []int
		want       []string
	}{
		{name: "model allowlist", profile: objects.APIKeyProfile{ModelIDs: []string{"claude官池/claude-opus-5-5"}}, want: []string{"claude官池/claude-opus-5-5"}},
		{name: "channel allowlist", profile: objects.APIKeyProfile{ChannelIDs: []int{public.ID}}, want: []string{"claude-opus-5-5"}},
		{name: "channel exclusion", profile: objects.APIKeyProfile{ChannelIDs: []int{pool.ID}, ChannelIDsMatchMode: objects.ChannelIDsMatchModeExclude}, want: []string{"claude-opus-5-5"}},
		{name: "project upper boundary", profile: objects.APIKeyProfile{ModelIDs: []string{"claude官池/claude-opus-5-5"}}, projectIDs: []int{public.ID}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.profile.Name = "test"
			key := &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{ActiveProfile: "test", Profiles: []objects.APIKeyProfile{tt.profile}},
				Edges: ent.APIKeyEdges{Project: &ent.Project{
					Profiles: &objects.ProjectProfiles{ActiveProfile: "project", Profiles: []objects.ProjectProfile{{Name: "project", ChannelIDs: tt.projectIDs}}},
				}},
			}
			models, err := modelSvc.ListEnabledModels(contexts.WithAPIKey(ctx, key))
			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, lo.Map(models, func(m ModelFacade, _ int) string { return m.ID }))
		})
	}

	require.NoError(t, systemSvc.SetModelSettings(ctx, SystemModelSettings{ModelBlacklistRegex: "^claude官池/.*"}))
	models, err := modelSvc.ListEnabledModels(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "claude-opus-5-5", models[0].ID)

	require.NoError(t, systemSvc.SetModelSettings(ctx, SystemModelSettings{}))
	_, err = channelSvc.UpdateChannelSettings(ctx, pool.ID, &UpdateChannelSettingsInput{IndependentListing: lo.ToPtr(false)})
	require.NoError(t, err)
	reloadEnabledChannels(t, ctx, client, channelSvc)
	models, err = modelSvc.ListEnabledModels(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "claude-opus-5-5", models[0].ID)
}

func TestIndependentModelListRejectsOutOfBandConflicts(t *testing.T) {
	for _, conflict := range []string{"configured", "channel", "duplicate owner"} {
		t.Run(conflict, func(t *testing.T) {
			ctx, client, channelSvc, modelSvc, systemSvc := setupIndependentModelsTest(t)
			createIndependentTestChannel(t, ctx, client, "Owner", "pool", []string{"m"})
			switch conflict {
			case "configured":
				_, err := client.Model.Create().SetDeveloper("test").SetModelID("pool/m").SetName("Conflict").SetGroup("").SetIcon("").SetModelCard(&objects.ModelCard{}).SetSettings(&objects.ModelSettings{}).SetStatus(model.StatusDisabled).Save(ctx)
				require.NoError(t, err)
			case "channel":
				createIndependentTestChannel(t, ctx, client, "Legacy conflict", "", []string{"pool/m"})
			case "duplicate owner":
				createIndependentTestChannel(t, ctx, client, "Other owner", "pool", []string{"m"})
			}
			reloadEnabledChannels(t, ctx, client, channelSvc)
			require.NoError(t, systemSvc.SetModelSettings(ctx, SystemModelSettings{}))
			_, err := modelSvc.ListEnabledModels(ctx)
			require.ErrorIs(t, err, ErrIndependentModelConflict)
		})
	}
}

func TestIndependentModelListDisabledOwnerKeepsNamespace(t *testing.T) {
	ctx, client, channelSvc, modelSvc, systemSvc := setupIndependentModelsTest(t)
	owner := createIndependentTestChannel(t, ctx, client, "Owner", "pool", []string{"m"})
	_, err := client.Channel.UpdateOne(owner).SetStatus(channel.StatusDisabled).Save(ctx)
	require.NoError(t, err)
	createIndependentTestChannel(t, ctx, client, "Legacy Sync", "", []string{"pool/m", "normal"})
	reloadEnabledChannels(t, ctx, client, channelSvc)
	require.NoError(t, systemSvc.SetModelSettings(ctx, SystemModelSettings{QueryAllChannelModels: true}))
	models, err := modelSvc.ListEnabledModels(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "normal", models[0].ID)
}
