package openai

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// TestParseStreamErrorEvent_InfersRateLimitStatus reproduces the production
// failure: an Azure OpenAI deployment returns HTTP 200, opens the SSE stream and
// then emits a rate-limit error event. The transport layer has no status to
// report, so before status inference the resulting ResponseError carried
// StatusCode 0 and the gateway treated an upstream 429 as an unknown failure.
func TestParseStreamErrorEvent_InfersRateLimitStatus(t *testing.T) {
	event := &httpclient.StreamEvent{
		Data: []byte(`{"error":{"code":"rate_limit_exceeded","message":"Your requests to gpt-6-astra for gpt-6-astra in eastus2 have exceeded rate limit.","type":"too_many_requests"}}`),
	}

	respErr := parseStreamErrorEvent(event)
	require.NotNil(t, respErr)
	require.Equal(t, http.StatusTooManyRequests, respErr.StatusCode)
	require.Equal(t, "rate_limit_exceeded", respErr.Detail.Code)
	require.Contains(t, respErr.Detail.Message, "exceeded rate limit")

	// The rendered message now leads with the status, which is what makes a
	// stream-borne 429 distinguishable from a statusless failure in the logs.
	require.Contains(t, respErr.Error(), "Request failed: Too Many Requests")
}

// TestParseStreamErrorEvent_SSEErrorEventInfersStatus covers the `event: error`
// branch, which builds its detail separately from the OpenAI-style branch.
func TestParseStreamErrorEvent_SSEErrorEventInfersStatus(t *testing.T) {
	event := &httpclient.StreamEvent{
		Type: "error",
		Data: []byte(`{"error":{"code":"insufficient_quota","message":"quota exhausted"}}`),
	}

	respErr := parseStreamErrorEvent(event)
	require.NotNil(t, respErr)
	require.Equal(t, http.StatusTooManyRequests, respErr.StatusCode)
}

// TestParseStreamErrorEvent_UnknownCodeKeepsStatusUnknown guards against
// inventing a status for errors we do not recognize.
func TestParseStreamErrorEvent_UnknownCodeKeepsStatusUnknown(t *testing.T) {
	event := &httpclient.StreamEvent{
		Data: []byte(`{"error":{"code":"weird_provider_specific","message":"boom"}}`),
	}

	respErr := parseStreamErrorEvent(event)
	require.NotNil(t, respErr)
	require.Equal(t, 0, respErr.StatusCode)
}
