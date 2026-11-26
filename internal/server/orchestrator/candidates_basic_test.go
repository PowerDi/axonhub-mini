package orchestrator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// TestDefaultChannelSelector_Select_SingleChannel tests selection when only one channel is available.
func TestDefaultChannelSelector_Select_SingleChannel(t *testing.T) {
	ctx, client := setupTest(t)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Single Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, ch.ID, result[0].Channel.ID)
}

// TestDefaultSelector_Select tests DefaultSelector returns all enabled channels supporting the model.
func TestDefaultSelector_Select(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)
	selector := NewDefaultSelector(channelService, modelService, systemService)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return 3 enabled channels (exclude disabled one)
	require.Len(t, result, 3)

	// Verify disabled channel is not included
	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	require.NotContains(t, channelIDs, channels[3].ID, "Disabled channel should not be included")
	require.Contains(t, channelIDs, channels[0].ID, "High weight channel should be included")
	require.Contains(t, channelIDs, channels[1].ID, "Medium weight channel should be included")
	require.Contains(t, channelIDs, channels[2].ID, "Low weight channel should be included")
}

// TestDefaultChannelSelector_Select_NoChannelsAvailable tests error when no channels are available.
func TestDefaultChannelSelector_Select_NoChannelsAvailable(t *testing.T) {
	ctx, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Empty(t, result) // Should return empty slice, not error
}

// TestDefaultChannelSelector_Select_ModelNotSupported tests when requested model is not supported.
func TestDefaultChannelSelector_Select_ModelNotSupported(t *testing.T) {
	ctx, client := setupTest(t)

	// Create channel that doesn't support the requested model
	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Limited Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService)

	req := &llm.Request{
		Model: "gpt-4", // This model is not supported by the channel
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Empty(t, result) // Should return empty slice when model not supported
}

// TestDefaultChannelSelector_Select_EmptyRequest tests handling of empty request.
func TestDefaultChannelSelector_Select_EmptyRequest(t *testing.T) {
	ctx, client := setupTest(t)

	createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService)

	// Empty request should still work
	req := &llm.Request{}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Empty(t, result) // Empty model should return empty slice
}

// TestSpecifiedChannelSelector_Select_ValidChannel tests SpecifiedChannelSelector with valid channel.
func TestSpecifiedChannelSelector_Select_ValidChannel(t *testing.T) {
	ctx, client := setupTest(t)

	// Create channel
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusDisabled). // Can be disabled for SpecifiedChannelSelector
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: ch.ID})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, ch.ID, result[0].Channel.ID)
}

func TestSpecifiedChannelSelector_Select_ModelMapping(t *testing.T) {
	ctx, client := setupTest(t)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Mapped Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4-turbo"}).
		SetSettings(&objects.ChannelSettings{
			ModelMappings: []objects.ModelMapping{{From: "gpt-4-fast", To: "gpt-4-turbo"}},
		}).
		SetDefaultTestModel("gpt-4-fast").
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	selector := NewSpecifiedChannelSelector(newTestChannelServiceForChannels(client), objects.GUID{ID: ch.ID})

	result, err := selector.Select(ctx, &llm.Request{Model: "gpt-4-fast"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, biz.ChannelModelEntry{
		RequestModel: "gpt-4-fast",
		ActualModel:  "gpt-4-turbo",
		Source:       "mapping",
	}, result[0].Models[0])
}

func TestSpecifiedChannelSelector_Select_HiddenDirectModel(t *testing.T) {
	ctx, client := setupTest(t)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Hidden Model Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4-turbo"}).
		SetSettings(&objects.ChannelSettings{HideOriginalModels: true}).
		SetDefaultTestModel("gpt-4-turbo").
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	selector := NewSpecifiedChannelSelector(newTestChannelServiceForChannels(client), objects.GUID{ID: ch.ID})

	result, err := selector.Select(ctx, &llm.Request{Model: "gpt-4-turbo"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "gpt-4-turbo", result[0].Models[0].ActualModel)
}

// TestSpecifiedChannelSelector_Select_ModelNotSupported tests SpecifiedChannelSelector with unsupported model.
func TestSpecifiedChannelSelector_Select_ModelNotSupported(t *testing.T) {
	ctx, client := setupTest(t)

	// Create channel with limited model support
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Limited Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: ch.ID})

	req := &llm.Request{
		Model: "gpt-4", // Not supported
	}

	result, err := selector.Select(ctx, req)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "model gpt-4 not supported")
}

