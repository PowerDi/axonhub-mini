package objects

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

// ChannelEndpoint represents an outbound API endpoint configuration within a Channel.
// Each endpoint specifies the upstream API format and an optional custom path override.
// Within a single channel, api_format must be unique.
type ChannelEndpoint struct {
	APIFormat string `json:"api_format"`
	Path      string `json:"path,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	Transport string `json:"transport,omitempty"`
}

const (
	ChannelEndpointTransportHTTP      = "http"
	ChannelEndpointTransportWebSocket = "websocket"
)

type (
	ProxyType   = httpclient.ProxyType
	ProxyConfig = httpclient.ProxyConfig
)

type ModelMapping struct {
	// From is the model name in the request.
	From string `json:"from"`

	// To is the model name in the provider.
	To string `json:"to"`
}

// ModelAPIFormatPolicy restricts which endpoint api_formats a model may use
// on a channel. Keyed by request model (same resolution as ModelMappings,
// i.e. matched against the request-side model name before any mapping is
// applied). An absent entry means all channel endpoints are allowed.
// When Allow is non-empty it takes precedence and Exclude is ignored.
type ModelAPIFormatPolicy struct {
	// Model is the request-side model name this policy applies to.
	Model string `json:"model"`

	// Exclude lists api_formats this model cannot use.
	Exclude []string `json:"exclude,omitempty"`

	// Allow, when non-empty, restricts the model to only these api_formats.
	// It overrides Exclude.
	Allow []string `json:"allow,omitempty"`
}

// GetModelAPIFormatPolicy returns the policy configured for the given request
// model, or nil when the model has no policy (all endpoints allowed).
// Lookup is by exact request-side model name; channels that lowercase model
// IDs must resolve the policy with the request model as matched by
// GetModelEntries (i.e. after lowercasing when enabled).
func (s *ChannelSettings) GetModelAPIFormatPolicy(model string) *ModelAPIFormatPolicy {
	if s == nil || model == "" || len(s.ModelAPIFormatPolicies) == 0 {
		return nil
	}

	for i := range s.ModelAPIFormatPolicies {
		if s.ModelAPIFormatPolicies[i].Model == model {
			return &s.ModelAPIFormatPolicies[i]
		}
	}

	return nil
}

// AllowsAPIFormat reports whether the policy permits the given api_format.
// A nil policy allows everything.
func (p *ModelAPIFormatPolicy) AllowsAPIFormat(apiFormat string) bool {
	if p == nil {
		return true
	}

	if len(p.Allow) > 0 {
		return containsString(p.Allow, apiFormat)
	}

	return !containsString(p.Exclude, apiFormat)
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}

	return false
}

type HeaderEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Override operation types.
const (
	OverrideOpSet          = "set"
	OverrideOpSetIfAbsent  = "set_if_absent"
	OverrideOpDelete       = "delete"
	OverrideOpRename       = "rename"
	OverrideOpCopy         = "copy"
	OverrideOpArrayAppend  = "array_append"
	OverrideOpArrayPrepend = "array_prepend"
	OverrideOpArrayInsert  = "array_insert"
	OverrideOpArrayRemove  = "array_remove"
)

// OverrideMatch defines a simple equality matcher for array_remove operations.
type OverrideMatch struct {
	// Path is resolved relative to each array item.
	Path string `json:"path"`
	// Eq is the value that removes the item when it matches.
	Eq string `json:"eq"`
}

// OverrideOperation defines a structured override operation for request body/header manipulation.
type OverrideOperation struct {
	Op        string `json:"op"`
	Path      string `json:"path,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Value     string `json:"value,omitempty"`
	Condition string `json:"condition,omitempty"`
	// Match identifies array items removed by array_remove.
	Match *OverrideMatch `json:"match,omitempty"`
	// Index is the target position for array_insert. Only used by array_insert.
	// Negative values count from the end (-1 = before last). Out-of-range values are clamped to [0, len].
	Index *int `json:"index,omitempty"`
	// Splat controls whether a JSON-array value is spread into the target array
	// (true: each element inserted individually) or inserted as a single nested element (false).
	// Only meaningful for array_append, array_prepend, and array_insert. Defaults to true.
	Splat *bool `json:"splat,omitempty"`
}

