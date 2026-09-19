package llm

import (
	"net/http"
	"strings"
)

// statusCodeByErrorCode maps provider error codes/types to the HTTP status they
// represent. Streaming errors arrive inside a 200 response body, so the
// transport layer has no status to report; without this mapping the resulting
// ResponseError carries StatusCode 0 and every status-based decision in the
// gateway (retryability, hammer classification, auto-disable thresholds,
// request-log reporting) silently treats the failure as unknown.
//
// Keys are matched case-insensitively against both ErrorDetail.Code and
// ErrorDetail.Type.
var statusCodeByErrorCode = map[string]int{
	// Rate limiting / quota.
	"rate_limit_exceeded": http.StatusTooManyRequests,
	"rate_limit_error":    http.StatusTooManyRequests,
	"too_many_requests":   http.StatusTooManyRequests,
	"insufficient_quota":  http.StatusTooManyRequests,
	"quota_exceeded":      http.StatusTooManyRequests,

	// Bad request.
	"invalid_request_error":   http.StatusBadRequest,
	"invalid_request":         http.StatusBadRequest,
	"context_length_exceeded": http.StatusBadRequest,
	"invalid_prompt":          http.StatusBadRequest,

	// Authentication / authorization.
	"authentication_error": http.StatusUnauthorized,
	"invalid_api_key":      http.StatusUnauthorized,
	"permission_error":     http.StatusForbidden,
	"permission_denied":    http.StatusForbidden,

	// Not found.
	"not_found_error": http.StatusNotFound,
	"model_not_found": http.StatusNotFound,

	// Payload too large.
	"request_too_large": http.StatusRequestEntityTooLarge,

	// Server-side failures.
	"api_error":             http.StatusInternalServerError,
	"internal_error":        http.StatusInternalServerError,
	"internal_server_error": http.StatusInternalServerError,
	"server_error":          http.StatusInternalServerError,
	"service_unavailable":   http.StatusServiceUnavailable,
	"engine_overloaded":     http.StatusServiceUnavailable,
	"overloaded":            http.StatusServiceUnavailable,

	// Anthropic uses a dedicated status for overload.
	"overloaded_error": 529,
}

// InferStatusCode derives the HTTP status a provider error represents from its
// code/type. It returns 0 when the error is not recognized, so callers can keep
// treating the status as unknown rather than inventing one.
//
// Code is consulted before Type because providers are more specific there
// (OpenAI sends code "rate_limit_exceeded" with type "too_many_requests"; both
// map to 429 here, but a mismatch should favour the narrower field).
func InferStatusCode(detail ErrorDetail) int {
	if status, ok := statusCodeByErrorCode[normalizeErrorKey(detail.Code)]; ok {
		return status
	}

	if status, ok := statusCodeByErrorCode[normalizeErrorKey(detail.Type)]; ok {
		return status
	}

	return 0
}

func normalizeErrorKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// NewStreamResponseError builds a ResponseError for a failure that arrived
// inside a streaming response body. The HTTP exchange itself succeeded (status
// 200) so there is no transport status to carry; the status is inferred from
// the provider's error code/type instead, leaving it at 0 when unrecognized.
func NewStreamResponseError(detail ErrorDetail) *ResponseError {
	return &ResponseError{
		StatusCode: InferStatusCode(detail),
		Detail:     detail,
	}
}
