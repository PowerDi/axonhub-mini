package biz

import (
	"context"
	"fmt"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestModelService_ListEnabledModels_ChannelIDsMatchMode(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:model-channel-filter?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	channelSvc := NewChannelServiceForTest(client)
	var channels []*Channel
	for i, name := range []string{"selected", "remaining", "outside-project", "other-tag"} {
		tags := []string{"production"}
		if name == "other-tag" {
			tags = []string{"staging"}
		}
		entity := &ent.Channel{
			ID:               i + 1,
			Name:             name,
			Type:             channel.TypeOpenai,
			BaseURL:          "https://example.com/v1",
			Credentials:      objects.ChannelCredentials{APIKey: "test"},
			SupportedModels:  []string{name, "shared"},
			DefaultTestModel: name,
			Status:           channel.StatusEnabled,
			Tags:             tags,
		}
		built, err := channelSvc.buildChannelWithTransformer(entity)
		require.NoError(t, err)
		channels = append(channels, built)
	}
	channelSvc.SetEnabledChannelsForTest(channels)

	allModels := []string{"selected", "remaining", "outside-project", "other-tag", "shared"}
	for _, id := range allModels {
		_, err := client.Model.Create().
			SetDeveloper("openai").
			SetModelID(id + "-configured").
			SetName(id).
			SetType(model.TypeChat).
			SetGroup("test").
			SetIcon("icon").
			SetModelCard(&objects.ModelCard{}).
			SetSettings(&objects.ModelSettings{Associations: []*objects.ModelAssociation{{
				Type:    "model",
				ModelID: &objects.ModelIDAssociation{ModelID: id},
			}}}).
			SetStatus(model.StatusEnabled).
			Save(ctx)
		require.NoError(t, err)
	}

	systemSvc := &SystemService{
		AbstractService: &AbstractService{db: client},
		Cache:           xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}
	svc := &ModelService{
		AbstractService: &AbstractService{db: client},
		channelService:  channelSvc,
		systemService:   systemSvc,
	}

	tests := []struct {
		name       string
		mode       objects.ChannelIDsMatchMode
		channelIDs []int
		tagMode    objects.ChannelTagsMatchMode
		tags       []string
		projectIDs []int
		modelIDs   []string
		want       []string
	}{
		{name: "legacy defaults to include", channelIDs: []int{1}, want: []string{"selected", "shared"}},
		{name: "include selected", mode: objects.ChannelIDsMatchModeInclude, channelIDs: []int{1}, want: []string{"selected", "shared"}},
		{name: "exclude selected keeps shared model", mode: objects.ChannelIDsMatchModeExclude, channelIDs: []int{1}, want: []string{"remaining", "outside-project", "other-tag", "shared"}},
		{name: "empty include is unrestricted", mode: objects.ChannelIDsMatchModeInclude, want: allModels},
		{name: "empty exclude is unrestricted", mode: objects.ChannelIDsMatchModeExclude, want: allModels},
		{name: "unknown excluded ID is harmless", mode: objects.ChannelIDsMatchModeExclude, channelIDs: []int{99}, want: allModels},
		{name: "exclude all channels", mode: objects.ChannelIDsMatchModeExclude, channelIDs: []int{1, 2, 3, 4}},
		{name: "exclude stays within project", mode: objects.ChannelIDsMatchModeExclude, channelIDs: []int{1}, projectIDs: []int{1, 2}, want: []string{"remaining", "shared"}},
		{name: "exclude intersects allowed tags", mode: objects.ChannelIDsMatchModeExclude, channelIDs: []int{1}, tags: []string{"production"}, want: []string{"remaining", "outside-project", "shared"}},
		{name: "exclude intersects excluded tags", mode: objects.ChannelIDsMatchModeExclude, channelIDs: []int{1}, tagMode: objects.ChannelTagsMatchModeNone, tags: []string{"production"}, want: []string{"other-tag", "shared"}},
		{name: "model allowlist cannot restore excluded channel", mode: objects.ChannelIDsMatchModeExclude, channelIDs: []int{1}, modelIDs: []string{"selected", "selected-configured", "remaining", "remaining-configured"}, want: []string{"remaining"}},
	}

	for _, queryAll := range []bool{false, true} {
		t.Run(fmt.Sprintf("queryAllChannelModels=%t", queryAll), func(t *testing.T) {
			require.NoError(t, systemSvc.SetModelSettings(ctx, SystemModelSettings{QueryAllChannelModels: queryAll}))
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					apiKey := &ent.APIKey{
						Profiles: &objects.APIKeyProfiles{
							ActiveProfile: "test",
							Profiles: []objects.APIKeyProfile{{
								Name:                 "test",
								ChannelIDs:           tt.channelIDs,
								ChannelIDsMatchMode:  tt.mode,
								ChannelTags:          tt.tags,
								ChannelTagsMatchMode: tt.tagMode,
								ModelIDs:             tt.modelIDs,
							}},
						},
						Edges: ent.APIKeyEdges{Project: &ent.Project{
							Profiles: &objects.ProjectProfiles{
								ActiveProfile: "project",
								Profiles: []objects.ProjectProfile{{
									Name:       "project",
									ChannelIDs: tt.projectIDs,
								}},
							},
						}},
					}
					models, err := svc.ListEnabledModels(contexts.WithAPIKey(ctx, apiKey))
					require.NoError(t, err)
					want := lo.Map(tt.want, func(id string, _ int) string { return id + "-configured" })
					if queryAll {
						want = append(want, tt.want...)
					}
					require.ElementsMatch(t, want, lo.Map(models, func(m ModelFacade, _ int) string { return m.ID }))
				})
			}
		})
	}
}
