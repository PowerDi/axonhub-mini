package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// setupPolicyChannel creates an enabled openai-typed channel whose endpoints
// expose both OpenAI and Anthropic protocols, with per-model policies.
func setupPolicyChannel(t *testing.T, name string, supportedModels []string, policies []objects.ModelAPIFormatPolicy) *biz.Channel {
	t.Helper()

	ctx, client := setupTest(t)

	chEnt, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels(supportedModels).
		SetDefaultTestModel(supportedModels[0]).
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			ModelAPIFormatPolicies: policies,
		}).
		SetEndpoints([]objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatAnthropicMessage.String()},
		}).
		Save(ctx)
	require.NoError(t, err)

	svc := newTestChannelServiceForChannels(client)
	ch, err := svc.GetChannel(ctx, chEnt.ID)
	require.NoError(t, err)

	return ch
}

// setupStarvedPolicyChannel creates an enabled anthropic-typed channel whose
// only endpoint is anthropic/messages, with a policy that excludes it — the
// policy starves every chat-capable endpoint.
func setupStarvedPolicyChannel(t *testing.T, name string, supportedModels []string, policies []objects.ModelAPIFormatPolicy) *biz.Channel {
	t.Helper()

	ctx, client := setupTest(t)

	chEnt, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName(name).
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels(supportedModels).
		SetDefaultTestModel(supportedModels[0]).
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			ModelAPIFormatPolicies: policies,
		}).
		Save(ctx)
	require.NoError(t, err)

	svc := newTestChannelServiceForChannels(client)
	ch, err := svc.GetChannel(ctx, chEnt.ID)
	require.NoError(t, err)

	return ch
}

// TestLegacySelectionAppliesModelPolicy covers selectChannelCadidates: a
// model excluded from the channel's anthropic endpoint must resolve to the
// OpenAI endpoint (protocol conversion), while an unlisted model keeps the
// inbound-matching behaviour.
func TestLegacySelectionAppliesModelPolicy(t *testing.T) {
	ch := setupPolicyChannel(t, "Policy Channel", []string{"deepseek-chat", "claude-sonnet"}, []objects.ModelAPIFormatPolicy{
		{Model: "deepseek-chat", Exclude: []string{llm.APIFormatAnthropicMessage.String()}},
	})

	req := &llm.Request{
		Model:       "deepseek-chat",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}

	entry, ok := ch.GetModelEntries()["deepseek-chat"]
	require.True(t, ok)
	require.NotNil(t, entry.Policy)

	endpoints := ch.ResolveEndpoints()
	// Without the policy the anthropic endpoint would win (inbound match).
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), SelectAPIFormat(endpoints, req))
	// With the policy the OpenAI endpoint is used.
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), SelectAPIFormatForModel(endpoints, req, entry.Policy))

	// claude-sonnet has no policy: pass-through selection is unchanged.
	otherEntry, ok := ch.GetModelEntries()["claude-sonnet"]
	require.True(t, ok)
	require.Nil(t, otherEntry.Policy)
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), SelectAPIFormatForModel(endpoints, req, otherEntry.Policy))
}

// TestPopulateAPIFormatSingleEntryPolicy ensures populateAPIFormat applies
// the entry policy when the candidate carries exactly one model.
func TestPopulateAPIFormatSingleEntryPolicy(t *testing.T) {
	ch := setupPolicyChannel(t, "Policy Channel", []string{"deepseek-chat"}, []objects.ModelAPIFormatPolicy{
		{Model: "deepseek-chat", Exclude: []string{llm.APIFormatAnthropicMessage.String()}},
	})

	entry, ok := ch.GetModelEntries()["deepseek-chat"]
	require.True(t, ok)

	req := &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}

	candidates := []*ChannelModelsCandidate{
		{Channel: ch, Models: []biz.ChannelModelEntry{entry}},
	}

	populated := populateAPIFormat(candidates, req)
	require.Len(t, populated, 1)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), populated[0].APIFormat)
}

