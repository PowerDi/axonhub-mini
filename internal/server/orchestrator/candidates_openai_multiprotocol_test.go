package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// setupMultiprotocolPolicyChannel creates an enabled openai_multiprotocol
// channel whose default endpoints already expose both OpenAI and Anthropic
// protocols, with a per-model policy that excludes the Anthropic endpoint.
func setupMultiprotocolPolicyChannel(t *testing.T, name string) *biz.Channel {
	t.Helper()

	ctx, client := setupTest(t)

	anthropic := llm.APIFormatAnthropicMessage.String()
	chEnt, err := client.Channel.Create().
		SetType(channel.TypeOpenaiMultiprotocol).
		SetName(name).
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"deepseek-chat", "claude-sonnet"}).
		SetDefaultTestModel("deepseek-chat").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Exclude: []string{anthropic}},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	svc := newTestChannelServiceForChannels(client)
	ch, err := svc.GetChannel(ctx, chEnt.ID)
	require.NoError(t, err)

	return ch
}

// TestOpenaiMultiprotocolPolicyExcludesAnthropicEndpoint: the channel type
// exposes four default endpoints including anthropic/messages; a per-model
// policy that excludes the anthropic endpoint must steer an inbound
// anthropic request to the OpenAI chat_completions endpoint (protocol
// conversion) instead of the pass-through anthropic endpoint.
func TestOpenaiMultiprotocolPolicyExcludesAnthropicEndpoint(t *testing.T) {
	ch := setupMultiprotocolPolicyChannel(t, "Multiprotocol Policy Channel")

	entry, ok := ch.GetModelEntries()["deepseek-chat"]
	require.True(t, ok)
	require.NotNil(t, entry.Policy)

	endpoints := ch.ResolveEndpoints()
	require.Len(t, endpoints, 4)

	req := &llm.Request{
		Model:       "deepseek-chat",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}

	// Without the policy the anthropic endpoint wins (inbound match).
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), SelectAPIFormat(endpoints, req))
	// With the policy the OpenAI chat endpoint is used.
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), SelectAPIFormatForModel(endpoints, req, entry.Policy))

	// claude-sonnet has no policy: pass-through selection is unchanged.
	otherEntry, ok := ch.GetModelEntries()["claude-sonnet"]
	require.True(t, ok)
	require.Nil(t, otherEntry.Policy)
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), SelectAPIFormatForModel(endpoints, req, otherEntry.Policy))
}

// TestOpenaiMultiprotocolPolicyRequestConvertsViaChatCompletions is the
// end-to-end linkage: an inbound anthropic request for a model whose policy
// excludes the anthropic endpoint must be sent through the channel's
// chat_completions outbound (conversion), not the anthropic one.
func TestOpenaiMultiprotocolPolicyRequestConvertsViaChatCompletions(t *testing.T) {
	ch := setupMultiprotocolPolicyChannel(t, "Multiprotocol Policy Channel")

	entry, ok := ch.GetModelEntries()["deepseek-chat"]
	require.True(t, ok)

	req := chatRequest("deepseek-chat")

	candidate := &ChannelModelsCandidate{
		Channel: ch,
		Models:  []biz.ChannelModelEntry{entry},
	}
	candidate.APIFormat = SelectAPIFormat(ch.ResolveEndpoints(), req)
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), candidate.APIFormat)

	processor := &PersistentOutboundTransformer{
		state: &PersistenceState{
			OriginalModel:           "deepseek-chat",
			ChannelModelsCandidates: []*ChannelModelsCandidate{candidate},
			CurrentCandidateIndex:   0,
			CurrentModelIndex:       0,
			RequestExec:             &ent.RequestExecution{ID: 1}, // dummy to skip creation
		},
	}

	httpReq, err := processor.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, httpReq)
	// The wrapped outbound is the converted chat_completions transformer.
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), processor.wrapped.APIFormat().String())
}
