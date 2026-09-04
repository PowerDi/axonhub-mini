package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestOpenaiMultiprotocolChannel_DefaultEndpoints(t *testing.T) {
	endpoints := DefaultEndpointsForChannelType(channel.TypeOpenaiMultiprotocol)
	require.Len(t, endpoints, 4)

	formats := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		formats = append(formats, ep.APIFormat)
	}
	require.Equal(t, []string{
		llm.APIFormatOpenAIChatCompletion.String(),
		llm.APIFormatOpenAICompletion.String(),
		llm.APIFormatOpenAIResponse.String(),
		llm.APIFormatAnthropicMessage.String(),
	}, formats)
}

func TestOpenaiMultiprotocolChannel_PrimaryOutboundIsChatCompletions(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Multiprotocol Channel").
		SetType(channel.TypeOpenaiMultiprotocol).
		SetBaseURL("https://api.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4o"}).
		SetDefaultTestModel("gpt-4o").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	_, ok := built.Outbound.(*openai.OutboundTransformer)
	require.True(t, ok, "TypeOpenaiMultiprotocol should create openai.OutboundTransformer")
	require.Equal(t, llm.APIFormatOpenAIChatCompletion, built.Outbound.APIFormat())
}

// TestOpenaiMultiprotocolChannel_NonPrimaryFormatsUseDedicatedOutbounds is the
// regression for the Xai-style special case in buildChannelWithOutbounds:
// without it, every default endpoint would be bound to the primary
// chat_completions transformer, sending chat payloads to completions,
// responses and anthropic endpoints.
func TestOpenaiMultiprotocolChannel_NonPrimaryFormatsUseDedicatedOutbounds(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })

	ctx := authz.WithTestBypass(context.Background())
	entity := client.Channel.Create().
		SetName("Multiprotocol Channel").
		SetType(channel.TypeOpenaiMultiprotocol).
		SetBaseURL("https://api.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4o"}).
		SetDefaultTestModel("gpt-4o").
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entity)
	require.NoError(t, err)

	require.Len(t, built.Outbounds, 4)

	// Primary format: the default endpoint shares the primary transformer.
	chat, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIChatCompletion.String())
	require.NoError(t, err)
	require.Same(t, built.Outbound, chat)

	// completions peer: dedicated completion transformer, not the primary.
	completion, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAICompletion.String())
	require.NoError(t, err)
	require.NotSame(t, built.Outbound, completion)
	require.Equal(t, llm.APIFormatOpenAICompletion, completion.APIFormat())

	// responses peer: dedicated responses transformer, not the primary.
	responsesOut, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIResponse.String())
	require.NoError(t, err)
	require.IsType(t, &responses.OutboundTransformer{}, responsesOut)
	require.NotSame(t, built.Outbound, responsesOut)
	require.Equal(t, llm.APIFormatOpenAIResponse, responsesOut.APIFormat())

	// anthropic peer: dedicated anthropic transformer, not the primary.
	anthropicOut, err := BuildOutboundByAPIFormat(built, llm.APIFormatAnthropicMessage.String())
	require.NoError(t, err)
	require.IsType(t, &anthropic.OutboundTransformer{}, anthropicOut)
	require.NotSame(t, built.Outbound, anthropicOut)
	require.Equal(t, llm.APIFormatAnthropicMessage, anthropicOut.APIFormat())
}
