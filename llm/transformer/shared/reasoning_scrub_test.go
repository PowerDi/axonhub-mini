package shared

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestStripSignedReasoning_ClearsSignedFieldsKeepsPlainText(t *testing.T) {
	input := []llm.Message{
		{
			Role: "assistant",
			Content: llm.MessageContent{
				Content: lo.ToPtr("Here is the answer."),
			},
			ReasoningContent:   lo.ToPtr("let me think about this"),
			ReasoningSignature: lo.ToPtr("sig-abc"),
			ReasoningItems: []llm.ReasoningItem{
				{ID: "rs_1", Content: "let me think about this", Signature: "sig-abc"},
			},
		},
	}

	got, changed := StripSignedReasoning(input)

	require.True(t, changed)
	require.Len(t, got, 1)
	// Signed credentials removed.
	require.Nil(t, got[0].ReasoningItems)
	require.Nil(t, got[0].ReasoningSignature)
	// Plain-text reasoning and visible content preserved.
	require.NotNil(t, got[0].ReasoningContent)
	require.Equal(t, "let me think about this", *got[0].ReasoningContent)
	require.NotNil(t, got[0].Content.Content)
	require.Equal(t, "Here is the answer.", *got[0].Content.Content)
}

func TestStripSignedReasoning_ClearsRedactedReasoning(t *testing.T) {
	input := []llm.Message{
		{
			Role: "assistant",
			Content: llm.MessageContent{
				Content: lo.ToPtr("visible"),
			},
			RedactedReasoningContent: lo.ToPtr("encrypted-blob"),
		},
	}

	got, changed := StripSignedReasoning(input)

	require.True(t, changed)
	require.Len(t, got, 1)
	require.Nil(t, got[0].RedactedReasoningContent)
	require.Equal(t, "visible", *got[0].Content.Content)
}

func TestStripSignedReasoning_DropsMessageLeftEmpty(t *testing.T) {
	input := []llm.Message{
		{
			Role: "assistant",
			// Only signed reasoning, no text/content/tool calls.
			ReasoningSignature: lo.ToPtr("sig-only"),
			ReasoningItems: []llm.ReasoningItem{
				{ID: "rs_1", Signature: "sig-only"},
			},
		},
		{
			Role: "user",
			Content: llm.MessageContent{
				Content: lo.ToPtr("next turn"),
			},
		},
	}

	got, changed := StripSignedReasoning(input)

	require.True(t, changed)
	// The signed-only assistant message is dropped; the user message survives.
	require.Len(t, got, 1)
	require.Equal(t, "user", got[0].Role)
}

func TestStripSignedReasoning_KeepsMessageWithPlainReasoningOnly(t *testing.T) {
	// A message with plain-text reasoning but no content/tool calls must survive:
	// keeping the text is what satisfies "reasoning must be passed back" upstreams.
	input := []llm.Message{
		{
			Role:               "assistant",
			ReasoningContent:   lo.ToPtr("thought text"),
			ReasoningSignature: lo.ToPtr("sig-x"),
			ReasoningItems: []llm.ReasoningItem{
				{Content: "thought text", Signature: "sig-x"},
			},
		},
	}

	got, changed := StripSignedReasoning(input)

	require.True(t, changed)
	require.Len(t, got, 1)
	require.Nil(t, got[0].ReasoningItems)
	require.Nil(t, got[0].ReasoningSignature)
	require.NotNil(t, got[0].ReasoningContent)
	require.Equal(t, "thought text", *got[0].ReasoningContent)
}

func TestStripSignedReasoning_NoReasoningIsNoOp(t *testing.T) {
	input := []llm.Message{
		{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("hello")},
		},
		{
			Role:    "assistant",
			Content: llm.MessageContent{Content: lo.ToPtr("hi")},
		},
	}

	got, changed := StripSignedReasoning(input)

	require.False(t, changed)
	// Unchanged slice returned as-is.
	require.Len(t, got, 2)
}

