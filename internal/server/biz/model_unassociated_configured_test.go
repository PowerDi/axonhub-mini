package biz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

func TestFindUnassociatedChannels_DisabledAssociationsAreConfigured(t *testing.T) {
	channels := []*ent.Channel{
		{ID: 1, Name: "primary", Status: "enabled", Tags: []string{"primary"}, SupportedModels: []string{"matched", "other"}},
		{ID: 2, Name: "backup", Status: "enabled", Tags: []string{"backup"}, SupportedModels: []string{"matched", "other"}},
	}
	tests := []struct {
		name        string
		association *objects.ModelAssociation
	}{
		{
			name: "channel_model",
			association: &objects.ModelAssociation{Type: "channel_model", ChannelModel: &objects.ChannelModelAssociation{
				ChannelID: 1, ModelID: "matched",
			}},
		},
		{
			name: "channel_regex",
			association: &objects.ModelAssociation{Type: "channel_regex", ChannelRegex: &objects.ChannelRegexAssociation{
				ChannelID: 1, Pattern: "^matched$",
			}},
		},
		{
			name: "model_with_excluded_channel",
			association: &objects.ModelAssociation{Type: "model", ModelID: &objects.ModelIDAssociation{
				ModelID: "matched", Exclude: []*objects.ExcludeAssociation{{ChannelIds: []int{2}}},
			}},
		},
		{
			name: "regex_with_excluded_name",
			association: &objects.ModelAssociation{Type: "regex", Regex: &objects.RegexAssociation{
				Pattern: "^matched$", Exclude: []*objects.ExcludeAssociation{{ChannelNamePattern: "^backup$"}},
			}},
		},
		{
			name: "regex_with_excluded_tag",
			association: &objects.ModelAssociation{Type: "regex", Regex: &objects.RegexAssociation{
				Pattern: "^matched$", Exclude: []*objects.ExcludeAssociation{{ChannelTags: []string{"backup"}}},
			}},
		},
		{
			name: "channel_tags_model",
			association: &objects.ModelAssociation{Type: "channel_tags_model", ChannelTagsModel: &objects.ChannelTagsModelAssociation{
				ChannelTags: []string{"primary"}, ModelID: "matched",
			}},
		},
		{
			name: "channel_tags_regex",
			association: &objects.ModelAssociation{Type: "channel_tags_regex", ChannelTagsRegex: &objects.ChannelTagsRegexAssociation{
				ChannelTags: []string{"primary"}, Pattern: "^matched$",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			association := tt.association
			association.Disabled = true
			association.Priority = 3
			association.When = &objects.ModelAssociationWhen{
				Enabled: true,
				Condition: &objects.Condition{
					Type: "condition", Field: "stream", Operator: "eq", Value: true,
				},
			}
			before, err := json.Marshal(association)
			require.NoError(t, err)

			// Nil and duplicate rules must neither panic nor change coverage.
			rules := []*objects.ModelAssociation{nil, association, association}
			wrappers := []*Channel{{Channel: channels[0]}, {Channel: channels[1]}}
			require.Empty(t, MatchConnections(rules, wrappers))
			require.Empty(t, MatchAssociations(rules, wrappers))

			byChannel := map[int][]string{}
			for _, info := range findUnassociatedChannels(channels, rules) {
				byChannel[info.Channel.ID] = info.Models
			}
			require.ElementsMatch(t, []string{"other"}, byChannel[1], "a disabled rule is still configured")
			require.ElementsMatch(t, []string{"matched", "other"}, byChannel[2], "other channels remain independent")

			after, err := json.Marshal(association)
			require.NoError(t, err)
			require.Equal(t, string(before), string(after), "detection must not modify shared rule state")
			require.Empty(t, MatchConnections(rules, wrappers), "disabled rules must remain inactive")
			require.Empty(t, MatchAssociations(rules, wrappers), "routing must remain unchanged")
		})
	}
}

func TestFindUnassociatedChannels_DisabledInheritedAssociations(t *testing.T) {
	channels := []*ent.Channel{
		{ID: 1, Status: "enabled", Tags: []string{"primary"}, SupportedModels: []string{"request-model"}},
		{ID: 2, Status: "enabled", Tags: []string{"backup"}, SupportedModels: []string{"request-model"}},
	}
	for _, inherited := range []*objects.ModelAssociation{
		{Type: "channel_model", Disabled: true, ChannelModel: &objects.ChannelModelAssociation{ChannelID: 1}},
		{Type: "channel_tags_model", Disabled: true, ChannelTagsModel: &objects.ChannelTagsModelAssociation{ChannelTags: []string{"primary"}}},
	} {
		t.Run(inherited.Type, func(t *testing.T) {
			settings := &SystemModelSettings{DeveloperSettings: []*DeveloperModelSettings{{
				Developer: "test-developer", Associations: []*objects.ModelAssociation{inherited},
			}}}
			before, err := json.Marshal(settings)
			require.NoError(t, err)

			for _, disableInheritance := range []bool{false, true} {
				m := &ent.Model{
					ModelID: "request-model", Developer: "test-developer",
					Settings: &objects.ModelSettings{DisableDeveloperSettingsInheritance: disableInheritance},
				}
				byChannel := map[int][]string{}
				for _, info := range findUnassociatedChannels(channels, EffectiveModelAssociations(settings, m)) {
					byChannel[info.Channel.ID] = info.Models
				}
				require.Equal(t, disableInheritance, len(byChannel[1]) > 0)
				require.Equal(t, []string{"request-model"}, byChannel[2])
			}

			after, err := json.Marshal(settings)
			require.NoError(t, err)
			require.Equal(t, string(before), string(after))
		})
	}
}

func TestFindUnassociatedChannels_DisabledRulesUseRequestModelEntries(t *testing.T) {
	tests := []struct {
		name            string
		supportedModels []string
		settings        *objects.ChannelSettings
		associatedModel string
		unassociated    []string
	}{
		{
			name: "mapped_aliases_remain_distinct", supportedModels: []string{"upstream"}, associatedModel: "alias-a",
			settings: &objects.ChannelSettings{
				ModelMappings:    []objects.ModelMapping{{From: "alias-a", To: "upstream"}, {From: "alias-b", To: "upstream"}},
				HideMappedModels: true,
			},
			unassociated: []string{"alias-b"},
		},
		{
			name: "actual_model_does_not_cover_aliases", supportedModels: []string{"upstream"}, associatedModel: "upstream",
			settings: &objects.ChannelSettings{
				ModelMappings:    []objects.ModelMapping{{From: "alias-a", To: "upstream"}, {From: "alias-b", To: "upstream"}},
				HideMappedModels: true,
			},
			unassociated: []string{"alias-a", "alias-b"},
		},
		{
			name: "extra_prefix", supportedModels: []string{"model"}, associatedModel: "vendor/model",
			settings: &objects.ChannelSettings{ExtraModelPrefix: "vendor", HideOriginalModels: true},
		},
		{
			name: "trimmed_prefix", supportedModels: []string{"vendor/model"}, associatedModel: "model",
			settings: &objects.ChannelSettings{AutoTrimedModelPrefixes: []string{"vendor"}, HideOriginalModels: true},
		},
		{
			name: "lowercased_request_key", supportedModels: []string{"MODEL"}, associatedModel: "model",
			settings: &objects.ChannelSettings{LowercaseModelID: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &ent.Channel{ID: 1, Status: "enabled", SupportedModels: tt.supportedModels, Settings: tt.settings}
			association := &objects.ModelAssociation{
				Type: "channel_model", Disabled: true,
				ChannelModel: &objects.ChannelModelAssociation{ChannelID: ch.ID, ModelID: tt.associatedModel},
			}
			result := findUnassociatedChannels([]*ent.Channel{ch}, []*objects.ModelAssociation{association})
			if len(tt.unassociated) == 0 {
				require.Empty(t, result)
			} else {
				require.Len(t, result, 1)
				require.ElementsMatch(t, tt.unassociated, result[0].Models)
			}
			require.True(t, association.Disabled)
		})
	}
}

func TestQueryUnassociatedChannels_ConfiguredModelScope(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  model.Status
		deleted bool
		covered bool
	}{
		{name: "enabled", status: model.StatusEnabled, covered: true},
		{name: "disabled", status: model.StatusDisabled, covered: true},
		{name: "archived", status: model.StatusArchived},
		{name: "deleted", status: model.StatusEnabled, deleted: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, ctx, client := setupImportTest(t, "file:configured_scope_"+tt.name+"?mode=memory&_fk=0")
			ch := createImportChannel(t, ctx, client, "test", []string{"provider-model"})
			association := channelModelAssoc(ch.ID, "provider-model", 0)
			association.Disabled = true
			m := createImportModel(t, ctx, client, "different-logical-model", tt.status, []*objects.ModelAssociation{association})
			if tt.deleted {
				require.NoError(t, client.Model.DeleteOne(m).Exec(ctx))
			}

			result, err := svc.QueryUnassociatedChannels(ctx)
			require.NoError(t, err)
			if tt.covered {
				require.Empty(t, result, "match the saved request ID, not the model's public ID")
			} else {
				require.Len(t, result, 1, "archived and deleted models do not contribute coverage")
				require.Equal(t, []string{"provider-model"}, result[0].Models)
			}
		})
	}
}

func TestQueryUnassociatedChannels_DeletedChannelDoesNotHideOtherSources(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:configured_deleted_channel?mode=memory&_fk=0")
	deleted := createImportChannel(t, ctx, client, "deleted", []string{"shared-model"})
	remaining := createImportChannel(t, ctx, client, "remaining", []string{"shared-model"})
	association := channelModelAssoc(deleted.ID, "shared-model", 0)
	association.Disabled = true
	createImportModel(t, ctx, client, "shared-model", model.StatusEnabled, []*objects.ModelAssociation{association})
	require.NoError(t, client.Channel.DeleteOne(deleted).Exec(ctx))

	result, err := svc.QueryUnassociatedChannels(ctx)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, remaining.ID, result[0].Channel.ID)
	require.Equal(t, []string{"shared-model"}, result[0].Models)
}