func HeaderEntriesToOverrideOperations(headers []HeaderEntry) []OverrideOperation {
	if len(headers) == 0 {
		return nil
	}

	ops := make([]OverrideOperation, 0, len(headers))
	for _, header := range headers {
		if header.Value == "__AXONHUB_CLEAR__" {
			ops = append(ops, OverrideOperation{Op: OverrideOpDelete, Path: header.Key})
			continue
		}

		ops = append(ops, OverrideOperation{Op: OverrideOpSet, Path: header.Key, Value: header.Value})
	}

	return ops
}

type TransformOptions struct {
	// ForceArrayInstructions forces the channel to accept array format for instructions.
	ForceArrayInstructions bool `json:"forceArrayInstructions"`

	// ForceArrayInputs forces the channel to accept array format for inputs.
	ForceArrayInputs bool `json:"forceArrayInputs"`

	// ReplaceDeveloperRoleWithSystem replaces developer role with system in messages for Bailian compatibility.
	ReplaceDeveloperRoleWithSystem bool `json:"replaceDeveloperRoleWithSystem"`

	// ReasoningEffortMapping maps inbound reasoning_effort values to outbound ones
	// for non-standard providers. The first entry whose From matches the effort value
	// wins; values not in the list pass through unchanged.
	// e.g. [{"from":"xhigh","to":"max"}] converts the unified "xhigh" level to "max"
	// for providers that only recognize "max".
	// Applied centrally by the orchestrator on the unified request before the outbound
	// transformer runs, so it affects every outbound protocol (chat completions,
	// responses, anthropic messages) for all clients. Strong-typed to mirror
	// ModelMapping; see llm.ReasoningEffortMapping.
	ReasoningEffortMapping []llm.ReasoningEffortMapping `json:"reasoningEffortMapping,omitempty"`
}

