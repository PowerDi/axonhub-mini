package objects

import "sort"

type ModelCardReasoning struct {
	Supported bool `json:"supported"`
	Default   bool `json:"default"`
}

type ModelCardModalities struct {
	// "text","image","video"
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type ModelCardCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

type ModelCardLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

type ModelCard struct {
	Reasoning ModelCardReasoning `json:"reasoning"`
	// ReasoningEfforts lists the reasoning effort levels the model accepts, using the
	// unified values from llm/reasoning.go. Nil means the levels are unknown.
	ReasoningEfforts []string            `json:"reasoningEfforts"`
	ToolCall         bool                `json:"toolCall"`
	Temperature      bool                `json:"temperature"`
	Modalities       ModelCardModalities `json:"modalities"`
	Vision           bool                `json:"vision"`
	Cost             ModelCardCost       `json:"cost"`
	Limit            ModelCardLimit      `json:"limit"`
	Knowledge        string              `json:"knowledge"`
	ReleaseDate      string              `json:"releaseDate"`
	LastUpdated      string              `json:"lastUpdated"`
}

type ModelSettings struct {
	DisableDeveloperSettingsInheritance bool                `json:"disableDeveloperSettingsInheritance"`
	Associations                        []*ModelAssociation `json:"associations"`
	LoadBalancerStrategy                string              `json:"loadBalancerStrategy"`
	TraceStickyMode                     string              `json:"traceStickyMode"`
}

const (
	ModelAssociationConditionFieldPromptTokens        = "prompt_tokens"
	ModelAssociationConditionFieldStream              = "stream"
	ModelAssociationConditionFieldRequestFormat       = "request_format"
	ModelAssociationConditionFieldReasoningEffort     = "reasoning_effort"
	ModelAssociationConditionFieldDailyTime           = "daily_time"
	ModelAssociationConditionFieldHasImage            = "has_image"
	ModelAssociationConditionFieldHasVideo            = "has_video"
	ModelAssociationConditionFieldHasDocument         = "has_document"
	ModelAssociationConditionFieldHasAudio            = "has_audio"
	ModelAssociationConditionFieldRequestHeader       = "request_header"
	ModelAssociationConditionFieldRequestHeaderPrefix = "request_header."
)

type ModelAssociation struct {
	// channel_model: the specified model id in the specified channel
	// channel_regex: the specified pattern in the specified channel
	// regex: the pattern for all channels
	// model: the specified model id
	// channel_tags_model: the specified model id in channels with specified tags (OR logic)
	// channel_tags_regex: the specified pattern in channels with specified tags (OR logic)
	Type             string                       `json:"type"`
	Priority         int                          `json:"priority"` // Lower value = higher priority, default 0
	Disabled         bool                         `json:"disabled"`
	When             *ModelAssociationWhen        `json:"when,omitempty"`
	ChannelModel     *ChannelModelAssociation     `json:"channelModel"`
	ChannelRegex     *ChannelRegexAssociation     `json:"channelRegex"`
	Regex            *RegexAssociation            `json:"regex"`
	ModelID          *ModelIDAssociation          `json:"modelId"`
	ChannelTagsModel *ChannelTagsModelAssociation `json:"channelTagsModel"`
	ChannelTagsRegex *ChannelTagsRegexAssociation `json:"channelTagsRegex"`
}

// NormalizeAssociationPriorities rewrites association priorities into dense
// tiers that start at 0. Rules that already shared a priority keep sharing one,
// and the tiers keep their relative order, so the failover order and the
// load-balancing groups the router reads out of Priority are unchanged while
// the values stay small and contiguous.
//
// The slice is reordered to match the priority order; nil entries are dropped.
func NormalizeAssociationPriorities(associations []*ModelAssociation) {
	items := make([]*ModelAssociation, 0, len(associations))
	for _, assoc := range associations {
		if assoc != nil {
			items = append(items, assoc)
		}
	}

	for i := len(items); i < len(associations); i++ {
		associations[i] = nil
	}

	if len(items) == 0 {
		return
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Priority < items[j].Priority
	})

	tier := 0
	previous := items[0].Priority

	for i, assoc := range items {
		if i > 0 && assoc.Priority != previous {
			tier++
			previous = assoc.Priority
		}

		assoc.Priority = tier
	}

	copy(associations, items)
}

type ModelAssociationWhen struct {
	Enabled   bool       `json:"enabled"`
	Condition *Condition `json:"condition,omitempty"`
}

type ExcludeAssociation struct {
	ChannelNamePattern string   `json:"channelNamePattern"`
	ChannelIds         []int    `json:"channelIds"`
	ChannelTags        []string `json:"channelTags"`
}

type ChannelModelAssociation struct {
	ChannelID int    `json:"channelId"`
	ModelID   string `json:"modelId"`
}

type ChannelRegexAssociation struct {
	ChannelID int    `json:"channelId"`
	Pattern   string `json:"pattern"`
}

type RegexAssociation struct {
	Pattern string                `json:"pattern"`
	Exclude []*ExcludeAssociation `json:"exclude"`
}

type ModelIDAssociation struct {
	ModelID string                `json:"modelId"`
	Exclude []*ExcludeAssociation `json:"exclude"`
}

type ChannelTagsModelAssociation struct {
	ChannelTags []string `json:"channelTags"`
	ModelID     string   `json:"modelId"`
}

type ChannelTagsRegexAssociation struct {
	ChannelTags []string `json:"channelTags"`
	Pattern     string   `json:"pattern"`
}
