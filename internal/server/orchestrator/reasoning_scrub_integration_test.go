package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	anthropictransformer "github.com/looplj/axonhub/llm/transformer/anthropic"
)

// TestChatCompletionOrchestrator_Process_StripsSignedReasoningAndRetriesSameChannel
// is the full-pipeline proof for the reasoning-scrub feature: a client replays a
// signed Anthropic thinking block, the first upstream rejects it with an
// "encrypted content could not be verified" 400, and the pipeline retries the
// SAME channel with a stripped body. The assertion is on the two captured
// upstream request bodies — both go to the same channel (same API key), the first
// still carries the account-bound signature, and the second (the same-channel
// retry) has it stripped while the plain reasoning text is kept.
//
// The channel must have Settings.ReasoningFilter enabled: the whole mechanism is
// gated on that flag, so with it off the 400 would pass straight through.
//
// Anthropic is used on both ends on purpose: it is the only inbound that parses a
// request-side thinking signature into the structured llm.Request, and its
// outbound re-serializes that signature verbatim, so the scrub is observable in
// the wire body rather than only in unit-level field assertions.
func TestChatCompletionOrchestrator_Process_StripsSignedReasoningAndRetriesSameChannel(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx = ent.NewContext(ctx, client)

	project := createTestProject(t, ctx, client)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)

	err := systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled:                 true,
		MaxChannelRetries:       1,
		MaxSingleChannelRetries: 1,
		RetryDelayMs:            0,
		LoadBalancerStrategy:    "adaptive",
	})
	require.NoError(t, err)

	// Standard base64 whose decoded bytes contain "claude" so GuessSignatureProvider
	// classifies it as Anthropic; otherwise the outbound would drop the whole
	// thinking block (prepareAnthropicReasoning) even before any scrub and the test
	// would be meaningless.
	signature := base64.StdEncoding.EncodeToString([]byte("claude-sonnet-thinking-signature-abc123"))
	const thinkingText = "let me add two and two"

	reqBody := map[string]any{
		"model":      "gpt-4",
		"max_tokens": 1024,
		"messages": []map[string]any{
			{"role": "user", "content": "What is 2+2?"},
			{"role": "assistant", "content": []map[string]any{
				{"type": "thinking", "thinking": thinkingText, "signature": signature},
				{"type": "text", "text": "4"},
			}},
			{"role": "user", "content": "Now what is 3+3?"},
		},
	}

	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	httpRequest := &httpclient.Request{
		Method: "POST",
		URL:    "/v1/messages",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	}

	// First upstream: 400 "encrypted content could not be verified" (real prod
	// shape). Second upstream: success. sequenceExecutor records every request so
	// both bodies can be inspected.
	anthropicErrBody := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"The encrypted content for item could not be verified. Reason: Encrypted content could not be decrypted or parsed."}}`)

	successResp := anthropictransformer.Message{
		ID:   "msg_recovered",
		Type: "message",
		Role: "assistant",
		Content: []anthropictransformer.MessageContentBlock{{
			Type: "text",
			Text: lo.ToPtr("6"),
		}},
		Model:      "claude-3-7-sonnet-latest",
		StopReason: lo.ToPtr("end_turn"),
		Usage: &anthropictransformer.Usage{
			InputTokens:  15,
			OutputTokens: 5,
		},
	}
	successBody, err := json.Marshal(successResp)
	require.NoError(t, err)

	executor := &sequenceExecutor{
		steps: []executorStep{
			{
				err: &httpclient.Error{
					Method:     "POST",
					URL:        "https://api.anthropic.com/v1/messages",
					StatusCode: 400,
					Status:     "400 Bad Request",
					Body:       anthropicErrBody,
				},
			},
			{
				resp: &httpclient.Response{
					StatusCode: 200,
					Body:       successBody,
					Headers:    http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	mkCandidate := func(name, key string) *ChannelModelsCandidate {
		ch, err := client.Channel.Create().
			SetType(channel.TypeAnthropic).
			SetName(name).
			SetBaseURL("https://api.anthropic.com").
			SetCredentials(objects.ChannelCredentials{APIKey: key}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetSettings(&objects.ChannelSettings{ReasoningFilter: true}).
			Save(ctx)
		require.NoError(t, err)

		outbound, err := anthropictransformer.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
		require.NoError(t, err)

		return &ChannelModelsCandidate{
			Channel:  &biz.Channel{Channel: ch, Outbound: outbound},
			Priority: 0,
			Models:   []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
		}
	}

	// A single channel: the recovery is a same-channel strip-and-retry, not a
	// failover. The account-bound credential is what the account rejects, so a
	// clean body has a real chance on the same channel.
	channelSelector := &staticChannelSelector{
		candidates: []*ChannelModelsCandidate{
			mkCandidate("Anthropic A", "key-a"),
		},
	}

	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:       channelSelector,
		Inbound:               anthropictransformer.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	ctx = contexts.WithProjectID(ctx, project.ID)

	result, err := orchestrator.Process(ctx, httpRequest)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)

	// Exactly two upstream attempts: the rejected one and the recovered one.
	require.Len(t, executor.requests, 2)

	// Both attempts hit the SAME channel: the recovery is a same-channel
	// strip-and-retry, not a failover to a different account.
	require.Equal(t,
		executor.requests[0].Headers.Get("X-API-Key"),
		executor.requests[1].Headers.Get("X-API-Key"),
		"both attempts must go to the same channel (same-channel retry, not failover)",
	)

	// First attempt replays the account-bound signature verbatim.
	require.True(t,
		bytes.Contains(executor.requests[0].Body, []byte(signature)),
		"first upstream attempt must carry the signed thinking block",
	)

	// On the same-channel retry the signature is gone, but the plain reasoning
	// text is kept (design: only signed credentials are stripped, plain reasoning
	// stays so providers that require it to be passed back are satisfied).
	require.False(t,
		bytes.Contains(executor.requests[1].Body, []byte(signature)),
		"the same-channel retry must strip the account-bound signature",
	)
	require.True(t,
		bytes.Contains(executor.requests[1].Body, []byte(thinkingText)),
		"plain reasoning text must be preserved after stripping the signature",
	)

	// Sanity: the persisted request recorded the successful recovery.
	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)
}
