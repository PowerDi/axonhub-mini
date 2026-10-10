package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestDefaultEndpointsForChannelType_ZenMuxProtocolDefaults(t *testing.T) {
	tests := []struct {
		name     string
		typ      channel.Type
		expected []objects.ChannelEndpoint
	}{
		{
			// This fork keeps zenmux/video available as a default endpoint on the
			// openai-compatible ZenMux type (upstream moved it to its own type only).
			name: "openai plus native video",
			typ:  channel.TypeZenmux,
			expected: append(
				append([]objects.ChannelEndpoint{}, DefaultEndpointsForChannelType(channel.TypeOpenai)...),
				objects.ChannelEndpoint{APIFormat: llm.APIFormatZenmuxVideo.String()},
			),
		},
		{name: "responses", typ: channel.TypeZenmuxResponses, expected: DefaultEndpointsForChannelType(channel.TypeNanogptResponses)},
		{name: "anthropic", typ: channel.TypeZenmuxAnthropic, expected: DefaultEndpointsForChannelType(channel.TypeMinimaxAnthropic)},
		{name: "gemini", typ: channel.TypeZenmuxGemini, expected: DefaultEndpointsForChannelType(channel.TypeGemini)},
		{name: "video", typ: channel.TypeZenmuxVideo, expected: []objects.ChannelEndpoint{{APIFormat: llm.APIFormatZenmuxVideo.String()}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, DefaultEndpointsForChannelType(tt.typ))
		})
	}
}

func TestValidateEndpointsForChannelType_ZenMuxVideoIsCustomOnAllZenMuxTypes(t *testing.T) {
	videoEndpoint := []objects.ChannelEndpoint{{APIFormat: llm.APIFormatZenmuxVideo.String()}}

	for _, channelType := range []channel.Type{
		channel.TypeZenmux,
		channel.TypeZenmuxResponses,
		channel.TypeZenmuxAnthropic,
		channel.TypeZenmuxGemini,
	} {
		t.Run(string(channelType), func(t *testing.T) {
			require.NoError(t, validateEndpointsForChannelType(channelType, videoEndpoint))
		})
	}

	require.Error(t, validateEndpointsForChannelType(channel.TypeOpenai, videoEndpoint))
}