type ChannelSettings struct {
	// ExtraModelPrefix sets the channel accept the model with the extra prefix.
	// e.g. a channel
	// supported_modles is ["deepseek-chat", "deepseek-reasoner"]
	// extraModelPrefix is "deepseek"
	// then the model "deepseek-chat", "deepseek-reasoner", "deepseek/deepseek-chat", "deepseek/deepseek-reasoner"  will be accepted.
	// And if other channel support "deepseek/deepseek-chat", "deepseek/deepseek-reasoner" modles, the two channels can accept the request both.
	ExtraModelPrefix string `json:"extraModelPrefix"`

	// AutoTrimedModelPrefixes configures prefixes to automatically trim the model name when added to supported models.
	// e.g. a channel
	// supported_modles is ["deepseek-ai/deepseek-chat", "openai/gpt-4"]
	// autoTrimedModelPrefixes is ["openai", "deepseek"]
	// then the model "openai/gpt-4", "deepseek/deepseek-chat", "deepseek-chat", "gpt-4" will be accepted.
	AutoTrimedModelPrefixes []string `json:"autoTrimedModelPrefixes"`

	// ModelMappings add model alias for the model in the channels.
	// e.g. {"from": "deepseek-chat", "to": "deepseek/deepseek-chat"} will add a alias "deepseek-chat" for "deepseek/deepseek-chat".
	ModelMappings []ModelMapping `json:"modelMappings"`

	// ModelAPIFormatPolicies restrict which endpoint api_formats individual
	// models may use on this channel. Keyed by request model name (same
	// resolution as ModelMappings: the request-side name before any mapping).
	// Models without an entry may use every channel endpoint.
	// Example: {"model": "deepseek-chat", "exclude": ["anthropic/messages"]}
	// makes anthropic-protocol requests for deepseek-chat fall back to an
	// OpenAI endpoint and be converted, while other models keep pass-through.
	ModelAPIFormatPolicies []ModelAPIFormatPolicy `json:"modelApiFormatPolicies,omitempty"`

	// HideOriginalModels hides the original models from the model list when model mappings are configured.
	// When enabled, only the mapped model names (from field) will be exposed, not the actual model names (to field).
	HideOriginalModels bool `json:"hideOriginalModels"`

	// HideMappedModels hides the mapped models from the model list when model mappings are configured.
	// When enabled, only the original model names (from field) will be exposed, not the mapped model names (to field).
	HideMappedModels bool `json:"hideMappedModels"`

	// LowercaseModelID converts model name matching keys to lowercase.
	// When enabled, only RequestModel (used for matching) is lowercased; ActualModel
	// (sent to provider) preserves original casing. This enables cross-channel load
	// balancing where providers use different casing for the same model.
	LowercaseModelID bool `json:"lowercaseModelId"`

	// OverrideParameters sets the channel override the request body.
	// A json string.
	// e.g. {"max_tokens": 100}, {"temperature": 0.7}
	// Deprecated Use bodyOverrideOperations instead.
	OverrideParameters string `json:"overrideParameters"`

	// BodyOverrideOperations sets the channel override operations for the request body.
	// When present (including an empty array), it takes precedence over OverrideParameters.
	BodyOverrideOperations []OverrideOperation `json:"bodyOverrideOperations,omitempty"`

	// OverrideHeaders sets the channel override the request headers.
	// e.g. [{"key": "User-Agent", "value": "AxonHub"}]
	// Supported ops: set (default), delete, rename, copy.
	// Deprecated Use headerOverrideOperations instead.
	OverrideHeaders []HeaderEntry `json:"overrideHeaders"`

	// HeaderOverrideOperations sets the channel override operations for request headers.
	// When present (including an empty array), it takes precedence over OverrideHeaders.
	HeaderOverrideOperations []OverrideOperation `json:"headerOverrideOperations,omitempty"`

	// Proxy configuration for the channel. If not set, defaults to environment proxy type.
	Proxy *httpclient.ProxyConfig `json:"proxy,omitempty"`

	// TransformOptions configures the transform options for the channel.
	TransformOptions TransformOptions `json:"transformOptions"`

	// PassThroughUserAgent controls whether to pass through the original User-Agent header to upstream AI providers.
	// When set to nil, it inherits from the global system setting.
	// When set to true/false, it overrides the global setting.
	PassThroughUserAgent *bool `json:"passThroughUserAgent,omitempty"`

	// PassThroughBody controls whether to forward the original request body directly
	// to the upstream provider and the raw provider response/stream directly to the client
	// without re-serialization through the transform pipelines.
	// Only effective when the inbound and outbound API formats are identical.
	// When set to nil, it inherits from the global system setting.
	// When set to true/false, it overrides the global setting.
	PassThroughBody *bool `json:"passThroughBody,omitempty"`

	// RateLimit configures the upstream rate limit for the channel.
	// When configured, the load balancer will skip channels that have exceeded their rate limits.
	RateLimit *ChannelRateLimit `json:"rateLimit,omitempty"`

	// RetryableStatusCodes configures additional HTTP status codes that should
	// trigger retry for this channel. Default retryable codes (429 and 5xx) are
	// always handled by the retry policy even when this list is empty.
	RetryableStatusCodes []int `json:"retryableStatusCodes,omitempty"`

	// RetryableErrorPatterns configures additional error text patterns that should
	// trigger retry for this channel. When Regex is false, Pattern is matched as a
	// case-sensitive substring of the error text.
	RetryableErrorPatterns []RetryableErrorPattern `json:"retryableErrorPatterns,omitempty"`

	// HammerRetry enables contended-channel mode: rate-limit-shaped failures are
	// retried on the same channel at a high frequency to grab a concurrency slot
	// ("挤模式"). It overrides the default 429 behaviour of skipping same-channel
	// retry, and exempts the channel from 429 cooldowns and circuit-breaker
	// counting for matched errors.
	HammerRetry *ChannelHammerRetry `json:"hammerRetry,omitempty"`

	// ProviderQuota holds provider-specific quota collection credentials and
	// options. Fields are sensitive (e.g. auth cookies) and only exposed to
	// operators holding channel write permission.
	ProviderQuota *ChannelProviderQuotaSettings `json:"providerQuota,omitempty"`
}

// ChannelProviderQuotaSettings groups per-provider quota collection settings.
type ChannelProviderQuotaSettings struct {
	// CommandCode holds the quota collection settings for Command Code channels.
	CommandCode *CommandCodeQuotaSettings `json:"commandCode,omitempty"`
}

// CommandCodeQuotaSettings holds the credentials used to query the Command Code
// account quota. AuthCookie is the commandcode.ai session cookie (a
// "__Secure-commandcode_prod_.session_token" style value) sent to the internal
// billing endpoints.
type CommandCodeQuotaSettings struct {
	AuthCookie string `json:"authCookie,omitempty"`
}

// String redacts the auth cookie so settings never leak it into logs.
func (s CommandCodeQuotaSettings) String() string {
	if s.AuthCookie == "" {
		return "CommandCodeQuotaSettings{AuthCookie: \"\"}"
	}
	return "CommandCodeQuotaSettings{AuthCookie: <redacted>}"
}

