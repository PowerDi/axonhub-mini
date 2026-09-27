package shared

import "github.com/looplj/axonhub/llm"

// StripSignedReasoning removes account-bound reasoning credentials from a
// message history so the request can be replayed on a different upstream
// account. It returns the rewritten messages and whether anything changed.
//
// Reasoning credentials are not something the client generated; they are signed
// by the upstream account that produced them, and only that account accepts
// them. When AxonHub fails over to another channel/account, replaying these
// credentials makes every candidate reject the request (HTTP 400:
// "encrypted content could not be verified", "thinking_signature_invalid",
// "invalid_encrypted_content", ...). Dropping them lets the request complete on
// any account; the only cost is losing thinking continuity for that turn.
//
// What is stripped (account-bound):
//   - ReasoningItems      — carry an upstream item id + opaque Signature/encrypted content.
//   - ReasoningSignature  — scalar Anthropic signature / Gemini thought signature / OpenAI encrypted_content.
//   - RedactedReasoningContent — Anthropic redacted_thinking encrypted blob.
//
// What is kept (not account-bound):
//   - ReasoningContent / Reasoning — plain-text reasoning. Any upstream accepts
//     it, and keeping it also satisfies providers (e.g. DeepSeek) that demand
//     "the reasoning_text/reasoning_content ... must be passed back to the API"
//     while still dropping the signature they cannot verify. The plain text of
//     each stripped ReasoningItem is already aggregated into ReasoningContent by
//     the inbound transformers, so removing the items loses no text.
//
// A message that carried only signed reasoning (no text, no content, no tool
// calls) becomes empty after stripping; it is dropped so the upstream does not
// reject an empty-content message.
func StripSignedReasoning(messages []llm.Message) ([]llm.Message, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	changed := false
	filtered := make([]llm.Message, 0, len(messages))

	for _, msg := range messages {
		stripped, msgChanged := stripMessageSignedReasoning(msg)
		if !msgChanged {
			filtered = append(filtered, msg)
			continue
		}

		changed = true
		if !messageHasSubstanceAfterReasoningStrip(stripped) {
			// Nothing meaningful left after removing signed reasoning: drop the
			// message rather than send empty content the upstream will reject.
			continue
		}

		filtered = append(filtered, stripped)
	}

	if !changed {
		return messages, false
	}

	return filtered, true
}

// stripMessageSignedReasoning clears the account-bound reasoning fields on a
// single message. It operates on the value copy from range, so nil-ing pointer
// and slice fields never mutates the caller's backing arrays.
func stripMessageSignedReasoning(msg llm.Message) (llm.Message, bool) {
	changed := false

	if len(msg.ReasoningItems) > 0 {
		msg.ReasoningItems = nil
		changed = true
	}

	if msg.ReasoningSignature != nil {
		msg.ReasoningSignature = nil
		changed = true
	}

	if msg.RedactedReasoningContent != nil {
		msg.RedactedReasoningContent = nil
		changed = true
	}

	return msg, changed
}

// messageHasSubstanceAfterReasoningStrip reports whether a message still carries
// anything worth sending once its signed reasoning has been removed. Plain-text
// reasoning counts as substance because it is kept.
func messageHasSubstanceAfterReasoningStrip(msg llm.Message) bool {
	if msg.Content.Content != nil || len(msg.Content.MultipleContent) > 0 {
		return true
	}

	if len(msg.ToolCalls) > 0 || len(msg.InlineToolResults) > 0 {
		return true
	}

	if msg.ReasoningContent != nil || msg.Reasoning != nil {
		return true
	}

	if msg.Refusal != "" || msg.ToolCallID != nil || msg.Audio != nil {
		return true
	}

	return false
}

// ScrubResponsesPortability removes the OpenAI Responses server-side references
// that a different upstream account cannot resolve, so a failed-over request can
// be replayed anywhere. It is the deep counterpart to StripSignedReasoning and
// only applies to the Responses family (the caller must gate on outbound
// format). It mutates the passed request in place, so the caller must pass a
// request it owns (a clone), and returns whether anything changed.
//
// StripSignedReasoning still handles the reasoning fields; this function only
// removes the extra portability hazards:
//   - PreviousResponseID — points at a server-side conversation on the original account.
//   - Store — forced false; storing a reply the next account cannot reference is pointless.
//   - compaction content parts — encrypted context only the signing account can decrypt.
func ScrubResponsesPortability(req *llm.Request) bool {
	if req == nil {
		return false
	}

	changed := false

	if req.PreviousResponseID != nil {
		req.PreviousResponseID = nil
		changed = true
	}

	// Force store:false. A nil or true value means the upstream would persist
	// this turn for later reference, which no other account can honor.
	if req.Store == nil || *req.Store {
		storeFalse := false
		req.Store = &storeFalse
		changed = true
	}

	if len(req.Messages) == 0 {
		return changed
	}

	scrubbed := make([]llm.Message, len(req.Messages))
	for i, msg := range req.Messages {
		next, msgChanged := scrubMessageCompaction(msg)
		if msgChanged {
			changed = true
		}

		scrubbed[i] = next
	}

	if changed {
		req.Messages = scrubbed
	}

	return changed
}

// scrubMessageCompaction drops compaction content parts and any part carrying an
// encrypted compact blob from a single message. Compaction folds prior context
// into ciphertext only the signing account can decrypt, so it must go before a
// replay on another account.
func scrubMessageCompaction(msg llm.Message) (llm.Message, bool) {
	if len(msg.Content.MultipleContent) == 0 {
		return msg, false
	}

	changed := false
	kept := make([]llm.MessageContentPart, 0, len(msg.Content.MultipleContent))

	for _, part := range msg.Content.MultipleContent {
		if part.Type == "compaction" || part.Type == "compaction_summary" {
			changed = true
			continue
		}

		if part.Compact != nil && part.Compact.EncryptedContent != "" {
			changed = true
			continue
		}

		kept = append(kept, part)
	}

	if !changed {
		return msg, false
	}

	msg.Content.MultipleContent = kept

	return msg, true
}
