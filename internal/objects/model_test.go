package objects

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func associationPriorities(associations []*ModelAssociation) []int {
	priorities := make([]int, 0, len(associations))
	for _, assoc := range associations {
		priorities = append(priorities, assoc.Priority)
	}

	return priorities
}

func TestNormalizeAssociationPrioritiesKeepsDenseTiers(t *testing.T) {
	associations := []*ModelAssociation{
		{Type: "channel_model", Priority: 0},
		{Type: "channel_model", Priority: 0},
		{Type: "channel_model", Priority: 1},
	}

	NormalizeAssociationPriorities(associations)

	require.Equal(t, []int{0, 0, 1}, associationPriorities(associations))
}

func TestNormalizeAssociationPrioritiesCompactsGapsAndKeepsOrder(t *testing.T) {
	associations := []*ModelAssociation{
		{Type: "channel_model", Priority: 30, ChannelModel: &ChannelModelAssociation{ChannelID: 1}},
		{Type: "channel_model", Priority: 5, ChannelModel: &ChannelModelAssociation{ChannelID: 2}},
		{Type: "channel_model", Priority: 5, ChannelModel: &ChannelModelAssociation{ChannelID: 3}},
		{Type: "channel_model", Priority: 99, ChannelModel: &ChannelModelAssociation{ChannelID: 4}},
	}

	NormalizeAssociationPriorities(associations)

	// Sorted by priority, and the two rules that shared 5 keep sharing a tier.
	require.Equal(t, []int{0, 0, 1, 2}, associationPriorities(associations))
	require.Equal(t, []int{2, 3, 1, 4}, []int{
		associations[0].ChannelModel.ChannelID,
		associations[1].ChannelModel.ChannelID,
		associations[2].ChannelModel.ChannelID,
		associations[3].ChannelModel.ChannelID,
	})
}

func TestNormalizeAssociationPrioritiesKeepsStableOrderWithinATier(t *testing.T) {
	associations := []*ModelAssociation{
		{Type: "channel_model", Priority: 7, ChannelModel: &ChannelModelAssociation{ChannelID: 1}},
		{Type: "channel_model", Priority: 7, ChannelModel: &ChannelModelAssociation{ChannelID: 2}},
		{Type: "channel_model", Priority: 2, ChannelModel: &ChannelModelAssociation{ChannelID: 3}},
	}

	NormalizeAssociationPriorities(associations)

	require.Equal(t, []int{3, 1, 2}, []int{
		associations[0].ChannelModel.ChannelID,
		associations[1].ChannelModel.ChannelID,
		associations[2].ChannelModel.ChannelID,
	})
}

func TestNormalizeAssociationPrioritiesHandlesEmptyAndNilEntries(t *testing.T) {
	NormalizeAssociationPriorities(nil)

	empty := []*ModelAssociation{}
	NormalizeAssociationPriorities(empty)
	require.Empty(t, empty)

	associations := []*ModelAssociation{
		nil,
		{Type: "channel_model", Priority: 4},
		nil,
		{Type: "channel_model", Priority: 4},
	}

	NormalizeAssociationPriorities(associations)

	require.Len(t, associations, 4)
	require.Nil(t, associations[2])
	require.Nil(t, associations[3])
	require.Equal(t, []int{0, 0}, associationPriorities(associations[:2]))
}
