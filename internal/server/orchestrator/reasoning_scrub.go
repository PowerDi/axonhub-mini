package orchestrator

import (
	"errors"
	"strings"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// reasoningErrorMessage extracts the best lowercased text to match reasoning
// credential errors against.
//
// The pipeline wraps upstream failures as *llm.ResponseError, whose Error()
// embeds the upstream message + code + type (see llm.ResponseError.Error), so
// err.Error() already carries everything we key on for that path. A bare
// *httpclient.Error only stringifies method+url+status and drops the body, so
// when we can unwrap a ResponseError we also fold in its Detail fields
// explicitly in case a future wrapper trims the chained message.
func reasoningErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(err.Error())

	var respErr *llm.ResponseError
	if errors.As(err, &respErr) {
		sb.WriteByte(' ')
		sb.WriteString(respErr.Detail.Message)
		sb.WriteByte(' ')
		sb.WriteString(respErr.Detail.Code)
		sb.WriteByte(' ')
		sb.WriteString(respErr.Detail.Type)
	}

	return strings.ToLower(sb.String())
}

// shouldStripReasoning reports whether an upstream error points specifically at
// account-bound reasoning credentials (signature / encrypted content / a
// server-side reference), meaning a replay on another account can only succeed
// once those are removed.
//
// It is deliberately narrow: stripping on any fast error would misclassify rate
// limits and ordinary parameter errors as credential problems and needlessly
// discard thinking continuity. The match set is the octopus baseline calibrated
// against real AxonHub production logs (see the design doc §5): it adds the
// space-form "encrypted content", the provider codes AxonHub actually returns,
// and a dedicated "must be passed back" class that the octopus criteria missed.
func shouldStripReasoning(err error) bool {
	if err == nil {
		return false
	}

	message := reasoningErrorMessage(err)

	// (1) Direct hits: credential keywords. Both the underscore form
	// (encrypted_content) and the space form the upstream actually prints
	// (encrypted content), plus the provider error codes seen in production.
	if containsAny(message,
		"encrypted_content", "encrypted content",
		"invalid_encrypted_content", "thinking_signature_invalid",
		"previous_response_id", "signature",
	) {
		return true
	}

	// (2) Resource-ownership errors: the referenced item belongs to a different
	// upstream resource/account. Matched by two co-occurring words rather than a
	// fixed phrase because the vendor name sits between them (see
	// isResourceOwnershipError).
	if isResourceOwnershipError(message) {
		return true
	}

	// (3) "must be passed back": some providers (DeepSeek via responses/messages)
	// reject with `reasoning_text`/`reasoning_content` in the thinking mode must
	// be passed back to the API. This carries no negative word and no
	// encrypted_content, so the octopus reasoning+rejected combination below
	// misses it entirely. It is its own class: the upstream wants the reasoning
	// echoed back, and the credential we sent is not one it accepts.
	if strings.Contains(message, "must be passed back") &&
		containsAny(message, "reasoning", "thinking") {
		return true
	}

	// (4) octopus baseline: conversation / reasoning paired with a rejection
	// word. "verified"/"could not be" are added for the production
	// "could not be verified" phrasing.
	rejected := containsAny(message,
		"invalid", "not found", "unknown", "expired", "verify", "verified",
		"decrypt", "mismatch", "does not exist", "not accessible",
		"does not belong", "could not be",
	)
	if strings.Contains(message, "conversation") {
		return rejected
	}

	return strings.Contains(message, "reasoning") &&
		!strings.Contains(message, "reasoning effort") &&
		rejected
}

// ownershipWords name the "belongs elsewhere" idea; presenceWords name the
// referenced entity's category. Both must appear: ownershipWords alone would
// pull in "please retry with another account"-style text, and presenceWords
// alone would trip on quota/rate-limit messages that mention "resource".
var (
	ownershipWords = []string{"different", "another", "other"}
	presenceWords  = []string{"resource", "account", "organization", "organisation", "project"}
)

// isResourceOwnershipError reports whether the error says the referenced item
// belongs to a different upstream resource/account.
//
// Matched by "both words present" rather than a fixed phrase: the vendor name
// sits between them, e.g. "The requested item was created under a different
// Azure OpenAI resource ..." has "different" and "resource" split by "Azure
// OpenAI". A false positive costs one extra scrubbed retry; a miss means the
// error is unrecoverable, so this tier stays deliberately loose.
func isResourceOwnershipError(message string) bool {
	if !containsAny(message, presenceWords...) {
		return false
	}

	return containsAny(message, ownershipWords...)
}

// reasoningFilterEnabled reports whether the channel opted in to reasoning
// scrubbing via Settings.ReasoningFilter. The whole mechanism (both the shallow
// signed-reasoning strip and the deep Responses portability scrub) is off unless
// this returns true, so a channel with the flag disabled lets the credential
// error pass through untouched.
func reasoningFilterEnabled(ch *biz.Channel) bool {
	if ch == nil || ch.Settings == nil {
		return false
	}

	return ch.Settings.ReasoningFilter
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}

	return false
}
