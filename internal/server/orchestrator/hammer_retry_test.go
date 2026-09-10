package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUpstream429() error {
	return &httpclient.Error{
		StatusCode: http.StatusTooManyRequests,
		Status:     http.StatusText(http.StatusTooManyRequests),
		Body:       []byte(`{"error":{"message":"rate limit exceeded"}}`),
	}
}

func newUpstream500(message string) error {
	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       []byte(`{"error":{"message":"` + message + `","code":"get_channel_failed"}}`),
	}
}

func TestClassifyHammerError(t *testing.T) {
	hammer := &objects.ChannelHammerRetry{
		ErrorPatterns: []objects.RetryableErrorPattern{
			{Pattern: "负载已经达到上限"},
			{Pattern: "rate limit exceeded"},
		},
	}

	t.Run("upstream 429 is hammerable and not a hard failure", func(t *testing.T) {
		hammerable, hard := classifyHammerError(newUpstream429(), hammer)
		assert.True(t, hammerable)
		assert.False(t, hard)
	})

	t.Run("pattern-matched 500 is a hard failure", func(t *testing.T) {
		hammerable, hard := classifyHammerError(newUpstream500("当前模型 gpt-6-astra 负载已经达到上限，请稍后重试"), hammer)
		assert.True(t, hammerable)
		assert.True(t, hard)
	})

	t.Run("unmatched 500 is not hammerable", func(t *testing.T) {
		hammerable, hard := classifyHammerError(newUpstream500("upstream connection reset"), hammer)
		assert.False(t, hammerable)
		assert.False(t, hard)
	})

	t.Run("nil hammer config never classifies", func(t *testing.T) {
		hammerable, hard := classifyHammerError(newUpstream429(), nil)
		assert.False(t, hammerable)
		assert.False(t, hard)
	})

	t.Run("local rpm rejection is not hammerable", func(t *testing.T) {
		err := newLocalRPMExhaustedError(&biz.Channel{Channel: &ent.Channel{Name: "test"}}, 10)
		hammerable, hard := classifyHammerError(err, hammer)
		assert.False(t, hammerable)
		assert.False(t, hard)
	})

	t.Run("nil error is not hammerable", func(t *testing.T) {
		hammerable, hard := classifyHammerError(nil, hammer)
		assert.False(t, hammerable)
		assert.False(t, hard)
	})

	t.Run("plain error is not hammerable", func(t *testing.T) {
		hammerable, hard := classifyHammerError(errors.New("boom"), hammer)
		assert.False(t, hammerable)
		assert.False(t, hard)
	})
}

func TestHammerDefaults(t *testing.T) {
	hammer := &objects.ChannelHammerRetry{}

	assert.Equal(t, objects.DefaultHammerRetryDelayMs, hammer.EffectiveDelayMs())
	assert.Equal(t, objects.DefaultHammerMaxRetries, hammer.EffectiveMaxRetries())
	assert.Equal(t, objects.DefaultHammerMaxDurationMs, hammer.EffectiveMaxDurationMs())
	assert.Equal(t, objects.DefaultHammerConsecutiveHardFailures, hammer.EffectiveConsecutiveHardFailureLimit())

	// Explicit values win over defaults.
	custom := &objects.ChannelHammerRetry{
		RetryDelayMs:                250,
		MaxRetries:                  10,
		MaxDurationMs:               30_000,
		ConsecutiveHardFailureLimit: 5,
	}
	assert.Equal(t, 250, custom.EffectiveDelayMs())
	assert.Equal(t, 10, custom.EffectiveMaxRetries())
	assert.Equal(t, 30_000, custom.EffectiveMaxDurationMs())
	assert.Equal(t, 5, custom.EffectiveConsecutiveHardFailureLimit())
}

func newHammerOutbound(hammer *objects.ChannelHammerRetry) *PersistentOutboundTransformer {
	channel := &biz.Channel{
		Channel: &ent.Channel{
			Name:     "hammer-test",
			Settings: &objects.ChannelSettings{HammerRetry: hammer},
		},
	}

	return &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		},
	}
}

