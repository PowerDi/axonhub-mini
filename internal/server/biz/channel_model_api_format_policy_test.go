package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestValidateModelAPIFormatPolicies(t *testing.T) {
	chat := llm.APIFormatOpenAIChatCompletion.String()

	require.NoError(t, ValidateModelAPIFormatPolicies(nil))
	require.NoError(t, ValidateModelAPIFormatPolicies(&objects.ChannelSettings{}))

	// Valid policies.
	require.NoError(t, ValidateModelAPIFormatPolicies(&objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Exclude: []string{chat}},
		},
	}))

	// Empty model.
	require.Error(t, ValidateModelAPIFormatPolicies(&objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Exclude: []string{chat}},
		},
	}))

	// Duplicate model.
	require.Error(t, ValidateModelAPIFormatPolicies(&objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Exclude: []string{chat}},
			{Model: "deepseek-chat", Allow: []string{chat}},
		},
	}))

	// Unknown api format.
	require.Error(t, ValidateModelAPIFormatPolicies(&objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Exclude: []string{"carrier-pigeon"}},
		},
	}))
}

// setupRoutabilityDB creates a test channel service with a request context.
func setupRoutabilityDB(t *testing.T) (context.Context, *ChannelService) {
	t.Helper()

	svc, client := setupTestChannelService(t)
	t.Cleanup(func() { _ = client.Close() })

	ctx := ent.NewContext(context.Background(), client)

	return authz.WithTestBypass(ctx), svc
}

func createRoutabilityChannel(t *testing.T, ctx context.Context, client *ent.Client, name string, status channel.Status, settings *objects.ChannelSettings, endpoints []objects.ChannelEndpoint) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"deepseek-chat"}).
		SetDefaultTestModel("deepseek-chat").
		SetStatus(status).
		SetSettings(settings).
		SetEndpoints(endpoints).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

// TestValidateChannelModelRoutability_PolicyEditStarvesModel: a policy whose
// allow-list references only formats the channel does not expose is rejected
// when no other enabled channel serves the model.
func TestValidateChannelModelRoutability_PolicyEditStarvesModel(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)

	// No other channel exists, so a starving policy has no fallback.
	err := svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusEnabled, nil, &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "deepseek-chat")
}

// TestValidateChannelModelRoutability_EndpointRemovalStarvesPolicy: removing
// the endpoint a policy's allow-list depends on is rejected — the opposite
// direction of the same check.
func TestValidateChannelModelRoutability_EndpointRemovalStarvesPolicy(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)
	client := ent.FromContext(ctx)

	anthropic := llm.APIFormatAnthropicMessage.String()

	// Existing channel: anthropic endpoint + policy restricting deepseek-chat
	// to anthropic only.
	settings := &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Allow: []string{anthropic}},
		},
	}
	ch := createRoutabilityChannel(t, ctx, client, "newapi", channel.StatusEnabled, settings, []objects.ChannelEndpoint{
		{APIFormat: anthropic},
	})

	// Simulate the endpoint edit dropping the anthropic override: the resolved
	// endpoints fall back to the openai default set, which the allow-list
	// starves on.
	err := svc.ValidateChannelModelRoutability(ctx, ch.ID, ch.Type, ch.Status, nil, settings)
	require.Error(t, err)
	require.Contains(t, err.Error(), "deepseek-chat")
}

// TestValidateChannelModelRoutability_AllowedWhenOtherChannelServes: the same
// starving policy passes when another enabled channel still serves the model
// with a usable endpoint.
func TestValidateChannelModelRoutability_AllowedWhenOtherChannelServes(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)
	client := ent.FromContext(ctx)

	anthropic := llm.APIFormatAnthropicMessage.String()
	openaiChat := llm.APIFormatOpenAIChatCompletion.String()

	// Fallback channel without any policy.
	createRoutabilityChannel(t, ctx, client, "fallback", channel.StatusEnabled, nil, nil)

	err := svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusEnabled, []objects.ChannelEndpoint{
		{APIFormat: anthropic},
	}, &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{
				Model:   "deepseek-chat",
				Exclude: []string{openaiChat, anthropic},
			},
		},
	})
	require.NoError(t, err)
}

// TestValidateChannelModelRoutability_NoPoliciesSkipped: channels without
// policies skip the check entirely (backward compatibility).
func TestValidateChannelModelRoutability_NoPoliciesSkipped(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)

	require.NoError(t, svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusEnabled, nil, nil))
	require.NoError(t, svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusEnabled, nil, &objects.ChannelSettings{}))
}

// TestUpdateChannelRejectsStarvingPolicy: end-to-end mutation path — editing
// a channel's settings with a starving policy fails before persistence.
func TestUpdateChannelRejectsStarvingPolicy(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)
	client := ent.FromContext(ctx)

	ch := createRoutabilityChannel(t, ctx, client, "newapi", channel.StatusEnabled, nil, nil)

	// deepseek-chat is served only by this channel; allowing only a format the
	// channel does not expose starves it, so the update is rejected before
	// anything is persisted.
	_, err := svc.UpdateChannel(ctx, ch.ID, &ent.UpdateChannelInput{
		Settings: &objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "deepseek-chat")

	// The stored settings must be untouched.
	unchanged, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Nil(t, unchanged.Settings)
}