// TestSpecifiedChannelSelector_Select_ChannelNotFound tests SpecifiedChannelSelector with non-existent channel.
func TestSpecifiedChannelSelector_Select_ChannelNotFound(t *testing.T) {
	ctx, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)
	selector := NewSpecifiedChannelSelector(channelService, objects.GUID{ID: 999}) // Non-existent ID

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "failed to get channel for test")
}

// TestSelectedChannelsSelector_Select_WithFilter tests SelectedChannelsSelector filters by allowed channel IDs.
func TestSelectedChannelsSelector_Select_WithFilter(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)

	// Only allow channels 0 and 2
	allowedIDs := []int{channels[0].ID, channels[2].ID}
	selector := WithSelectedChannelsSelector(baseSelector, allowedIDs)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return only 2 allowed channels
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	require.Contains(t, channelIDs, channels[0].ID, "Allowed channel 0 should be included")
	require.Contains(t, channelIDs, channels[2].ID, "Allowed channel 2 should be included")
	require.NotContains(t, channelIDs, channels[1].ID, "Non-allowed channel 1 should not be included")
}

// TestSelectedChannelsSelector_Select_EmptyFilter tests SelectedChannelsSelector with empty filter returns all.
func TestSelectedChannelsSelector_Select_EmptyFilter(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)

	// Empty filter should return all channels
	selector := WithSelectedChannelsSelector(baseSelector, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return all 3 enabled channels
	require.Len(t, result, 3)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	require.Contains(t, channelIDs, channels[0].ID)
	require.Contains(t, channelIDs, channels[1].ID)
	require.Contains(t, channelIDs, channels[2].ID)
}

func TestDefaultSelector_IndependentChannelModels(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprintf("fallback=%t", fallback), func(t *testing.T) {
			ctx, client := setupTest(t)
			owner, err := client.Channel.Create().
				SetType(channel.TypeAnthropic).SetName("claude官池").SetBaseURL("https://api.pool.example").
				SetCredentials(objects.ChannelCredentials{APIKey: "pool-key"}).
				SetSupportedModels([]string{"claude-opus-5-5"}).SetDefaultTestModel("claude-opus-5-5").
				SetSettings(&objects.ChannelSettings{ExtraModelPrefix: "claude官池", IndependentListing: true}).
				SetStatus(channel.StatusEnabled).Save(ctx)
			require.NoError(t, err)
			public, err := client.Channel.Create().
				SetType(channel.TypeAnthropic).SetName("林夕").SetBaseURL("https://api.public.example").
				SetCredentials(objects.ChannelCredentials{APIKey: "public-key"}).
				SetSupportedModels([]string{"claude-opus-5-5"}).SetDefaultTestModel("claude-opus-5-5").
				SetStatus(channel.StatusEnabled).Save(ctx)
			require.NoError(t, err)
			_, err = client.Model.Create().
				SetDeveloper("anthropic").SetModelID("claude-opus-5-5").SetName("Aggregate").SetGroup("").SetIcon("").SetModelCard(&objects.ModelCard{}).
				SetSettings(&objects.ModelSettings{Associations: []*objects.ModelAssociation{{
					Type: "channel_model", ChannelModel: &objects.ChannelModelAssociation{ChannelID: public.ID, ModelID: "claude-opus-5-5"},
				}}}).SetStatus(model.StatusEnabled).Save(ctx)
			require.NoError(t, err)

			channelSvc := newTestChannelServiceForChannels(client)
			systemSvc := newTestSystemService(client)
			require.NoError(t, systemSvc.SetModelSettings(ctx, biz.SystemModelSettings{
				QueryAllChannelModels: false, FallbackToChannelsOnModelNotFound: fallback,
			}))
			selector := NewDefaultSelector(channelSvc, newTestModelService(client), systemSvc)
			request := &llm.Request{Model: "claude官池/claude-opus-5-5"}
			result, err := selector.Select(ctx, request)
			require.NoError(t, err)
			require.Len(t, result, 1)
			require.Equal(t, owner.ID, result[0].Channel.ID)
			require.Equal(t, "claude-opus-5-5", result[0].Models[0].ActualModel)
			require.Equal(t, request.Model, result[0].Models[0].RequestModel)

			result, err = selector.Select(ctx, &llm.Request{Model: "claude-opus-5-5"})
			require.NoError(t, err)
			require.Len(t, result, 1)
			require.Equal(t, public.ID, result[0].Channel.ID, "independent listing must not modify aggregate rules")

			_, err = selector.Select(ctx, &llm.Request{Model: "claude官池/not-supported"})
			require.ErrorIs(t, err, biz.ErrInvalidModel, "unknown suffixes must not fall back")
			filtered := WithSelectedChannelsSelector(selector, []int{public.ID})
			result, err = filtered.Select(ctx, request)
			require.NoError(t, err)
			require.Empty(t, result, "channel scope restrictions must still apply")

			// Bypass service validation deliberately: runtime selection must fail
			// closed rather than prefer a configured Model or another channel.
			_, err = client.Model.Create().SetDeveloper("test").SetModelID(request.Model).
				SetName("Conflict").SetGroup("").SetIcon("").SetModelCard(&objects.ModelCard{}).SetSettings(&objects.ModelSettings{}).SetStatus(model.StatusDisabled).Save(ctx)
			require.NoError(t, err)
			_, err = selector.Select(ctx, request)
			require.ErrorIs(t, err, biz.ErrIndependentModelConflict)
			require.ErrorIs(t, err, biz.ErrInvalidModel)
		})
	}
}