func TestHammerCanRetryBudget(t *testing.T) {
	t.Run("hammerable error retries on same channel", func(t *testing.T) {
		outbound := newHammerOutbound(&objects.ChannelHammerRetry{MaxRetries: 5})
		allow, handled := outbound.hammerCanRetry(newUpstream429())
		assert.True(t, allow)
		assert.True(t, handled)
		assert.Equal(t, 1, outbound.state.HammerAttempts)
	})

	t.Run("attempts budget stops hammering", func(t *testing.T) {
		outbound := newHammerOutbound(&objects.ChannelHammerRetry{MaxRetries: 3})
		for i := 0; i < 3; i++ {
			_, handled := outbound.hammerCanRetry(newUpstream429())
			assert.True(t, handled)
		}
		allow, handled := outbound.hammerCanRetry(newUpstream429())
		assert.False(t, allow)
		assert.True(t, handled)
	})

	t.Run("consecutive hard failures trip the fuse", func(t *testing.T) {
		outbound := newHammerOutbound(&objects.ChannelHammerRetry{
			MaxRetries:                  100,
			ConsecutiveHardFailureLimit: 2,
			ErrorPatterns:               []objects.RetryableErrorPattern{{Pattern: "负载已经达到上限"}},
		})

		rateLimited := newUpstream500("当前模型 gpt-6-astra 负载已经达到上限，请稍后重试")

		// First hard failure: still hammering.
		allow, _ := outbound.hammerCanRetry(rateLimited)
		assert.True(t, allow)

		// A 429 resets the fuse.
		allow, _ = outbound.hammerCanRetry(newUpstream429())
		assert.True(t, allow)

		// Two consecutive hard failures after the reset: fuse trips.
		allow, _ = outbound.hammerCanRetry(rateLimited)
		assert.True(t, allow)
		allow, handled := outbound.hammerCanRetry(rateLimited)
		assert.False(t, allow)
		assert.True(t, handled)
	})

	t.Run("non-hammerable error is not handled", func(t *testing.T) {
		outbound := newHammerOutbound(&objects.ChannelHammerRetry{MaxRetries: 5})
		allow, handled := outbound.hammerCanRetry(errors.New("boom"))
		assert.False(t, allow)
		assert.False(t, handled)
	})

	t.Run("hammer disabled falls through", func(t *testing.T) {
		outbound := newHammerOutbound(nil)
		allow, handled := outbound.hammerCanRetry(newUpstream429())
		assert.False(t, allow)
		assert.False(t, handled)
	})
}

func TestHammerCanRetryInCanRetry(t *testing.T) {
	// The full CanRetry path: a hammer channel must retry an upstream 429 on
	// the same channel even though the default policy switches channels.
	outbound := newHammerOutbound(&objects.ChannelHammerRetry{MaxRetries: 5})
	assert.True(t, outbound.CanRetry(newUpstream429()))

	// A channel without hammer retry keeps the default 429 switch behaviour.
	outbound = newHammerOutbound(nil)
	assert.False(t, outbound.CanRetry(newUpstream429()))
}

func TestHammerSameChannelBudgetInterface(t *testing.T) {
	outbound := newHammerOutbound(&objects.ChannelHammerRetry{RetryDelayMs: 250})
	assert.True(t, outbound.OverridesSameChannelLimit())
	assert.Equal(t, 250*time.Millisecond, outbound.SameChannelRetryDelay())

	outbound = newHammerOutbound(nil)
	assert.False(t, outbound.OverridesSameChannelLimit())
	assert.Equal(t, time.Duration(0), outbound.SameChannelRetryDelay())
}

func TestHammerStateResetsOnChannelSwitch(t *testing.T) {
	outbound := newHammerOutbound(&objects.ChannelHammerRetry{MaxRetries: 5})

	// Burn most of the budget on channel A.
	for i := 0; i < 4; i++ {
		allow, handled := outbound.hammerCanRetry(newUpstream429())
		assert.True(t, allow)
		assert.True(t, handled)
	}
	assert.Equal(t, 4, outbound.state.HammerAttempts)
	assert.False(t, outbound.state.HammerStartedAt.IsZero())

	// Set up a second candidate so NextChannel has somewhere to go, then
	// switch — the hammer budget must reset.
	outbound.state.ChannelModelsCandidates = []*ChannelModelsCandidate{
		outbound.state.CurrentCandidate,
		newHammerOutbound(&objects.ChannelHammerRetry{MaxRetries: 5}).state.CurrentCandidate,
	}
	err := outbound.NextChannel(context.Background())
	require.NoError(t, err)
	assert.Zero(t, outbound.state.HammerAttempts)
	assert.True(t, outbound.state.HammerStartedAt.IsZero())
	assert.Zero(t, outbound.state.HammerConsecutiveHardFails)

	// The new channel starts with a full budget.
	allow, handled := outbound.hammerCanRetry(newUpstream429())
	assert.True(t, allow)
	assert.True(t, handled)
	assert.Equal(t, 1, outbound.state.HammerAttempts)
}

func TestHammerableFlagOnPerformanceRecord(t *testing.T) {
	// The performance middleware marks hammerable failures so auto-disable
	// can skip them. Verify the classification feeding that flag directly.
	hammer := &objects.ChannelHammerRetry{
		ErrorPatterns: []objects.RetryableErrorPattern{{Pattern: "负载已经达到上限"}},
	}

	// Upstream 429 and pattern-matched errors are hammerable.
	hammerable, _ := classifyHammerError(newUpstream429(), hammer)
	assert.True(t, hammerable)
	hammerable, _ = classifyHammerError(newUpstream500("当前模型 gpt-6-astra 负载已经达到上限，请稍后重试"), hammer)
	assert.True(t, hammerable)

	// Unrelated failures on the same hammer channel are NOT hammerable and
	// must still feed auto-disable (the channel may be genuinely broken).
	hammerable, _ = classifyHammerError(newUpstream500("internal server error"), hammer)
	assert.False(t, hammerable)
}