type RetryableErrorPattern struct {
	Pattern string `json:"pattern"`
	Regex   bool   `json:"regex,omitempty"`
}

// ChannelHammerRetry configures contended-channel ("挤模式") retry behaviour.
// Such channels reject most requests with 429 or rate-limit-shaped 5xx errors
// and only succeed by retrying at high frequency until a concurrency slot
// opens up. All fields are channel-local; nothing is inherited from system
// settings.
type ChannelHammerRetry struct {
	// RetryDelayMs is the delay between same-channel hammer attempts in
	// milliseconds. Defaults to DefaultHammerRetryDelayMs when nil or <= 0.
	// Pointer so an unconfigured channel marshals as null (not the misleading
	// zero) through GraphQL.
	RetryDelayMs *int `json:"retryDelayMs,omitempty"`

	// MaxRetries is the maximum number of same-channel hammer attempts for a
	// single request. Defaults to DefaultHammerMaxRetries when nil or <= 0.
	MaxRetries *int `json:"maxRetries,omitempty"`

	// MaxDurationMs caps the total time spent hammering for a single request.
	// When exceeded, the last error is returned immediately. Defaults to
	// DefaultHammerMaxDurationMs when nil or <= 0. The request context still
	// applies (client cancellation wins).
	MaxDurationMs *int `json:"maxDurationMs,omitempty"`

	// ErrorPatterns matches rate-limit-shaped errors that do not arrive as a
	// standard 429 (e.g. relays wrapping upstream rate limits as 500 with a
	// "负载已经达到上限" body). 429 is always hammered and never needs a
	// pattern. Uses the same matching semantics as RetryableErrorPatterns.
	ErrorPatterns []RetryableErrorPattern `json:"errorPatterns,omitempty"`

	// ConsecutiveHardFailureLimit stops hammering after this many consecutive
	// non-429 failures (pattern-matched 5xx etc.), because a persistent
	// non-429 error usually means the channel is genuinely broken rather than
	// merely contended. 429 never counts towards this limit. Defaults to
	// DefaultHammerConsecutiveHardFailures when unset or <= 0.
	ConsecutiveHardFailureLimit *int `json:"consecutiveHardFailureLimit,omitempty"`
}

const (
	// DefaultHammerRetryDelayMs is the default delay between hammer attempts.
	DefaultHammerRetryDelayMs = 500
	// DefaultHammerMaxRetries is the default maximum hammer attempts per request.
	DefaultHammerMaxRetries = 50
	// DefaultHammerMaxDurationMs is the default hammering time budget per request.
	DefaultHammerMaxDurationMs = 120_000
	// DefaultHammerConsecutiveHardFailures is the default consecutive
	// non-429 failure fuse.
	DefaultHammerConsecutiveHardFailures = 3
)

// Enabled reports whether hammer retry is active for the channel.
func (h *ChannelHammerRetry) Enabled() bool {
	return h != nil
}

// EffectiveDelayMs returns the hammer retry delay after defaulting.
func (h *ChannelHammerRetry) EffectiveDelayMs() int {
	if h == nil || h.RetryDelayMs == nil || *h.RetryDelayMs <= 0 {
		return DefaultHammerRetryDelayMs
	}
	return *h.RetryDelayMs
}

// EffectiveMaxRetries returns the hammer max attempts after defaulting.
func (h *ChannelHammerRetry) EffectiveMaxRetries() int {
	if h == nil || h.MaxRetries == nil || *h.MaxRetries <= 0 {
		return DefaultHammerMaxRetries
	}
	return *h.MaxRetries
}

// EffectiveMaxDurationMs returns the hammer time budget after defaulting.
func (h *ChannelHammerRetry) EffectiveMaxDurationMs() int {
	if h == nil || h.MaxDurationMs == nil || *h.MaxDurationMs <= 0 {
		return DefaultHammerMaxDurationMs
	}
	return *h.MaxDurationMs
}

// EffectiveConsecutiveHardFailureLimit returns the hard-failure fuse after defaulting.
func (h *ChannelHammerRetry) EffectiveConsecutiveHardFailureLimit() int {
	if h == nil || h.ConsecutiveHardFailureLimit == nil || *h.ConsecutiveHardFailureLimit <= 0 {
		return DefaultHammerConsecutiveHardFailures
	}
	return *h.ConsecutiveHardFailureLimit
}

