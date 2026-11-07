//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestSelectAPIFormat(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
		{APIFormat: "openai/embeddings"},
		{APIFormat: "openai/image_generation"},
		{APIFormat: "openai/moderations"},
	}

	require.Equal(t, "openai/responses", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeChat}))
	require.Equal(t, "openai/embeddings", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeEmbedding}))
	require.Equal(t, "openai/image_generation", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeImage}))
	require.Equal(t, "openai/moderations", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeModeration}))

	geminiEndpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatGeminiContents.String()},
		{APIFormat: llm.APIFormatGeminiEmbedding.String()},
	}

	require.Equal(t, llm.APIFormatGeminiContents.String(), SelectAPIFormat(geminiEndpoints, &llm.Request{RequestType: llm.RequestTypeChat}))
	require.Equal(t, llm.APIFormatGeminiEmbedding.String(), SelectAPIFormat(geminiEndpoints, &llm.Request{RequestType: llm.RequestTypeEmbedding}))
	require.Equal(t, llm.APIFormatGeminiContents.String(), SelectAPIFormat(geminiEndpoints, &llm.Request{RequestType: llm.RequestTypeImage}))
}

func TestSelectAPIFormat_PrefersMatchingFormat(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
		{APIFormat: "openai/chat_completions"},
	}

	require.Equal(t, "openai/chat_completions", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
	}))
}

func TestSelectAPIFormat_PrefersResponsesProtocolMatchingInboundTransport(t *testing.T) {
	// Given both Responses endpoint protocols on one channel and a WS request
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIResponse.String()},
		{APIFormat: llm.APIFormatOpenAIResponseWebSocket.String()},
	}

	// When the candidate format is selected
	format := SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIResponseWebSocket,
	})

	// Then the exact downstream protocol wins instead of the first endpoint
	require.Equal(t, llm.APIFormatOpenAIResponseWebSocket.String(), format)
}

func TestResponsesEndpointProtocolMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source llm.APIFormat
		target llm.APIFormat
		want   string
	}{
		{name: "http to http", source: llm.APIFormatOpenAIResponse, target: llm.APIFormatOpenAIResponse, want: llm.APIFormatOpenAIResponse.String()},
		{name: "websocket to websocket", source: llm.APIFormatOpenAIResponseWebSocket, target: llm.APIFormatOpenAIResponseWebSocket, want: llm.APIFormatOpenAIResponseWebSocket.String()},
		{name: "websocket to http", source: llm.APIFormatOpenAIResponseWebSocket, target: llm.APIFormatOpenAIResponse, want: llm.APIFormatOpenAIResponse.String()},
		{name: "http to websocket", source: llm.APIFormatOpenAIResponse, target: llm.APIFormatOpenAIResponseWebSocket, want: llm.APIFormatOpenAIResponseWebSocket.String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := SelectAPIFormat([]objects.ChannelEndpoint{{APIFormat: tt.target.String()}}, &llm.Request{RequestType: llm.RequestTypeChat, APIFormat: tt.source})

			require.Equal(t, tt.want, format)
		})
	}
}

func TestSelectAPIFormat_FallsBackWhenNoMatch(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
	}

	require.Equal(t, "openai/responses", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
	}))
}

func TestSelectAPIFormat_Video(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/video"},
		{APIFormat: "seedance/video"},
	}

	require.Equal(t, "openai/video", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatOpenAIVideo,
	}))

	require.Equal(t, "seedance/video", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatSeedanceVideo,
	}))
}

func TestSelectAPIFormat_Compact(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
		{APIFormat: "openai/responses_compact"},
	}

	require.Equal(t, "openai/responses_compact", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeCompact,
		APIFormat:   llm.APIFormatOpenAIResponseCompact,
	}))
}

func TestSelectAPIFormat_AlphaSearchRequiresExplicitEndpoint(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	}

	require.Empty(t, SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
	}))
}

