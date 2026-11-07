package biz

import (
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func isResponsesEndpoint(format string) bool {
	return format == llm.APIFormatOpenAIResponse.String() || format == llm.APIFormatOpenAIResponseWebSocket.String()
}

// NormalizeEndpoints returns canonical copies; it never changes persisted input.
// Legacy explicit transport takes precedence over the inherited Base URL scheme.
func NormalizeEndpoints(endpoints []objects.ChannelEndpoint, baseURL string) []objects.ChannelEndpoint {
	normalized := slices.Clone(endpoints)
	for i, ep := range normalized {
		if !isResponsesEndpoint(ep.APIFormat) {
			continue
		}
		resolvedBase := ep.BaseURL
		if resolvedBase == "" {
			resolvedBase = baseURL
		}
		if ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
			legacy := ep
			legacy.BaseURL = resolvedBase
			if endpointTransport(legacy) == objects.ChannelEndpointTransportWebSocket {
				ep.APIFormat = llm.APIFormatOpenAIResponseWebSocket.String()
			} else if ep.Transport == objects.ChannelEndpointTransportHTTP {
				// Keep explicit HTTP authoritative after omitting the legacy field.
				base := strings.TrimSpace(resolvedBase)
				switch {
				case strings.HasPrefix(strings.ToLower(base), "wss://"):
					ep.BaseURL = "https://" + base[len("wss://"):]
				case strings.HasPrefix(strings.ToLower(base), "ws://"):
					ep.BaseURL = "http://" + base[len("ws://"):]
				}
			}
		}
		ep.Transport = ""
		normalized[i] = ep
	}
	return normalized
}

// DefaultEndpoints applies stored Responses-family intent before the normal merge.
// Codex's family entries are authoritative; other providers retain their defaults.
func (c *Channel) DefaultEndpoints() []objects.ChannelEndpoint {
	if c.Channel == nil {
		return nil
	}
	defaults := slices.Clone(DefaultEndpointsForChannelType(c.Type))
	if c.Type != channel.TypeCodex {
		for i, ep := range defaults {
			if ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
				// Default transport follows the base URL even after an implicit WS
				// endpoint has been made explicit to persist canonical policy identities.
				source := *c.Channel
				source.Endpoints = slices.DeleteFunc(slices.Clone(c.Endpoints), func(ep objects.ChannelEndpoint) bool {
					return ep.APIFormat == llm.APIFormatOpenAIResponseWebSocket.String()
				})
				defaults[i].APIFormat = legacyResponsesFormat(&source)
			}
		}
		return defaults
	}
	allowed := make(map[string]bool)
	for _, ep := range NormalizeEndpoints(c.Endpoints, c.BaseURL) {
		if isResponsesEndpoint(ep.APIFormat) {
			allowed[ep.APIFormat] = true
		}
	}
	if len(allowed) == 0 {
		allowed[llm.APIFormatOpenAIResponseWebSocket.String()] = true
		allowed[llm.APIFormatOpenAIResponse.String()] = endpointTransport(objects.ChannelEndpoint{BaseURL: c.BaseURL}) != objects.ChannelEndpointTransportWebSocket
	}
	return slices.DeleteFunc(defaults, func(ep objects.ChannelEndpoint) bool {
		return isResponsesEndpoint(ep.APIFormat) && !allowed[ep.APIFormat]
	})
}

func legacyResponsesFormat(c *ent.Channel) string {
	for _, ep := range c.Endpoints {
		if ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
			return NormalizeEndpoints([]objects.ChannelEndpoint{ep}, c.BaseURL)[0].APIFormat
		}
	}
	for _, ep := range c.Endpoints {
		if ep.APIFormat == llm.APIFormatOpenAIResponseWebSocket.String() {
			return llm.APIFormatOpenAIResponse.String()
		}
	}
	return NormalizeEndpoints([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIResponse.String()}}, c.BaseURL)[0].APIFormat
}