// TestSaveChannelEndpointsRejectsPolicyStarvation: removing the endpoint a
// policy relies on fails the save.
func TestSaveChannelEndpointsRejectsPolicyStarvation(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)
	client := ent.FromContext(ctx)

	anthropic := llm.APIFormatAnthropicMessage.String()

	settings := &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Allow: []string{anthropic}},
		},
	}
	ch := createRoutabilityChannel(t, ctx, client, "newapi", channel.StatusEnabled, settings, []objects.ChannelEndpoint{
		{APIFormat: anthropic},
	})

	_, err := svc.SaveChannelEndpoints(ctx, SaveChannelEndpointsInput{
		ChannelID: objects.GUID{ID: ch.ID},
		Endpoints: nil,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "deepseek-chat")
}

// TestValidateChannelModelRoutability_DisabledChannelSkipped: a channel that
// is not enabled carries no traffic, so a starving configuration on it is
// not rejected — cleanup edits must stay possible.
func TestValidateChannelModelRoutability_DisabledChannelSkipped(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)
	client := ent.FromContext(ctx)

	// Enabled channel with a starving policy: rejected.
	err := svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusEnabled, nil, &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
		},
	})
	require.Error(t, err)

	// The same configuration on a disabled channel: allowed.
	require.NoError(t, svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusDisabled, nil, &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
		},
	}))
	require.NoError(t, svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusArchived, nil, &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "deepseek-chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
		},
	}))

	// End-to-end: updating a disabled channel's settings with a starving
	// policy succeeds and persists.
	ch := createRoutabilityChannel(t, ctx, client, "disabled-newapi", channel.StatusDisabled, nil, nil)
	updated, err := svc.UpdateChannel(ctx, ch.ID, &ent.UpdateChannelInput{
		Settings: &objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, updated.Settings)
}

// TestCreateChannelValidatesPolicyOrder: an unknown api_format in a policy
// reports the structural error, not the routability error.
func TestCreateChannelValidatesPolicyOrder(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)

	_, err := svc.CreateChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "newapi",
		BaseURL:          lo.ToPtr("https://newapi.example.com/v1"),
		Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
		SupportedModels:  []string{"deepseek-chat"},
		DefaultTestModel: "deepseek-chat",
		Settings: &objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Exclude: []string{"carrier-pigeon"}},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported api_format")
	require.NotContains(t, err.Error(), "unroutable")
}

// TestDuplicateChannelRejectsStarvingPolicy: the duplicate path goes through
// createChannel and must reject a starving policy just like a direct create.
func TestDuplicateChannelRejectsStarvingPolicy(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)
	client := ent.FromContext(ctx)

	// The source channel serves a different model, so it cannot act as the
	// fallback for the starving policy the duplicate introduces.
	source, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("source").
		SetBaseURL("https://newapi.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.DuplicateChannel(ctx, source.ID, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "newapi-copy",
		BaseURL:          lo.ToPtr("https://newapi.example.com/v1"),
		Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
		SupportedModels:  []string{"deepseek-chat"},
		DefaultTestModel: "deepseek-chat",
		Settings: &objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "deepseek-chat")
}

// TestBulkCreateChannelsRejectsStarvingPolicy: the bulk import path also
// validates per-model policies.
func TestBulkCreateChannelsRejectsStarvingPolicy(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)

	_, err := svc.BulkCreateChannels(ctx, BulkCreateChannelsInput{
		Name:             "newapi",
		Type:             channel.TypeOpenai,
		BaseURL:          lo.ToPtr("https://newapi.example.com/v1"),
		APIKeys:          []string{"test-key"},
		SupportedModels:  []string{"deepseek-chat"},
		DefaultTestModel: "deepseek-chat",
		Settings: &objects.ChannelSettings{
			ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
				{Model: "deepseek-chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "deepseek-chat")
}

// TestValidateChannelModelRoutability_LowercaseFallbackChannel: a fallback
// channel with LowercaseModelID exposes lowercased entry keys; a mixed-case
// policy key must still find it instead of wrongly rejecting the edit.
func TestValidateChannelModelRoutability_LowercaseFallbackChannel(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)
	client := ent.FromContext(ctx)

	// Fallback channel serving "DeepSeek-Chat" under lowercase matching: its
	// entry keys are exposed lowercased ("deepseek-chat").
	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("fallback").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"DeepSeek-Chat"}).
		SetDefaultTestModel("DeepSeek-Chat").
		SetStatus(channel.StatusEnabled).
		SetSettings(&objects.ChannelSettings{LowercaseModelID: true}).
		Save(ctx)
	require.NoError(t, err)

	// The policy key uses mixed casing ("DeepSeek-Chat"), so the exact-match
	// lookup misses the fallback channel's lowercased "deepseek-chat" key and
	// only the lowercase fallback in otherChannelServesModel can find it.
	// Without that branch this edit would be wrongly rejected.
	require.NoError(t, svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusEnabled, nil, &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "DeepSeek-Chat", Allow: []string{llm.APIFormatGeminiContents.String()}},
		},
	}))

	// Control: a policy key that matches neither exactly nor lowercased is
	// still rejected — the lowercase fallback is a lookup aid, not a
	// catch-all.
	require.Error(t, svc.ValidateChannelModelRoutability(ctx, 0, channel.TypeOpenai, channel.StatusEnabled, nil, &objects.ChannelSettings{
		ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
			{Model: "GPT-Four", Allow: []string{llm.APIFormatGeminiContents.String()}},
		},
	}))
}
