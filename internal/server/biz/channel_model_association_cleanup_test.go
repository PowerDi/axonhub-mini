package biz

import (
	"testing"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/stretchr/testify/require"
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
			Type: "model",
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