// normalizeModelAPIFormatPolicies preserves legacy WS-only restrictions when
// Responses HTTP and WebSocket receive separate endpoint identities.
func normalizeModelAPIFormatPolicies(settings *objects.ChannelSettings, source *ent.Channel) *objects.ChannelSettings {
	if settings == nil || len(settings.ModelAPIFormatPolicies) == 0 || legacyResponsesFormat(source) != llm.APIFormatOpenAIResponseWebSocket.String() {
		return settings
	}
	changed := false
	copy := *settings
	copy.ModelAPIFormatPolicies = slices.Clone(settings.ModelAPIFormatPolicies)
	for i, policy := range copy.ModelAPIFormatPolicies {
		changed = changed || slices.Contains(policy.Allow, llm.APIFormatOpenAIResponse.String()) || slices.Contains(policy.Exclude, llm.APIFormatOpenAIResponse.String())
		copy.ModelAPIFormatPolicies[i].Allow = normalizeLegacyPolicyFormats(policy.Allow)
		copy.ModelAPIFormatPolicies[i].Exclude = normalizeLegacyPolicyFormats(policy.Exclude)
	}
	if !changed {
		return settings
	}
	return &copy
}

func normalizeLegacyPolicyFormats(formats []string) []string {
	formats = slices.Clone(formats)
	for i, format := range formats {
		if format == llm.APIFormatOpenAIResponse.String() {
			formats[i] = llm.APIFormatOpenAIResponseWebSocket.String()
		}
	}
	return formats
}

// ModelAPIFormatPolicy resolves the channel request-side policy, including
// normalization of persisted legacy Responses transport settings.
func (c *Channel) ModelAPIFormatPolicy(model string) *objects.ModelAPIFormatPolicy {
	if c == nil || c.Channel == nil {
		return nil
	}
	return normalizeModelAPIFormatPolicies(c.Settings, c.Channel).GetModelAPIFormatPolicy(model)
}

// normalizeUpdatedModelAPIFormatPolicies interprets echoed policies using the
// stored identity. Only new or actually edited policies use the submitted one.
func normalizeUpdatedModelAPIFormatPolicies(settings *objects.ChannelSettings, existing, proposed *ent.Channel) *objects.ChannelSettings {
	if settings == nil {
		return nil
	}
	copy := *settings
	copy.ModelAPIFormatPolicies = slices.Clone(settings.ModelAPIFormatPolicies)
	changed := false
	sameFormats := func(a, b []string) bool {
		return slices.Equal(slices.Compact(slices.Sorted(slices.Values(a))), slices.Compact(slices.Sorted(slices.Values(b))))
	}
	for i, policy := range copy.ModelAPIFormatPolicies {
		source := proposed
		if old := existing.Settings.GetModelAPIFormatPolicy(policy.Model); old != nil && sameFormats(old.Allow, policy.Allow) && sameFormats(old.Exclude, policy.Exclude) {
			source = existing
		}
		single := &objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{policy}}
		normalized := normalizeModelAPIFormatPolicies(single, source)
		if normalized != single {
			copy.ModelAPIFormatPolicies[i] = normalized.ModelAPIFormatPolicies[0]
			changed = true
		}
	}
	if !changed {
		return settings
	}
	return &copy
}

// Canonical policies must not be read as legacy aliases again on the next read.
// An explicit WS endpoint records the identity of an inherited WS default, with
// no extra endpoint capabilities and without changing the stored base URL.
func normalizePolicyEndpoints(source *ent.Channel) []objects.ChannelEndpoint {
	endpoints := NormalizeEndpoints(source.Endpoints, source.BaseURL)
	if source.Settings == nil || !slices.ContainsFunc(source.Settings.ModelAPIFormatPolicies, func(policy objects.ModelAPIFormatPolicy) bool {
		return slices.Contains(policy.Allow, llm.APIFormatOpenAIResponse.String()) || slices.Contains(policy.Exclude, llm.APIFormatOpenAIResponse.String())
	}) {
		return endpoints
	}
	if legacyResponsesFormat(source) != llm.APIFormatOpenAIResponseWebSocket.String() || slices.ContainsFunc(endpoints, func(ep objects.ChannelEndpoint) bool { return isResponsesEndpoint(ep.APIFormat) }) {
		return endpoints
	}
	if slices.ContainsFunc(DefaultEndpointsForChannelType(source.Type), func(ep objects.ChannelEndpoint) bool { return ep.APIFormat == llm.APIFormatOpenAIResponse.String() }) {
		endpoints = append(endpoints, objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAIResponseWebSocket.String()})
	}
	return endpoints
}
