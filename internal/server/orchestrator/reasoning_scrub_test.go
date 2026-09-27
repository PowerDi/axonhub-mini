package orchestrator

import (
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// respErr builds a *llm.ResponseError shaped like the ones the pipeline wraps
// upstream failures into, so shouldStripReasoning is exercised against the same
// Error() text seen in production.
func respErr(message, code, typ string) error {
	return &llm.ResponseError{
		StatusCode: 400,
		Detail: llm.ErrorDetail{
			Message: message,
			Code:    code,
			Type:    typ,
		},
	}
}

func TestShouldStripReasoning(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// --- real production samples (design doc §5) ---
		{
			name: "encrypted content space-form, null code",
			err:  respErr("The encrypted content for item cmp_047 could not be verified. Reason: Encrypted content could not be decrypted or parsed. [trace_id=abc]", "", "invalid_request_error"),
			want: true,
		},
		{
			name: "thinking_signature_invalid code",
			err:  respErr("The encrypted content for item rs_05 could not be verified. Reason: Encrypted content could not be decrypted or parsed.", "thinking_signature_invalid", "invalid_request_error"),
			want: true,
		},
		{
			name: "invalid_encrypted_content code, opaque message",
			err:  respErr("bad response status code 400 (request id: 202609)", "invalid_encrypted_content", "invalid_request_error"),
			want: true,
		},
		{
			name: "reasoning_text must be passed back, no negative word",
			err:  respErr("The `reasoning_text` in the thinking mode must be passed back to the API. (request_id: c33)", "", "invalid_request_error"),
			want: true,
		},
		{
			name: "reasoning_content must be passed back (anthropic messages)",
			err:  respErr("The `reasoning_content` in the thinking mode must be passed back to the API. (request_id: d82)", "", "api_error"),
			want: true,
		},
		// --- octopus baseline classes ---
		{
			name: "signature keyword",
			err:  errors.New("invalid signature for thinking block"),
			want: true,
		},
		{
			name: "previous_response_id",
			err:  respErr("previous_response_id not found", "", "invalid_request_error"),
			want: true,
		},
		{
			name: "resource ownership with vendor name between words",
			err:  respErr("The requested item was created under a different Azure OpenAI resource. [trace_id=x]", "", "invalid_request_error"),
			want: true,
		},
		{
			name: "conversation with rejection word",
			err:  respErr("conversation not found", "", "invalid_request_error"),
			want: true,
		},
		{
			name: "reasoning with rejection word",
			err:  respErr("reasoning item is invalid", "", "invalid_request_error"),
			want: true,
		},
		// --- must NOT match ---
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "rate limit",
			err:  respErr("Rate limit reached for this resource, please retry later", "rate_limit_exceeded", "rate_limit_error"),
			want: false,
		},
		{
			name: "reasoning effort is a legit parameter",
			err:  respErr("invalid value for reasoning effort", "", "invalid_request_error"),
			want: false,
		},
		{
			name: "ordinary 400 parameter error",
			err:  respErr("missing required parameter: model", "", "invalid_request_error"),
			want: false,
		},
		{
			name: "plain reasoning mention without rejection",
			err:  respErr("reasoning tokens: 1234", "", ""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldStripReasoning(tt.err); got != tt.want {
				t.Errorf("shouldStripReasoning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsPortabilityScrub(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "resource ownership triggers deep scrub",
			err:  respErr("created under a different Azure OpenAI resource", "", "invalid_request_error"),
			want: true,
		},
		{
			name: "another account",
			err:  errors.New("this item belongs to another account"),
			want: true,
		},
		{
			name: "plain signature error does not force deep scrub",
			err:  respErr("thinking signature invalid", "thinking_signature_invalid", ""),
			want: false,
		},
		{
			name: "resource word alone (quota) is not ownership",
			err:  respErr("insufficient quota for this resource", "", ""),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsPortabilityScrub(tt.err); got != tt.want {
				t.Errorf("needsPortabilityScrub() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsResourceOwnershipError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"different resource", "created under a different resource", true},
		{"another organization", "belongs to another organization", true},
		{"other project", "owned by other project", true},
		{"ownership word without presence word", "please use a different model", false},
		{"presence word without ownership word", "this resource is unavailable", false},
		{"neither", "internal server error", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isResourceOwnershipError(tt.message); got != tt.want {
				t.Errorf("isResourceOwnershipError(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

// signedReasoningMessage returns an assistant message carrying both plain-text
// reasoning and an account-bound signature/encrypted item, plus visible content.
func signedReasoningMessage() llm.Message {
	return llm.Message{
		Role: "assistant",
		Content: llm.MessageContent{
			Content: lo.ToPtr("the answer is 42"),
		},
		ReasoningContent:   lo.ToPtr("let me think step by step"),
		ReasoningSignature: lo.ToPtr("sig_abc123"),
		ReasoningItems: []llm.ReasoningItem{
			{ID: "rs_1", Content: "let me think step by step", Signature: "sig_abc123"},
		},
	}
}

// newScrubTestTransformer builds a transformer whose single candidate uses the
// given outbound format and ReasoningFilter flag, marked as already needing a
// scrub. applyReasoningScrub reads only the current candidate, so this is enough
// to exercise both scrub depths without a database.
func newScrubTestTransformer(reasoningFilter bool) *PersistentOutboundTransformer {
	candidate := &ChannelModelsCandidate{
		Channel: &biz.Channel{
			Channel: &ent.Channel{
				Settings: &objects.ChannelSettings{ReasoningFilter: reasoningFilter},
			},
		},
	}

	return &PersistentOutboundTransformer{
		state: &PersistenceState{
			ChannelModelsCandidates: []*ChannelModelsCandidate{candidate},
			CurrentCandidate:        candidate,
			ReasoningStripped:       true,
		},
	}
}

func TestApplyReasoningScrub_NoOpUntilMarked(t *testing.T) {
	p := newScrubTestTransformer(false)
	p.state.ReasoningStripped = false

	req := &llm.Request{
		APIFormat: llm.APIFormatAnthropicMessage,
		Messages:  []llm.Message{signedReasoningMessage()},
	}

	got := p.applyReasoningScrub(req, llm.APIFormatAnthropicMessage)

	require.Same(t, req, got, "unmarked request must pass through untouched")
	require.NotNil(t, got.Messages[0].ReasoningSignature)
}

func TestApplyReasoningScrub_ShallowStripsSignatureKeepsPlainText(t *testing.T) {
	p := newScrubTestTransformer(false)

	req := &llm.Request{
		APIFormat: llm.APIFormatAnthropicMessage,
		Messages:  []llm.Message{signedReasoningMessage()},
	}

	got := p.applyReasoningScrub(req, llm.APIFormatAnthropicMessage)

	require.NotSame(t, req, got, "scrub must clone, not mutate the input")
	// Input must be untouched (shared state.LlmRequest safety).
	require.NotNil(t, req.Messages[0].ReasoningSignature)
	require.Len(t, req.Messages[0].ReasoningItems, 1)

	// Signed fields gone, plain text + content kept.
	require.Nil(t, got.Messages[0].ReasoningSignature)
	require.Empty(t, got.Messages[0].ReasoningItems)
	require.NotNil(t, got.Messages[0].ReasoningContent)
	require.Equal(t, "let me think step by step", *got.Messages[0].ReasoningContent)
	require.Equal(t, "the answer is 42", *got.Messages[0].Content.Content)
}

func TestApplyReasoningScrub_DeepScrubGatedOnResponsesFormat(t *testing.T) {
	// ReasoningFilter on, but Anthropic outbound: deep scrub must NOT touch
	// server-side references (Anthropic has none); PreviousResponseID stays.
	p := newScrubTestTransformer(true)

	req := &llm.Request{
		APIFormat:          llm.APIFormatAnthropicMessage,
		PreviousResponseID: lo.ToPtr("resp_123"),
		Messages:           []llm.Message{signedReasoningMessage()},
	}

	got := p.applyReasoningScrub(req, llm.APIFormatAnthropicMessage)

	require.NotNil(t, got.PreviousResponseID, "deep scrub must not run for non-Responses outbound")
	require.Nil(t, got.Messages[0].ReasoningSignature, "shallow strip still runs")
}

func TestApplyReasoningScrub_DeepScrubOnResponsesWithFilter(t *testing.T) {
	p := newScrubTestTransformer(true)

	storeTrue := true
	req := &llm.Request{
		APIFormat:          llm.APIFormatOpenAIResponse,
		PreviousResponseID: lo.ToPtr("resp_123"),
		Store:              &storeTrue,
		Messages:           []llm.Message{signedReasoningMessage()},
	}

	got := p.applyReasoningScrub(req, llm.APIFormatOpenAIResponse)

	require.Nil(t, got.PreviousResponseID, "deep scrub clears previous_response_id")
	require.NotNil(t, got.Store)
	require.False(t, *got.Store, "deep scrub forces store:false")
	require.Nil(t, got.Messages[0].ReasoningSignature)
	// Input untouched.
	require.NotNil(t, req.PreviousResponseID)
	require.True(t, *req.Store)
}

func TestApplyReasoningScrub_DeepScrubSkippedWithoutFilterOrOwnership(t *testing.T) {
	// Responses outbound, but ReasoningFilter off and not a resource-ownership
	// error: shallow only, server-side refs preserved.
	p := newScrubTestTransformer(false)

	req := &llm.Request{
		APIFormat:          llm.APIFormatOpenAIResponse,
		PreviousResponseID: lo.ToPtr("resp_123"),
		Messages:           []llm.Message{signedReasoningMessage()},
	}

	got := p.applyReasoningScrub(req, llm.APIFormatOpenAIResponse)

	require.NotNil(t, got.PreviousResponseID, "shallow-only must keep server-side references")
	require.Nil(t, got.Messages[0].ReasoningSignature)
}

func TestApplyReasoningScrub_ResourceOwnershipLatchForcesDeepScrub(t *testing.T) {
	// ReasoningFilter off, but the latched resource-ownership flag must force
	// the deep scrub anyway (mirrors needsPortabilityScrub semantics).
	p := newScrubTestTransformer(false)
	p.state.ReasoningNeedsPortabilityScrub = true

	req := &llm.Request{
		APIFormat:          llm.APIFormatOpenAIResponse,
		PreviousResponseID: lo.ToPtr("resp_123"),
		Messages:           []llm.Message{signedReasoningMessage()},
	}

	got := p.applyReasoningScrub(req, llm.APIFormatOpenAIResponse)

	require.Nil(t, got.PreviousResponseID, "latched ownership flag forces deep scrub regardless of channel setting")
}

// canRetryTransformer builds a transformer with n plain candidates positioned at
// index idx, so CanRetry's HasMoreChannels() branch can be exercised.
func canRetryTransformer(n, idx int) *PersistentOutboundTransformer {
	candidates := make([]*ChannelModelsCandidate, n)
	for i := range candidates {
		candidates[i] = &ChannelModelsCandidate{
			Channel: &biz.Channel{Channel: &ent.Channel{}},
			Models:  []biz.ChannelModelEntry{{RequestModel: "m", ActualModel: "m"}},
		}
	}

	return &PersistentOutboundTransformer{
		state: &PersistenceState{
			ChannelModelsCandidates: candidates,
			CurrentCandidate:        candidates[idx],
			CurrentCandidateIndex:   idx,
		},
	}
}

func TestCanRetry_ReasoningErrorPrefersChannelSwitch(t *testing.T) {
	p := canRetryTransformer(2, 0) // more channels remain

	got := p.CanRetry(respErr("thinking signature invalid", "thinking_signature_invalid", "invalid_request_error"))

	require.False(t, got, "with more channels, reasoning error must route to NextChannel")
	require.True(t, p.state.ReasoningStripped, "detection must latch the strip flag")
}

func TestCanRetry_ReasoningErrorLastCandidateRetriesSameChannel(t *testing.T) {
	p := canRetryTransformer(2, 1) // last candidate

	got := p.CanRetry(respErr("thinking signature invalid", "thinking_signature_invalid", "invalid_request_error"))

	require.True(t, got, "on the last candidate, retry same channel so the stripped body still gets a chance")
	require.True(t, p.state.ReasoningStripped)
}

func TestCanRetry_ResourceOwnershipLatchesPortabilityScrub(t *testing.T) {
	p := canRetryTransformer(2, 0)

	p.CanRetry(respErr("The requested item was created under a different Azure OpenAI resource.", "", "invalid_request_error"))

	require.True(t, p.state.ReasoningStripped)
	require.True(t, p.state.ReasoningNeedsPortabilityScrub, "resource-ownership error must latch deep scrub")
}

func TestCanRetry_ReasoningDetectionIsOncePerRequest(t *testing.T) {
	p := canRetryTransformer(2, 0)

	// First reasoning error latches the flags and prefers a switch.
	require.False(t, p.CanRetry(respErr("thinking signature invalid", "thinking_signature_invalid", "")))
	require.True(t, p.state.ReasoningStripped)

	// A second reasoning error must not re-enter the reasoning branch (already
	// stripped); it falls through to the normal retryability logic. A 400 that
	// is not otherwise retryable and has no more models on the channel returns
	// false, confirming the reasoning branch did not short-circuit again.
	got := p.CanRetry(respErr("thinking signature invalid", "thinking_signature_invalid", ""))
	require.False(t, got, "second reasoning error falls through, not handled by the once-only branch")
}

func TestCanRetry_NonReasoningErrorLeavesFlagsUnset(t *testing.T) {
	p := canRetryTransformer(2, 0)

	p.CanRetry(respErr("missing required parameter: model", "", "invalid_request_error"))

	require.False(t, p.state.ReasoningStripped)
	require.False(t, p.state.ReasoningNeedsPortabilityScrub)
}
