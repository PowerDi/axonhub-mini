package orchestrator

import (
	"errors"
	"time"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// hammerConfigForChannel returns the channel's hammer retry configuration, or
// nil when hammer mode is disabled for the channel.
func hammerConfigForChannel(ch *biz.Channel) *objects.ChannelHammerRetry {
	if ch == nil || ch.Channel == nil || ch.Settings == nil {
		return nil
	}

	return ch.Settings.HammerRetry
}

// classifyHammerError classifies err for hammer retry. It returns (hammerable,
// hardFailure):
//
//   - hammerable: the failure is rate-limit-shaped (upstream 429, or a non-429
//     error matching the channel's ErrorPatterns) and may be retried on the
//     same channel,
//   - hardFailure: the failure is a pattern-matched non-429 — hammerable, but
//     counted towards the consecutive-hard-failure fuse because a persistent
//     non-429 error usually means the channel is genuinely broken.
//
// Local admission rejections (channel queue, local RPM) never reach upstream
// and are never hammerable.
func classifyHammerError(err error, hammer *objects.ChannelHammerRetry) (hammerable, hardFailure bool) {
	if err == nil || hammer == nil {
		return false, false
	}

	// Local admission rejections must not hammer: the request never reached
	// upstream and retrying immediately would just spin on the local limiter.
	if isChannelQueueError(err) || isLocalRPMExhaustedError(err) {
		return false, false
	}

	// Upstream 429: the bread-and-butter hammer case. Local admission errors
	// are also synthesized as 429-shaped, but the check above already
	// filtered them out.
	if httpclient.IsRateLimitErr(err) {
		return true, false
	}

	if hammer.MatchesHardFailure(hammerErrorMessage(err)) {
		return true, true
	}

	return false, false
}

// hammerErrorMessage extracts the text used for hammer pattern matching. The
// raw HTTP error's Error() string only carries method/URL/status, so the
// response body (where relays put their rate-limit text, e.g. new-api's
// "负载已经达到上限") is appended when present.
func hammerErrorMessage(err error) string {
	message := err.Error()

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && len(httpErr.Body) > 0 {
		message += "\n" + string(httpErr.Body)
	}

	var llmErr *llm.ResponseError
	if errors.As(err, &llmErr) && llmErr.Detail.Message != "" {
		message += "\n" + llmErr.Detail.Message
	}

	return message
}

// hammerBudgetExhausted reports whether the hammer time budget has been
// consumed for the current request. The budget starts at the first hammerable
// failure; a zero HammerStartedAt means no budget is running yet.
func hammerBudgetExhausted(startedAt time.Time, maxDurationMs int) bool {
	if startedAt.IsZero() {
		return false
	}

	return time.Since(startedAt) > time.Duration(maxDurationMs)*time.Millisecond
}

// hammerCanRetry implements the PersistentOutboundTransformer side of hammer
// retry: it classifies the error, updates the per-request hammer counters on
// the persistence state, and reports whether another same-channel hammer
// attempt is allowed.
//
// It returns (allow, handled):
//
//   - handled=false: hammer mode is off for the channel or the error is not
//     hammerable — the caller falls through to the default CanRetry logic,
//   - handled=true, allow=true: retry the same channel,
//   - handled=true, allow=false: the hammer budget (attempts, duration, or
//     consecutive-hard-failure fuse) is exhausted — stop retrying.
func (p *PersistentOutboundTransformer) hammerCanRetry(err error) (allow, handled bool) {
	hammer := hammerConfigForChannel(p.GetCurrentChannel())
	if hammer == nil {
		return false, false
	}

	hammerable, hardFailure := classifyHammerError(err, hammer)
	if !hammerable {
		return false, false
	}

	s := p.state
	if s == nil {
		return false, false
	}

	// Start (or keep) the per-request hammer budget clock.
	if s.HammerStartedAt.IsZero() {
		s.HammerStartedAt = time.Now()
	}

	if hardFailure {
		s.HammerConsecutiveHardFails++
	} else {
		// An upstream 429 resets the fuse: contention is alive, so a previous
		// hard-failure streak was transient.
		s.HammerConsecutiveHardFails = 0
	}

	s.HammerAttempts++

	if s.HammerAttempts >= hammer.EffectiveMaxRetries() {
		return false, true
	}

	if hammerBudgetExhausted(s.HammerStartedAt, hammer.EffectiveMaxDurationMs()) {
		return false, true
	}

	if s.HammerConsecutiveHardFails >= hammer.EffectiveConsecutiveHardFailureLimit() {
		return false, true
	}

	return true, true
}

// OverridesSameChannelLimit implements pipeline.SameChannelRetryBudget.
// Hammer channels track their own attempt budget, so the pipeline-wide
// maxSameChannelRetries limit must not gate their retries.
func (p *PersistentOutboundTransformer) OverridesSameChannelLimit() bool {
	return hammerConfigForChannel(p.GetCurrentChannel()) != nil
}

// SameChannelRetryDelay implements pipeline.SameChannelRetryBudget, returning
// the hammer channel's delay between same-channel attempts. A zero value
// makes the pipeline fall back to the global retry delay.
func (p *PersistentOutboundTransformer) SameChannelRetryDelay() time.Duration {
	hammer := hammerConfigForChannel(p.GetCurrentChannel())
	if hammer == nil {
		return 0
	}

	return time.Duration(hammer.EffectiveDelayMs()) * time.Millisecond
}