// TestPopulateAPIFormatMultipleEntriesFallbackUnfiltered verifies that a
// multi-entry candidate keeps the legacy channel-level format (policies are
// applied per entry at outbound time), and that a policy-free candidate is
// byte-for-byte identical to the legacy behaviour.
func TestPopulateAPIFormatMultipleEntriesFallbackUnfiltered(t *testing.T) {
	ch := setupPolicyChannel(t, "Policy Channel", []string{"deepseek-chat", "claude-sonnet"}, []objects.ModelAPIFormatPolicy{
		{Model: "deepseek-chat", Exclude: []string{llm.APIFormatAnthropicMessage.String()}},
	})

	entries := ch.GetModelEntries()
	deepseek := entries["deepseek-chat"]
	claude := entries["claude-sonnet"]

	req := &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}

	candidates := []*ChannelModelsCandidate{
		{Channel: ch, Models: []biz.ChannelModelEntry{deepseek, claude}},
	}

	populated := populateAPIFormat(candidates, req)
	require.Len(t, populated, 1)
	// Channel-level format still prefers the inbound format; per-entry
	// resolution happens later in apiFormatForEntry.
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), populated[0].APIFormat)
}

// TestAPIFormatForEntryPerModelPolicies is the core entry-granularity check:
// the same channel carries two models with different policies and the entry
// index decides which endpoint format is used.
func TestAPIFormatForEntryPerModelPolicies(t *testing.T) {
	ch := setupPolicyChannel(t, "Policy Channel", []string{"deepseek-chat", "claude-sonnet"}, []objects.ModelAPIFormatPolicy{
		{Model: "deepseek-chat", Exclude: []string{llm.APIFormatAnthropicMessage.String()}},
		// claude-sonnet only speaks anthropic on this channel.
		{Model: "claude-sonnet", Allow: []string{llm.APIFormatAnthropicMessage.String()}},
	})

	entries := ch.GetModelEntries()
	deepseek := entries["deepseek-chat"]
	claude := entries["claude-sonnet"]

	req := &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}

	candidate := &ChannelModelsCandidate{
		Channel: ch,
		Models:  []biz.ChannelModelEntry{deepseek, claude},
	}
	candidate.APIFormat = SelectAPIFormat(ch.ResolveEndpoints(), req)

	// deepseek-chat entry: candidate format (anthropic) is disallowed, so the
	// format falls back to the OpenAI endpoint (conversion).
	format, ok := apiFormatForEntry(candidate, 0, req)
	require.True(t, ok)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), format)

	// claude-sonnet entry: anthropic is allowed, candidate format stands.
	format, ok = apiFormatForEntry(candidate, 1, req)
	require.True(t, ok)
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), format)
}

// TestAPIFormatForEntryWithoutPolicyIsCandidateFormat ensures a policy-free
// entry always resolves to the candidate-level format regardless of index.
func TestAPIFormatForEntryWithoutPolicyIsCandidateFormat(t *testing.T) {
	ch := setupPolicyChannel(t, "Policy Channel", []string{"gpt-4"}, nil)

	entry, ok := ch.GetModelEntries()["gpt-4"]
	require.True(t, ok)
	require.Nil(t, entry.Policy)

	req := &llm.Request{RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatAnthropicMessage}

	candidate := &ChannelModelsCandidate{
		Channel: ch,
		Models:  []biz.ChannelModelEntry{entry},
	}
	candidate.APIFormat = SelectAPIFormat(ch.ResolveEndpoints(), req)

	format, ok := apiFormatForEntry(candidate, 0, req)
	require.True(t, ok)
	require.Equal(t, candidate.APIFormat, format)
}