// A nil policy must reproduce SelectAPIFormat exactly — the backward
// compatibility contract for channels without per-model policies.
func TestSelectAPIFormatForModel_NilPolicyMatchesLegacy(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatAnthropicMessage.String()},
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	}

	req := &llm.Request{RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatAnthropicMessage}
	require.Equal(t, SelectAPIFormat(endpoints, req), SelectAPIFormatForModel(endpoints, req, nil))
}

func TestSelectAPIFormatForModel_ExcludeFallsBackToConversion(t *testing.T) {
	// The user scenario: a newapi channel exposing both protocols, where
	// DeepSeek may only use the OpenAI endpoint.
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatAnthropicMessage.String()},
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	}
	policy := &objects.ModelAPIFormatPolicy{
		Model:   "deepseek-chat",
		Exclude: []string{llm.APIFormatAnthropicMessage.String()},
	}

	// Inbound Anthropic request must fall back to the OpenAI endpoint
	// (protocol conversion) instead of passing through.
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), SelectAPIFormatForModel(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}, policy))

	// Inbound OpenAI request is unaffected.
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), SelectAPIFormatForModel(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
	}, policy))
}

func TestSelectAPIFormatForModel_AllowOverridesExclude(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatAnthropicMessage.String()},
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatGeminiContents.String()},
	}

	// Allow wins even though Exclude also lists the allowed format.
	policy := &objects.ModelAPIFormatPolicy{
		Model:   "claude-sonnet",
		Exclude: []string{llm.APIFormatGeminiContents.String()},
		Allow:   []string{llm.APIFormatGeminiContents.String()},
	}

	require.Equal(t, llm.APIFormatGeminiContents.String(), SelectAPIFormatForModel(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}, policy))
}

func TestSelectAPIFormatForModel_AllowRestrictsToAllowedSet(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatAnthropicMessage.String()},
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	}
	policy := &objects.ModelAPIFormatPolicy{
		Model: "gpt-4",
		Allow: []string{llm.APIFormatAnthropicMessage.String()},
	}

	// The inbound format is excluded from the allowed set, so the first
	// allowed endpoint is used instead.
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), SelectAPIFormatForModel(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
	}, policy))
}

func TestSelectAPIFormatForModel_ZeroEndpointsAfterFilter(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	}
	policy := &objects.ModelAPIFormatPolicy{
		Model: "deepseek-chat",
		Exclude: []string{
			llm.APIFormatOpenAIChatCompletion.String(),
			llm.APIFormatAnthropicMessage.String(),
		},
	}

	require.Empty(t, SelectAPIFormatForModel(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}, policy))
}

