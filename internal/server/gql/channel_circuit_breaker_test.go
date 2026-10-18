package gql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
)

func TestChannelCircuitBreakerStatsSkipsClosedAndEmptyTimestamps(t *testing.T) {
	failureAt := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	probeAt := failureAt.Add(5 * time.Minute)

	results := channelCircuitBreakerStats([]*biz.ModelCircuitBreakerStats{
		nil,
		{ModelID: "healthy", State: biz.StateClosed},
		{ModelID: "open", State: biz.StateOpen, ConsecutiveFailures: 5, LastFailureAt: failureAt, NextProbeAt: probeAt},
		{ModelID: "half-open", State: biz.StateHalfOpen, ConsecutiveFailures: 3, LastFailureAt: failureAt},
	})

	require.Len(t, results, 2)

	require.Equal(t, "open", results[0].ModelID)
	require.Equal(t, ChannelCircuitBreakerStateOpen, results[0].State)
	require.Equal(t, 5, results[0].ConsecutiveFailures)
	require.Equal(t, failureAt, *results[0].LastFailureAt)
	require.Equal(t, probeAt, *results[0].NextProbeAt)

	require.Equal(t, "half-open", results[1].ModelID)
	require.Equal(t, ChannelCircuitBreakerStateHalfOpen, results[1].State)
	require.Nil(t, results[1].NextProbeAt)
}

func TestChannelCircuitBreakerStatsReturnsNonNullEmptySlice(t *testing.T) {
	results := channelCircuitBreakerStats(nil)

	require.NotNil(t, results)
	require.Empty(t, results)
}

// openTestBreaker drives the shared breaker for one channel/model to the open
// state using the real transition logic instead of reaching into its internals.
func openTestBreaker(t *testing.T, ctx context.Context, channelID int, modelID string) {
	t.Helper()

	breaker := orchestrator.SharedModelCircuitBreaker()
	for range biz.DefaultModelCircuitBreakerPolicy().OpenThreshold {
		breaker.RecordError(ctx, channelID, modelID, false)
	}

	require.Equal(t, biz.StateOpen, breaker.GetModelCircuitBreakerStats(ctx, channelID, modelID).State)
}

func resetTestBreakers(t *testing.T, channelIDs []int, modelIDs []string) {
	t.Helper()

	t.Cleanup(func() {
		breaker := orchestrator.SharedModelCircuitBreaker()
		for _, channelID := range channelIDs {
			for _, modelID := range modelIDs {
				require.NoError(t, breaker.ResetModelStatus(context.Background(), channelID, modelID))
			}
		}
	})
}

func TestChannelResolverLiveCircuitBreakerStatsOnlyReportsTheChannel(t *testing.T) {
	ctx := contexts.WithUser(context.Background(), &ent.User{IsOwner: true})

	const (
		channelID      = 990001
		otherChannelID = 990002
		modelID        = "gpt-4o"
	)

	openTestBreaker(t, ctx, channelID, modelID)
	openTestBreaker(t, ctx, otherChannelID, modelID)
	resetTestBreakers(t, []int{channelID, otherChannelID}, []string{modelID})

	stats, err := (&channelResolver{&Resolver{}}).LiveCircuitBreakerStats(ctx, &ent.Channel{ID: channelID})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, modelID, stats[0].ModelID)
	require.Equal(t, ChannelCircuitBreakerStateOpen, stats[0].State)
	require.NotNil(t, stats[0].NextProbeAt)
}

func TestMutationResolverResetChannelModelCircuitBreakersResetsOnlyThatChannel(t *testing.T) {
	ctx := contexts.WithUser(context.Background(), &ent.User{IsOwner: true})

	const (
		channelID      = 990003
		otherChannelID = 990004
	)

	modelIDs := []string{"gpt-4o", "gpt-4o-mini"}
	for _, modelID := range modelIDs {
		openTestBreaker(t, ctx, channelID, modelID)
	}

	openTestBreaker(t, ctx, otherChannelID, "gpt-4o")
	resetTestBreakers(t, []int{channelID, otherChannelID}, []string{"gpt-4o", "gpt-4o-mini"})

	reset, err := (&mutationResolver{&Resolver{}}).ResetChannelModelCircuitBreakers(ctx, objects.GUID{ID: channelID})
	require.NoError(t, err)
	require.True(t, reset)

	breaker := orchestrator.SharedModelCircuitBreaker()
	for _, modelID := range modelIDs {
		require.Equal(t, biz.StateClosed, breaker.GetModelCircuitBreakerStats(ctx, channelID, modelID).State)
	}

	require.Equal(t, biz.StateOpen, breaker.GetModelCircuitBreakerStats(ctx, otherChannelID, "gpt-4o").State)
}

func TestMutationResolverResetChannelModelCircuitBreakersRequiresWriteScope(t *testing.T) {
	ctx := contexts.WithUser(context.Background(), &ent.User{
		Scopes: []string{string(scopes.ScopeReadChannels)},
	})

	_, err := (&mutationResolver{&Resolver{}}).ResetChannelModelCircuitBreakers(ctx, objects.GUID{ID: 990005})
	require.ErrorIs(t, err, ErrNotOwner)
}
