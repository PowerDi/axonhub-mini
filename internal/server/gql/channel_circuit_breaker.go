package gql

import (
	"time"

	"github.com/looplj/axonhub/internal/server/biz"
)

// channelCircuitBreakerStats maps runtime snapshots onto the GraphQL type.
// Closed is the healthy state and tells an operator nothing, so only tripped
// breakers are exposed; zero timestamps become null instead of year 1.
func channelCircuitBreakerStats(stats []*biz.ModelCircuitBreakerStats) []*ChannelCircuitBreakerStats {
	results := make([]*ChannelCircuitBreakerStats, 0, len(stats))

	for _, item := range stats {
		if item == nil || item.State == biz.StateClosed {
			continue
		}

		var lastFailureAt *time.Time
		if !item.LastFailureAt.IsZero() {
			value := item.LastFailureAt
			lastFailureAt = &value
		}

		var nextProbeAt *time.Time
		if !item.NextProbeAt.IsZero() {
			value := item.NextProbeAt
			nextProbeAt = &value
		}

		results = append(results, &ChannelCircuitBreakerStats{
			ModelID:             item.ModelID,
			State:               ChannelCircuitBreakerState(item.State),
			ConsecutiveFailures: item.ConsecutiveFailures,
			LastFailureAt:       lastFailureAt,
			NextProbeAt:         nextProbeAt,
		})
	}

	return results
}