// TestPolicyStarvesRequest locks the capability-intersection semantics of the
// starvation predicate — the regression line for the "first endpoint" fallback
// hole where a policy leaving only non-capable endpoints (e.g. embeddings for
// a chat request) used to produce a non-empty, wrong-protocol format.
func TestPolicyStarvesRequest(t *testing.T) {
	// A typical openai channel surface: chat + embedding endpoints.
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatOpenAIEmbedding.String()},
	}
	chatReq := &llm.Request{RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatAnthropicMessage}

	t.Run("nil policy never starves", func(t *testing.T) {
		require.False(t, PolicyStarvesRequest(endpoints, chatReq, nil))
		// Even with zero endpoints a nil policy must not starve — this keeps
		// the legacy "unknown channel / no endpoints" behaviour untouched.
		require.False(t, PolicyStarvesRequest(nil, chatReq, nil))
	})

	t.Run("chat request with only embedding endpoints left starves", func(t *testing.T) {
		// Regression: the policy filters away the chat endpoint, and
		// SelectAPIFormat's first-endpoint fallback would return
		// "openai/embeddings" (non-empty, wrong protocol) — the predicate
		// must still report starvation.
		policy := &objects.ModelAPIFormatPolicy{
			Model:   "deepseek-chat",
			Exclude: []string{llm.APIFormatOpenAIChatCompletion.String()},
		}
		require.True(t, PolicyStarvesRequest(endpoints, chatReq, policy))
	})

	t.Run("chat request with a chat endpoint kept does not starve", func(t *testing.T) {
		policy := &objects.ModelAPIFormatPolicy{
			Model:   "deepseek-chat",
			Exclude: []string{llm.APIFormatAnthropicMessage.String()},
		}
		require.False(t, PolicyStarvesRequest(endpoints, chatReq, policy))
	})

	t.Run("allow list without any capable endpoint starves", func(t *testing.T) {
		policy := &objects.ModelAPIFormatPolicy{
			Model: "deepseek-chat",
			Allow: []string{llm.APIFormatOpenAIEmbedding.String()},
		}
		require.True(t, PolicyStarvesRequest(endpoints, chatReq, policy))
	})

	t.Run("allow list containing a capable endpoint does not starve", func(t *testing.T) {
		policy := &objects.ModelAPIFormatPolicy{
			Model: "deepseek-chat",
			Allow: []string{llm.APIFormatOpenAIChatCompletion.String()},
		}
		require.False(t, PolicyStarvesRequest(endpoints, chatReq, policy))
	})

	t.Run("all policy-permitted endpoints filtered starves even when non-empty", func(t *testing.T) {
		// No endpoint at all survives the policy filter.
		policy := &objects.ModelAPIFormatPolicy{
			Model:   "deepseek-chat",
			Exclude: []string{llm.APIFormatOpenAIChatCompletion.String(), llm.APIFormatOpenAIEmbedding.String()},
		}
		require.True(t, PolicyStarvesRequest(endpoints, chatReq, policy))
	})

	t.Run("unknown request type uses any permitted endpoint", func(t *testing.T) {
		// Mirrors SelectAPIFormat's behaviour for request types without a
		// capability set: any policy-permitted endpoint is usable.
		unknownReq := &llm.Request{RequestType: llm.RequestType("weird")}
		policy := &objects.ModelAPIFormatPolicy{
			Model:   "deepseek-chat",
			Exclude: []string{llm.APIFormatOpenAIChatCompletion.String()},
		}
		require.False(t, PolicyStarvesRequest(endpoints, unknownReq, policy))
		require.True(t, PolicyStarvesRequest(nil, unknownReq, policy))
	})
}

func TestSelectAPIFormat_DecisionsRequiresExplicitEndpoint(t *testing.T) {
	// Given a chat-only channel surface and a Decisions request
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	}

	// When the request is routed
	format := SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeDecisions,
		APIFormat:   llm.APIFormatOpenAIDecisions,
	})

	// Then no incompatible fallback format is selected
	require.Empty(t, format)
}

func TestSelectAPIFormat_SystemOneRequiresExplicitEndpoint(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatAnthropicMessage.String()},
	}

	require.Empty(t, SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatTypeSafeSystemOne,
	}))

	typesafeEndpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}

	require.Equal(t, llm.APIFormatTypeSafeSystemOne.String(), SelectAPIFormat(typesafeEndpoints, &llm.Request{
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatTypeSafeSystemOne,
	}))
}

func TestSpecifiedChannelSelector_DecisionsWithoutEndpointReturnsNoCandidate(t *testing.T) {
	// Given a specified channel with a model and only a Chat endpoint
	ctx := authz.WithTestBypass(context.Background())
	client := enttest.NewEntClient(t, "sqlite3", "file:specified-channel-decisions?mode=memory&_fk=1")
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	channelEntity, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("chat-only").
		SetBaseURL("https://example.test").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"decisions-model"}).
		SetDefaultTestModel("decisions-model").
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}}).
		Save(ctx)
	require.NoError(t, err)

	channelService := biz.NewChannelServiceForTest(client)
	t.Cleanup(channelService.Stop)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{Type: "Channel", ID: channelEntity.ID})

	// When the specified selector receives a forced Decisions request
	candidates, err := selector.Select(ctx, &llm.Request{
		Model:       "decisions-model",
		RequestType: llm.RequestTypeDecisions,
		APIFormat:   llm.APIFormatOpenAIDecisions,
	})

	// Then the incompatible channel is not retained with an empty API format
	require.NoError(t, err)
	require.Empty(t, candidates)
}