// MatchesHardFailure reports whether err is a hammerable non-429 error:
// an HTTP error whose text matches one of the configured ErrorPatterns.
func (h *ChannelHammerRetry) MatchesHardFailure(message string) bool {
	if h == nil || len(h.ErrorPatterns) == 0 {
		return false
	}

	for _, pattern := range h.ErrorPatterns {
		if pattern.Pattern == "" {
			continue
		}

		if pattern.Regex {
			if matched, regexErr := regexp.MatchString(pattern.Pattern, message); regexErr == nil && matched {
				return true
			}

			continue
		}

		if strings.Contains(message, pattern.Pattern) {
			return true
		}
	}

	return false
}

type ChannelRateLimit struct {
	RPM           *int64 `json:"rpm,omitempty"`           // Requests Per Minute, nil = unlimited
	TPM           *int64 `json:"tpm,omitempty"`           // Tokens Per Minute, nil = unlimited
	MaxConcurrent *int64 `json:"maxConcurrent,omitempty"` // Maximum concurrent requests, nil = unlimited

	// QueueSize controls the limiter mode when MaxConcurrent is set:
	//   nil / 0 = soft mode (count only, no blocking, no rejection — preserves PR #1322 scoring behaviour)
	//   > 0     = hard mode (FIFO wait queue with bounded capacity; excess requests rejected)
	// Has no effect when MaxConcurrent is unset or <= 0.
	QueueSize *int64 `json:"queueSize,omitempty"`

	// QueueTimeoutMs is the per-channel queue wait timeout in milliseconds.
	//   nil / 0 = no per-channel timeout (only the request context bounds the wait)
	//   > 0     = waiters that exceed this duration receive ErrChannelQueueTimeout
	// Only meaningful in hard mode (QueueSize > 0).
	QueueTimeoutMs *int64 `json:"queueTimeoutMs,omitempty"`
}

