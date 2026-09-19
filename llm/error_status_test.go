package llm

import (
	"net/http"
	"testing"
)

func TestInferStatusCode(t *testing.T) {
	tests := []struct {
		name   string
		detail ErrorDetail
		want   int
	}{
		{
			// The production case: Azure OpenAI emits this inside a 200 stream body.
			name:   "openai rate limit code",
			detail: ErrorDetail{Code: "rate_limit_exceeded", Type: "too_many_requests"},
			want:   http.StatusTooManyRequests,
		},
		{
			name:   "type only",
			detail: ErrorDetail{Type: "rate_limit_error"},
			want:   http.StatusTooManyRequests,
		},
		{
			name:   "case and space insensitive",
			detail: ErrorDetail{Code: "  Rate_Limit_Exceeded "},
			want:   http.StatusTooManyRequests,
		},
		{
			name:   "code wins over type",
			detail: ErrorDetail{Code: "invalid_request_error", Type: "api_error"},
			want:   http.StatusBadRequest,
		},
		{
			name:   "insufficient quota is 429",
			detail: ErrorDetail{Code: "insufficient_quota"},
			want:   http.StatusTooManyRequests,
		},
		{
			name:   "anthropic overloaded",
			detail: ErrorDetail{Type: "overloaded_error"},
			want:   529,
		},
		{
			name:   "unknown stays zero",
			detail: ErrorDetail{Code: "something_we_never_saw", Type: "stream_error"},
			want:   0,
		},
		{
			name:   "empty stays zero",
			detail: ErrorDetail{},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferStatusCode(tt.detail); got != tt.want {
				t.Fatalf("InferStatusCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewStreamResponseError(t *testing.T) {
	err := NewStreamResponseError(ErrorDetail{
		Message: "Your requests to gpt-6-astra have exceeded rate limit.",
		Code:    "rate_limit_exceeded",
		Type:    "too_many_requests",
	})

	if err.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want 429", err.StatusCode)
	}

	// With a status present, Error() must now carry the "Request failed:" prefix
	// that was missing from the reported production log line.
	got := err.Error()
	if want := "Request failed: Too Many Requests, "; len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("Error() = %q, want prefix %q", got, want)
	}
}

func TestNewStreamResponseError_UnknownKeepsZero(t *testing.T) {
	err := NewStreamResponseError(ErrorDetail{Message: "stream error", Type: "stream_error"})
	if err.StatusCode != 0 {
		t.Fatalf("StatusCode = %d, want 0 for unrecognized error", err.StatusCode)
	}
}