// TestPopulateAPIFormatSkipsStarvedSingleEntryCandidate: when a single-entry
// candidate's policy filters out every usable endpoint, the candidate is
// skipped at selection time instead of failing (or passing through) at
// request time.
func TestPopulateAPIFormatSkipsStarvedSingleEntryCandidate(t *testing.T) {
	// An anthropic-typed channel exposes only the anthropic endpoint; a
	// policy excluding it starves the model completely.
	ch := setupStarvedPolicyChannel(t, "Starved Channel", []string{"deepseek-chat"}, []objects.ModelAPIFormatPolicy{
		{Model: "deepseek-chat", Exclude: []string{llm.APIFormatAnthropicMessage.String()}},
	})

	entry, ok := ch.GetModelEntries()["deepseek-chat"]
	require.True(t, ok)

	req := &llm.Request{RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatAnthropicMessage}

	candidates := []*ChannelModelsCandidate{
		{Channel: ch, Models: []biz.ChannelModelEntry{entry}},
	}

	populated := populateAPIFormat(candidates, req)
	require.Empty(t, populated)
}

// TestLegacySelectionSkipsStarvedPolicyEntry: the legacy supported_models
// path also skips channels whose policy starves the requested model.
func TestLegacySelectionSkipsStarvedPolicyEntry(t *testing.T) {
	ctx, client := setupTest(t)

	// An anthropic-typed channel has exactly one endpoint (anthropic); a
	// policy excluding it leaves the model with nothing.
	_, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Starved Channel").
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"deepseek-chat"}).
		SetDefaultTestModel("deepseek-chat").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Exclude: []string{llm.APIFormatAnthropicMessage.String()}},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	selector := NewDefaultSelector(channelService, modelService, newTestSystemService(client))

	// No Model entity exists, so selection falls back to the legacy
	// supported_models path.
	req := &llm.Request{
		Model:       "deepseek-chat",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}
	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Empty(t, result)
}

// TestTransformRequestRetryKeepsPolicyIsolation is the end-to-end P1-1
// regression: a multi-entry candidate where entry A carries a policy and
// entry B does not. After A's attempt fails and retry advances to B, B must
// resolve from the untouched channel-level format (pass-through), not from a
// format leaked by A's policy resolution.
// chatRequest builds a minimal chat request the real outbound transformers
// accept (they require at least one message).
func chatRequest(model string) *llm.Request {
	return &llm.Request{
		Model:       model,
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
		Messages: []llm.Message{{
			Role: "user",
			Content: llm.MessageContent{
				Content: lo.ToPtr("hello"),
			},
		}},
	}
}

func TestTransformRequestRetryKeepsPolicyIsolation(t *testing.T) {
	ch := setupPolicyChannel(t, "Policy Channel", []string{"deepseek-chat", "claude-sonnet"}, []objects.ModelAPIFormatPolicy{
		{Model: "deepseek-chat", Exclude: []string{llm.APIFormatAnthropicMessage.String()}},
	})

	entries := ch.GetModelEntries()
	deepseek := entries["deepseek-chat"]
	claude := entries["claude-sonnet"]

	req := chatRequest("deepseek-chat")

	candidate := &ChannelModelsCandidate{
		Channel: ch,
		Models:  []biz.ChannelModelEntry{deepseek, claude},
	}
	candidate.APIFormat = SelectAPIFormat(ch.ResolveEndpoints(), req)

	processor := &PersistentOutboundTransformer{
		state: &PersistenceState{
			OriginalModel:           "deepseek-chat",
			ChannelModelsCandidates: []*ChannelModelsCandidate{candidate},
			CurrentCandidateIndex:   0,
			CurrentModelIndex:       0,
			RequestExec:             &ent.RequestExecution{ID: 1}, // dummy to skip creation
		},
	}

	// First attempt: deepseek-chat's policy forbids the channel-level
	// anthropic format, so the outbound must be the OpenAI (converted) one.
	httpReq, err := processor.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, httpReq)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), processor.wrapped.APIFormat().String())

	// The shared candidate must NOT be polluted by entry A's resolution.
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), candidate.APIFormat)

	// Simulate the failure of attempt A: the model request is retryable
	// because another model remains on the candidate.
	require.True(t, processor.CanRetry(errors.New("boom")))
	require.NoError(t, processor.PrepareForRetry(context.Background()))
	require.Equal(t, 1, processor.state.CurrentModelIndex)

	// Second attempt: claude-sonnet has no policy and must use the
	// channel-level anthropic format (pass-through), not the openai format
	// resolved for deepseek-chat.
	httpReq, err = processor.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, httpReq)
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), processor.wrapped.APIFormat().String())

	// The candidate's channel-level format remains untouched after B too.
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), candidate.APIFormat)
}