func TestDefaultSelector_IndependentChannelModelsRespectHiddenMappings(t *testing.T) {
	ctx, client := setupTest(t)
	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).SetName("Pool").SetBaseURL("https://example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"m"}).SetDefaultTestModel("m").
		SetSettings(&objects.ChannelSettings{
			ExtraModelPrefix: "pool", IndependentListing: true, HideMappedModels: true,
			ModelMappings: []objects.ModelMapping{{From: "alias", To: "m"}},
		}).SetStatus(channel.StatusEnabled).Save(ctx)
	require.NoError(t, err)
	channelSvc := newTestChannelServiceForChannels(client)
	systemSvc := newTestSystemService(client)
	require.NoError(t, systemSvc.SetModelSettings(ctx, biz.SystemModelSettings{FallbackToChannelsOnModelNotFound: true}))
	selector := NewDefaultSelector(channelSvc, newTestModelService(client), systemSvc)
	_, err = selector.Select(ctx, &llm.Request{Model: "pool/m"})
	require.ErrorIs(t, err, biz.ErrInvalidModel)
}

func TestDefaultSelector_DisabledIndependentOwnerCannotFallBack(t *testing.T) {
	ctx, client := setupTest(t)
	_, err := client.Channel.Create().SetName("Disabled Owner").SetType(channel.TypeOpenai).
		SetBaseURL("https://example.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"m"}).SetDefaultTestModel("m").
		SetSettings(&objects.ChannelSettings{ExtraModelPrefix: "pool", IndependentListing: true}).
		SetStatus(channel.StatusDisabled).Save(ctx)
	require.NoError(t, err)
	// Simulate an upstream model-sync bypassing edit-time validation.
	_, err = client.Channel.Create().SetName("Legacy Sync").SetType(channel.TypeOpenai).
		SetBaseURL("https://example.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"pool/m"}).SetDefaultTestModel("pool/m").
		SetStatus(channel.StatusEnabled).Save(ctx)
	require.NoError(t, err)
	systemSvc := newTestSystemService(client)
	require.NoError(t, systemSvc.SetModelSettings(ctx, biz.SystemModelSettings{FallbackToChannelsOnModelNotFound: true}))
	selector := NewDefaultSelector(newTestChannelServiceForChannels(client), newTestModelService(client), systemSvc)
	_, err = selector.Select(ctx, &llm.Request{Model: "pool/m"})
	require.ErrorIs(t, err, biz.ErrInvalidModel)
}