func TestStripSignedReasoning_KeepsPlainReasoningContentOnly(t *testing.T) {
	// OpenAI Chat / DeepSeek style: only plain reasoning_content, no signature.
	// Nothing account-bound, so it must be a no-op.
	input := []llm.Message{
		{
			Role:             "assistant",
			Content:          llm.MessageContent{Content: lo.ToPtr("answer")},
			ReasoningContent: lo.ToPtr("plain chain of thought"),
		},
	}

	got, changed := StripSignedReasoning(input)

	require.False(t, changed)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].ReasoningContent)
	require.Equal(t, "plain chain of thought", *got[0].ReasoningContent)
}

func TestStripSignedReasoning_DoesNotMutateInput(t *testing.T) {
	input := []llm.Message{
		{
			Role:               "assistant",
			Content:            llm.MessageContent{Content: lo.ToPtr("answer")},
			ReasoningSignature: lo.ToPtr("sig-abc"),
			ReasoningItems: []llm.ReasoningItem{
				{ID: "rs_1", Content: "text", Signature: "sig-abc"},
			},
		},
	}

	_, changed := StripSignedReasoning(input)

	require.True(t, changed)
	// Original slice element must be untouched.
	require.NotNil(t, input[0].ReasoningSignature)
	require.Len(t, input[0].ReasoningItems, 1)
}

func TestScrubResponsesPortability_ClearsServerSideReferences(t *testing.T) {
	req := &llm.Request{
		Model:              "gpt-6",
		PreviousResponseID: lo.ToPtr("resp_123"),
		Store:              lo.ToPtr(true),
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
		},
	}

	changed := ScrubResponsesPortability(req)

	require.True(t, changed)
	require.Nil(t, req.PreviousResponseID)
	require.NotNil(t, req.Store)
	require.False(t, *req.Store)
}

func TestScrubResponsesPortability_ForcesStoreFalseWhenNil(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-6",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
		},
	}

	changed := ScrubResponsesPortability(req)

	require.True(t, changed)
	require.NotNil(t, req.Store)
	require.False(t, *req.Store)
}

func TestScrubResponsesPortability_DropsCompactionParts(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-6",
		Messages: []llm.Message{
			{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("keep me")},
						{Type: "compaction", Compact: &llm.CompactContent{
							ID:               "cmp_1",
							EncryptedContent: "blob",
						}},
					},
				},
			},
		},
		Store: lo.ToPtr(false),
	}

	changed := ScrubResponsesPortability(req)

	require.True(t, changed)
	parts := req.Messages[0].Content.MultipleContent
	require.Len(t, parts, 1)
	require.Equal(t, "text", parts[0].Type)
}

func TestScrubResponsesPortability_DropsPartsWithEncryptedCompact(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-6",
		Messages: []llm.Message{
			{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("visible")},
						{
							Type:    "compaction_summary",
							Compact: &llm.CompactContent{ID: "cmp_2", EncryptedContent: "enc"},
						},
					},
				},
			},
		},
		Store: lo.ToPtr(false),
	}

	changed := ScrubResponsesPortability(req)

	require.True(t, changed)
	require.Len(t, req.Messages[0].Content.MultipleContent, 1)
	require.Equal(t, "text", req.Messages[0].Content.MultipleContent[0].Type)
}

func TestScrubResponsesPortability_NoServerReferencesStillForcesStore(t *testing.T) {
	// Even with no previous_response_id, an unset Store must be pinned to false
	// so the next account is never asked to persist the turn.
	req := &llm.Request{
		Model: "gpt-6",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
		},
		Store: lo.ToPtr(false),
	}

	changed := ScrubResponsesPortability(req)

	// Store already false and nothing else to scrub -> no change.
	require.False(t, changed)
}

func TestScrubResponsesPortability_NilRequest(t *testing.T) {
	require.False(t, ScrubResponsesPortability(nil))
}