// TestTransformRequestStarvedEntrySkipsToEndOfCandidate is the end-to-end
// P1-2 regression: an entry whose policy starves it of every usable endpoint
// is rejected with errEntryStarvedByPolicy, retry advances within the
// candidate, and when no entry remains the candidate is skipped (CanRetry
// false) so the pipeline switches channels.
func TestTransformRequestStarvedEntrySkipsToEndOfCandidate(t *testing.T) {
	// An anthropic-typed channel exposes only the anthropic endpoint; the
	// policy starves deepseek-chat while claude-sonnet keeps pass-through.
	ch := setupStarvedPolicyChannel(t, "Starved Channel", []string{"deepseek-chat", "claude-sonnet"}, []objects.ModelAPIFormatPolicy{
		{Model: "deepseek-chat", Exclude: []string{llm.APIFormatAnthropicMessage.String()}},
	})

	entries := ch.GetModelEntries()
	deepseek := entries["deepseek-chat"]
	claude := entries["claude-sonnet"]

	req := chatRequest("deepseek-chat")

	candidate := &ChannelModelsCandidate{
		Channel: ch,
		Models:  []biz.ChannelModelEntry{deepseek, claude},
	}
	candidate.APIFormat = SelectAPIFormat(ch.ResolveEndpoints(), req)

	processor := &PersistentOutboundTransformer{
		state: &PersistenceState{
			OriginalModel:           "deepseek-chat",
			ChannelModelsCandidates: []*ChannelModelsCandidate{candidate},
			CurrentCandidateIndex:   0,
			CurrentModelIndex:       0,
			RequestExec:             &ent.RequestExecution{ID: 1},
		},
	}

	// Attempt on the starved entry: TransformRequest must fail with the
	// starvation error instead of falling back to the primary (anthropic)
	// outbound, which the policy forbids.
	_, err := processor.TransformRequest(context.Background(), req)
	require.Error(t, err)
	require.ErrorIs(t, err, errEntryStarvedByPolicy)
	require.Contains(t, err.Error(), "deepseek-chat")

	// Retry is possible because another entry remains on the candidate.
	require.True(t, processor.CanRetry(err))
	require.NoError(t, processor.PrepareForRetry(context.Background()))

	// The surviving entry works normally through pass-through.
	httpReq, err := processor.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, httpReq)
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), processor.wrapped.APIFormat().String())

	// When the last entry itself is starved, no same-channel retry remains:
	// CanRetry is false so the pipeline moves to the next candidate.
	require.False(t, processor.CanRetry(fmt.Errorf("%w: model %q on channel %q", errEntryStarvedByPolicy, "claude-sonnet", ch.Name)))
}

