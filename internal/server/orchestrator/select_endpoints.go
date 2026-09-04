package orchestrator

import (
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// SelectAPIFormat selects the most appropriate APIFormat from a channel's resolved endpoints
// based on the request type and inbound API format. Prefers an endpoint whose API format
// matches the inbound request format so that pass-through can be enabled when identical
// formats are used. Falls back to the first capable endpoint, then the first endpoint.
func SelectAPIFormat(endpoints []objects.ChannelEndpoint, req *llm.Request) string {
	if len(endpoints) == 0 {
		return ""
	}

	preferredFormat := string(req.APIFormat)
	allowed := llm.CapableAPIFormats(req.RequestType)

	if allowed != nil {
		if preferredFormat != "" {
			for _, ep := range endpoints {
				if _, ok := allowed[ep.APIFormat]; ok && ep.APIFormat == preferredFormat {
					return ep.APIFormat
				}
			}
		}

		for _, ep := range endpoints {
			if _, ok := allowed[ep.APIFormat]; ok {
				return ep.APIFormat
			}
		}

		if req.RequestType == llm.RequestTypeAlphaSearch {
			return ""
		}
	}

	return endpoints[0].APIFormat
}

// SelectAPIFormatForModel applies the channel's per-model API format policy
// before the standard SelectAPIFormat preference logic: endpoints the model's
// policy disallows are dropped, then the usual "prefer inbound format → first
// capable" selection runs over the remainder. A nil policy behaves exactly
// like SelectAPIFormat, preserving backward compatibility for channels
// without per-model policies.
func SelectAPIFormatForModel(endpoints []objects.ChannelEndpoint, req *llm.Request, policy *objects.ModelAPIFormatPolicy) string {
	if policy == nil {
		return SelectAPIFormat(endpoints, req)
	}

	filtered := make([]objects.ChannelEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if policy.AllowsAPIFormat(ep.APIFormat) {
			filtered = append(filtered, ep)
		}
	}

	return SelectAPIFormat(filtered, req)
}

// PolicyStarvesRequest reports whether the per-model policy leaves no
// endpoint that can serve the request type. This is the starvation signal
// used across selection and outbound paths: a starved entry must be skipped,
// never routed through SelectAPIFormatForModel — its legacy "first endpoint"
// fallback would silently pick a wrong-protocol endpoint (e.g. an embedding
// endpoint for a chat request) that the policy indirectly forbids.
//
// For request types without a known capability set (allowed == nil), any
// policy-permitted endpoint counts as usable, mirroring SelectAPIFormat.
func PolicyStarvesRequest(endpoints []objects.ChannelEndpoint, req *llm.Request, policy *objects.ModelAPIFormatPolicy) bool {
	if policy == nil {
		return false
	}
	if req == nil {
		// No request context: treat any permitted endpoint as usable.
		for _, ep := range endpoints {
			if policy.AllowsAPIFormat(ep.APIFormat) {
				return false
			}
		}
		return true
	}

	allowed := llm.CapableAPIFormats(req.RequestType)
	for _, ep := range endpoints {
		if !policy.AllowsAPIFormat(ep.APIFormat) {
			continue
		}

		if allowed == nil {
			return false
		}

		if _, ok := allowed[ep.APIFormat]; ok {
			return false
		}
	}

	return true
}