// DisabledAPIKey 记录被禁用的 API key 信息（敏感，按 credentials 同级保护）
// 注意：禁用判断以 Key 明文为主键。
type DisabledAPIKey struct {
	Key        string     `json:"key"`
	DisabledAt time.Time  `json:"disabledAt"`
	ErrorCode  int        `json:"errorCode"`
	Reason     string     `json:"reason,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}

// IsExpired reports whether a temporary API key disable has elapsed.
func (dk DisabledAPIKey) IsExpired() bool {
	return dk.ExpiresAt != nil && time.Now().After(*dk.ExpiresAt)
}

type ChannelCredentials struct {
	// APIKey is the API key for the channel, for the single key channel, e.g. Codex, Claude code, Antigravity.
	// It is kept for backward compatibility with existing data, recommend to use OAuth instead.
	APIKey string `json:"apiKey,omitempty"`

	// OAuth is the OAuth credentials for the channel, for the OAuth channel, e.g. Codex, Claude code, Antigravity.
	OAuth *OAuthCredentials `json:"oauth,omitempty"`

	// APIKeys is a list of API keys for the channel.
	// When multiple keys are provided, they will be used in a round-robin fashion.
	APIKeys []string `json:"apiKeys,omitempty"`

	// ManagementAPIKey is an optional provider management/console API key used only
	// for server-side quota checks (e.g. ZenMux). It is never attached to inference
	// requests and never exposed to clients beyond credential write APIs.
	ManagementAPIKey string `json:"managementApiKey,omitempty"`

	// Azure configuration for the channel.
	Azure *AzureCredential `json:"azure,omitempty"`

	// GCP is the GCP credentials for the channel.
	GCP *GCPCredential `json:"gcp,omitempty"`
}

// GetAllAPIKeys returns all API keys for the channel, combining APIKey and APIKeys fields.
// This ensures backward compatibility with old data that only has APIKey set.
func (c *ChannelCredentials) GetAllAPIKeys() []string {
	if c == nil {
		return nil
	}

	var keys []string

	// Add legacy APIKey if present (only if not OAuth credential)
	if c.APIKey != "" && !c.IsOAuth() {
		keys = append(keys, c.APIKey)
	}

	// Add new APIKeys
	keys = append(keys, c.APIKeys...)

	return keys
}

// OAuthCredentialRef identifies the OAuth credential of a channel in the
// auto-disable bookkeeping. A channel holds at most one OAuth credential, and
// its access token is replaced on every refresh, so a fixed sentinel is used as
// the stable identity instead of the token itself.
//
//nolint:gosec // Not a credential: a fixed sentinel that never authenticates anything.
const OAuthCredentialRef = "__oauth__"

// GetAllCredentialRefs returns the identities of every credential the channel
// can be disabled on. Key-based channels are identified by the API keys
// themselves; an OAuth channel is represented by the single OAuthCredentialRef
// so that auto-disable and scheduled recovery treat it as a one-key channel.
//
// This is deliberately separate from GetAllAPIKeys: the sentinel must never
// reach outbound requests, credential management UI, the channel tester or
// backups, all of which consume GetAllAPIKeys.
func (c *ChannelCredentials) GetAllCredentialRefs() []string {
	if c == nil {
		return nil
	}

	if c.IsOAuth() {
		return []string{OAuthCredentialRef}
	}

	return c.GetAllAPIKeys()
}

// GetEnabledCredentialRefs returns the credential refs that are not disabled.
func (c *ChannelCredentials) GetEnabledCredentialRefs(disabledKeys []DisabledAPIKey) []string {
	return filterDisabled(c.GetAllCredentialRefs(), disabledKeys)
}

// GetEnabledAPIKeys returns API keys that are not disabled.
func (c *ChannelCredentials) GetEnabledAPIKeys(disabledKeys []DisabledAPIKey) []string {
	return filterDisabled(c.GetAllAPIKeys(), disabledKeys)
}

// filterDisabled drops every candidate that carries an active disable record.
func filterDisabled(candidates []string, disabledKeys []DisabledAPIKey) []string {
	if len(disabledKeys) == 0 {
		return candidates
	}

	disabledSet := make(map[string]struct{}, len(disabledKeys))
	for _, dk := range disabledKeys {
		if dk.Key == "" || dk.IsExpired() {
			continue
		}

		disabledSet[dk.Key] = struct{}{}
	}

	enabled := make([]string, 0, len(candidates))
	for _, key := range candidates {
		if _, ok := disabledSet[key]; ok {
			continue
		}

		enabled = append(enabled, key)
	}

	return enabled
}

// IsOAuth returns true if OAuth credentials are configured and valid.
// It checks both the new OAuth field and legacy APIKey field for backward compatibility.
func (c *ChannelCredentials) IsOAuth() bool {
	if c == nil {
		return false
	}

	// Check new OAuth field first
	if c.OAuth != nil && c.OAuth.AccessToken != "" {
		return true
	}

	// Backward compatibility: check if APIKey contains OAuth JSON
	return isOAuthJSON(c.APIKey)
}

func (c *ChannelCredentials) ResolveOAuthCredentials() (*OAuthCredentials, error) {
	if c != nil && c.OAuth != nil && strings.TrimSpace(c.OAuth.AccessToken) != "" {
		return c.OAuth, nil
	}
	if c == nil {
		return oauth.ParseCredentialsJSON("")
	}
	return oauth.ParseCredentialsJSON(c.APIKey)
}

// isOAuthJSON checks if a string is an OAuth JSON credential.
func isOAuthJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") && strings.Contains(s, "access_token")
}

type OAuthCredentials = oauth.OAuthCredentials

type AzureCredential struct {
	// APIVersion is a optional version for the channel.
	APIVersion string `json:"apiVersion"`
}

type GCPCredential struct {
	Region    string `json:"region"`
	ProjectID string `json:"projectID"`
	JSONData  string `json:"jsonData"`
}

type GCPCredentialsJSON struct {
	Type                    string `json:"type" validate:"required"`
	ProjectID               string `json:"projectID" validate:"required"`
	PrivateKeyID            string `json:"privateKeyID" validate:"required"`
	PrivateKey              string `json:"privateKey" validate:"required"`
	ClientEmail             string `json:"clientEmail" validate:"required"`
	ClientID                string `json:"clientID" validate:"required"`
	AuthURI                 string `json:"authURI" validate:"required"`
	TokenURI                string `json:"tokenURI" validate:"required"`
	AuthProviderX509CertURL string `json:"authProviderX509CertURL" validate:"required"`
	ClientX509CertURL       string `json:"clientX509CertURL" validate:"required"`
	UniverseDomain          string `json:"universeDomain" validate:"required"`
}

type CapabilityPolicy string

const (
	CapabilityPolicyUnlimited CapabilityPolicy = "unlimited"
	CapabilityPolicyRequire   CapabilityPolicy = "require"
	CapabilityPolicyForbid    CapabilityPolicy = "forbid"
)

type ChannelPolicies struct {
	Stream CapabilityPolicy `json:"stream,omitempty"`

	// APIKeyAutoDisableRules are the channel's own auto-disable rules. They are
	// evaluated before the global retry policy and, when one matches, own the
	// failure outright. Channels without rules fall back to the global policy.
	APIKeyAutoDisableRules []APIKeyAutoDisableRule `json:"apiKeyAutoDisableRules,omitempty"`
}

type APIKeyAutoDisableAction string

const (
	APIKeyAutoDisableActionTemporary APIKeyAutoDisableAction = "temporary_disable"

	// APIKeyAutoDisableActionPermanentDelete disables the credential and then
	// removes it from the channel's credentials entirely.
	APIKeyAutoDisableActionPermanentDelete APIKeyAutoDisableAction = "permanent_disable_delete"

	// APIKeyAutoDisableActionPermanent disables the credential with no expiry but
	// keeps it on the channel, so an operator can inspect and re-enable it by hand.
	APIKeyAutoDisableActionPermanent APIKeyAutoDisableAction = "permanent_disable"

	// APIKeyAutoDisableActionUntilCron disables the credential until the first
	// occurrence of DisableUntilCron after the failure. It exists because quota
	// resets happen at fixed wall-clock times, which a relative duration cannot
	// express: the delay needed to reach 03:00 depends on when the failure hit.
	APIKeyAutoDisableActionUntilCron APIKeyAutoDisableAction = "disable_until_cron"
)

// APIKeyAutoDisableRule applies to one channel and matches status codes and/or
// error-message patterns. Empty conditions match any upstream error.
//
// Rules act on a single credential. Channels holding several API keys disable
// only the failing key and keep serving on the rest; the channel itself is
// disabled once every credential is unavailable, and recovers as soon as one
// becomes available again. An OAuth channel has exactly one credential
// (OAuthCredentialRef), so for it the two levels coincide.
type APIKeyAutoDisableRule struct {
	StatusCodes     []int                   `json:"statusCodes,omitempty"`
	KeywordPatterns []string                `json:"keywordPatterns,omitempty"`
	Times           int                     `json:"times"`
	Action          APIKeyAutoDisableAction `json:"action"`

	// DisableDurationMinutes applies to APIKeyAutoDisableActionTemporary. A nil
	// value disables the credential indefinitely.
	DisableDurationMinutes *int `json:"disableDurationMinutes,omitempty"`

	// DisableUntilCron and DisableUntilTimezone apply to
	// APIKeyAutoDisableActionUntilCron. DisableUntilCron uses the standard
	// 5-field crontab format; an empty timezone means UTC.
	DisableUntilCron     string `json:"disableUntilCron,omitempty"`
	DisableUntilTimezone string `json:"disableUntilTimezone,omitempty"`
}

// ParseOverrideOperations parses the override parameters string.
// Supports both legacy map format (JSON object) and new operation array format (JSON array).
// Legacy format is automatically converted to OverrideOperation slice.
func ParseOverrideOperations(raw string) ([]OverrideOperation, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "[]" {
		return nil, nil
	}

	if raw[0] == '[' {
		var ops []OverrideOperation
		if err := json.Unmarshal([]byte(raw), &ops); err != nil {
			return nil, fmt.Errorf("invalid override operations: %w", err)
		}

		return ops, nil
	}

	var legacy map[string]any
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return nil, fmt.Errorf("invalid override parameters: %w", err)
	}

	ops := make([]OverrideOperation, 0, len(legacy))
	for key, value := range legacy {
		if strVal, ok := value.(string); ok && strVal == "__AXONHUB_CLEAR__" {
			ops = append(ops, OverrideOperation{Op: OverrideOpDelete, Path: key})
		} else {
			// Convert value to string
			var strValue string

			switch v := value.(type) {
			case string:
				strValue = v
			default:
				strValue = fmt.Sprintf("%v", value)
			}

			ops = append(ops, OverrideOperation{Op: OverrideOpSet, Path: key, Value: strValue})
		}
	}

	return ops, nil
}

// SerializeOverrideOperations converts override operations to a JSON string for storage.
func SerializeOverrideOperations(ops []OverrideOperation) (string, error) {
	if len(ops) == 0 {
		return "[]", nil
	}

	data, err := json.Marshal(ops)
	if err != nil {
		return "", fmt.Errorf("failed to serialize override operations: %w", err)
	}

	return string(data), nil
}