// TestSpecifiedChannelSelectorAppliesModelPolicy is the P1-3 regression:
// the channel test flow must respect per-model policies, both for endpoint
// selection and for starvation errors.
func TestSpecifiedChannelSelectorAppliesModelPolicy(t *testing.T) {
	ctx, client := setupTest(t)

	anthropic := llm.APIFormatAnthropicMessage.String()
	policy := []objects.ModelAPIFormatPolicy{
		{Model: "deepseek-chat", Exclude: []string{anthropic}},
	}

	chEnt, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"deepseek-chat", "claude-sonnet"}).
		SetDefaultTestModel("deepseek-chat").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{ModelAPIFormatPolicies: policy}).
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: anthropic}}).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: chEnt.ID})

	req := &llm.Request{
		Model:       "deepseek-chat",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}

	// The policy excludes anthropic for deepseek-chat: the test request must
	// be steered to the converted OpenAI endpoint, exactly like real traffic.
	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), result[0].APIFormat)

	// A model whose policy starves every usable endpoint fails the test with
	// a clear error instead of probing a forbidden endpoint.
	starvingReq := &llm.Request{
		Model:       "deepseek-chat",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}
	// Reconfigure the channel so deepseek-chat's policy starves it: remove
	// the openai chat default endpoint by overriding with anthropic only and
	// excluding anthropic — deepseek has no endpoint left while claude keeps
	// the anthropic one. claude-sonnet must still work.
	_, err = client.Channel.UpdateOne(chEnt).
		SetSettings(&objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Exclude: []string{anthropic, llm.APIFormatOpenAIChatCompletion.String()}},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	channelService2 := newTestChannelServiceForChannels(client)
	selector2 := NewSpecifiedChannelSelector(channelService2, objects.GUID{ID: chEnt.ID})

	_, err = selector2.Select(ctx, starvingReq)
	require.Error(t, err)
	require.Contains(t, err.Error(), "deepseek-chat")
	require.Contains(t, err.Error(), "per-model policy")

	claudeReq := &llm.Request{
		Model:       "claude-sonnet",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}
	result, err = selector2.Select(ctx, claudeReq)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, anthropic, result[0].APIFormat)
}

// TestSpecifiedChannelSelectorLowercasePolicyKey is the R3 regression: on a
// LowercaseModelID channel, policies are keyed by the lowercased request
// model while direct entries keep their original casing — the channel test
// flow must resolve the policy the same way the request path does (exact
// first, then lowercase).
func TestSpecifiedChannelSelectorLowercasePolicyKey(t *testing.T) {
	ctx, client := setupTest(t)

	anthropic := llm.APIFormatAnthropicMessage.String()

	chEnt, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Lowercase Test Channel").
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"DeepSeek-Chat"}).
		SetDefaultTestModel("DeepSeek-Chat").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			LowercaseModelID: true,
			// Policy keyed by the lowercased request model, as clients send it.
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Exclude: []string{anthropic}},
			},
		}).
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: anthropic}}).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: chEnt.ID})

	// Test the direct (original-case) model: without the lowercase policy
	// resolution the policy would be nil and the request would pass through
	// the anthropic endpoint instead of converting through the OpenAI one.
	req := &llm.Request{
		Model:       "DeepSeek-Chat",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}
	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), result[0].APIFormat)
}

// TestSpecifiedChannelSelectorExplicitAPIFormatOverridesPolicy is the R4
// regression: an explicit test apiFormat probes exactly that endpoint even
// when the per-model policy would reroute or starve the request, and a
// channel without the requested endpoint fails with a clear error.
func TestSpecifiedChannelSelectorExplicitAPIFormatOverridesPolicy(t *testing.T) {
	ctx, client := setupTest(t)

	anthropic := llm.APIFormatAnthropicMessage.String()

	chEnt, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Explicit Endpoint Test Channel").
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"deepseek-chat"}).
		SetDefaultTestModel("deepseek-chat").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Exclude: []string{anthropic}},
			},
		}).
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: anthropic}}).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)

	req := &llm.Request{
		Model:       "deepseek-chat",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}

	// Without an explicit format the policy reroutes to the OpenAI endpoint.
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: chEnt.ID})
	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), result[0].APIFormat)

	// With an explicit anthropic format the test probes anthropic anyway:
	// the tester picked the endpoint, the policy must not reroute it.
	explicit := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: chEnt.ID})
	explicit.APIFormat = anthropic
	result, err = explicit.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, anthropic, result[0].APIFormat)
	// The entry policy is stripped so the outbound keeps the requested
	// format at request time instead of re-applying the filter.
	require.Nil(t, result[0].Models[0].Policy)

	// A format the channel does not expose fails with a clear error rather
	// than silently probing a different endpoint.
	missing := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: chEnt.ID})
	missing.APIFormat = llm.APIFormatGeminiContents.String()
	_, err = missing.Select(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no "+llm.APIFormatGeminiContents.String()+" endpoint")
}
