package biz

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestResponsesEndpointLegacyNormalizationEnt(t *testing.T) {
	// Given: a real isolated SQLite Ent driver and legacy persisted configuration.
	client := enttest.NewEntClient(t, "sqlite3", "file:"+filepath.Join(t.TempDir(), "endpoints.db")+"?_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)
	t.Cleanup(svc.Stop)
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	legacy := []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket", Path: "/custom"}}
	created, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-legacy").
		SetBaseURL("https://chatgpt.com/backend-api/codex#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
		SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetEndpoints(legacy).
		SetSettings(&objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "test-model", Allow: []string{h}}}}).Save(ctx)
	require.NoError(t, err)

	t.Run("read preserves legacy storage", func(t *testing.T) {
		// When
		read, err := client.Channel.Get(ctx, created.ID)
		require.NoError(t, err)
		resolved := (&Channel{Channel: read}).ResolveEndpoints()
		// Then
		require.Equal(t, objects.ChannelEndpoint{APIFormat: w, Path: "/custom"}, resolved[0])
		require.Equal(t, legacy, read.Endpoints)
		require.Equal(t, []string{h}, read.Settings.ModelAPIFormatPolicies[0].Allow)
		require.Equal(t, "https://chatgpt.com/backend-api/codex#", read.BaseURL)
	})
	t.Run("collision rejected before persistence", func(t *testing.T) {
		// When
		_, err := svc.SaveChannelEndpoints(ctx, SaveChannelEndpointsInput{ChannelID: objects.GUID{ID: created.ID}, Endpoints: []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket"}, {APIFormat: w}}})
		// Then
		require.ErrorContains(t, err, "duplicate api_format")
		read, err := client.Channel.Get(ctx, created.ID)
		require.NoError(t, err)
		require.Equal(t, legacy, read.Endpoints)
	})
	t.Run("explicit dual save translates old model restriction", func(t *testing.T) {
		// When
		_, err := svc.SaveChannelEndpoints(ctx, SaveChannelEndpointsInput{ChannelID: objects.GUID{ID: created.ID}, Endpoints: []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}})
		require.NoError(t, err)
		read, err := client.Channel.Get(ctx, created.ID)
		// Then
		require.NoError(t, err)
		require.Equal(t, []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, read.Endpoints)
		require.Equal(t, []string{w}, read.Settings.ModelAPIFormatPolicies[0].Allow)
		require.Equal(t, []string{w}, (&Channel{Channel: read}).ForcedAPIFormats("test-model"))
		require.Equal(t, "https://chatgpt.com/backend-api/codex#", read.BaseURL)
	})
	t.Run("disabled channel may remove W without broadening its model policy", func(t *testing.T) {
		// When
		_, err := svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{Endpoints: []objects.ChannelEndpoint{{APIFormat: h}}})
		// Then
		require.NoError(t, err)
		read, readErr := client.Channel.Get(ctx, created.ID)
		require.NoError(t, readErr)
		require.True(t, policyFiltersAll((&Channel{Channel: read}).ResolveEndpoints(), read.Settings.GetModelAPIFormatPolicy("test-model"), llm.RequestTypeChat))
	})
	t.Run("create canonicalizes legacy input", func(t *testing.T) {
		// When
		created, err := svc.createChannel(ctx, ent.CreateChannelInput{Type: channel.TypeCodex, Name: "synthetic-new", BaseURL: lo.ToPtr("https://chatgpt.com/backend-api/codex#"), Credentials: objects.ChannelCredentials{APIKey: "synthetic-key"}, SupportedModels: []string{"test-model"}, DefaultTestModel: "test-model", Endpoints: legacy, Settings: &objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "test-model", Allow: []string{h}}}}})
		require.NoError(t, err)
		read, err := client.Channel.Get(ctx, created.ID)
		// Then
		require.NoError(t, err)
		require.Equal(t, []objects.ChannelEndpoint{{APIFormat: w, Path: "/custom"}}, read.Endpoints)
		require.Equal(t, []string{w}, read.Settings.ModelAPIFormatPolicies[0].Allow)
	})
	t.Run("update uses old H restriction identity before replacing endpoints", func(t *testing.T) {
		// Given
		original, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-http").
			SetBaseURL("https://example.test#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
			SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetEndpoints([]objects.ChannelEndpoint{{APIFormat: h}}).
			SetSettings(&objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "test-model", Allow: []string{h}}}}).Save(ctx)
		require.NoError(t, err)
		// When
		_, err = svc.UpdateChannel(ctx, original.ID, &ent.UpdateChannelInput{Endpoints: legacy})
		// Then
		require.NoError(t, err)
		read, readErr := client.Channel.Get(ctx, original.ID)
		require.NoError(t, readErr)
		require.Equal(t, []string{h}, read.Settings.ModelAPIFormatPolicies[0].Allow)
		require.True(t, policyFiltersAll((&Channel{Channel: read}).ResolveEndpoints(), read.Settings.GetModelAPIFormatPolicy("test-model"), llm.RequestTypeChat))
	})
	t.Run("combined model and endpoint update persists canonical dual and old W identity", func(t *testing.T) {
		// Given
		original, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-update").
			SetBaseURL("https://chatgpt.com/backend-api/codex#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
			SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetEndpoints(legacy).
			SetSettings(&objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "test-model", Allow: []string{h}}}}).Save(ctx)
		require.NoError(t, err)
		// When
		_, err = svc.UpdateChannel(ctx, original.ID, &ent.UpdateChannelInput{SupportedModels: []string{"test-model", "new-model"}, Endpoints: []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}})
		require.NoError(t, err)
		read, err := client.Channel.Get(ctx, original.ID)
		// Then
		require.NoError(t, err)
		require.Equal(t, []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, read.Endpoints)
		require.Equal(t, []string{w}, read.Settings.ModelAPIFormatPolicies[0].Allow)
		require.Equal(t, "https://chatgpt.com/backend-api/codex#", read.BaseURL)
	})
	t.Run("inherited WSS collision rejected on create", func(t *testing.T) {
		// Given
		input := ent.CreateChannelInput{Type: channel.TypeCodex, Name: "synthetic-collision", BaseURL: lo.ToPtr("wss://example.test#"), Credentials: objects.ChannelCredentials{APIKey: "synthetic-key"}, SupportedModels: []string{"test-model"}, DefaultTestModel: "test-model", Endpoints: []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}}
		// When
		_, err := svc.createChannel(ctx, input)
		// Then
		require.ErrorContains(t, err, "duplicate api_format")
	})
}

func TestResponsesEndpointLegacyNormalizationStoredJSON(t *testing.T) {
	// Given
	dsn := "file:" + filepath.Join(t.TempDir(), "stored.db") + "?_fk=0"
	client := enttest.NewEntClient(t, "sqlite3", dsn)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	created, err := client.Channel.Create().SetType(channel.TypeCodex).SetName("synthetic-json").
		SetBaseURL("https://chatgpt.com/backend-api/codex#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
		SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket", BaseURL: "wss://other.test/custom#"}}).
		SetSettings(&objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "test-model", Allow: []string{h}}}}).Save(ctx)
	require.NoError(t, err)
	var beforeEndpoints, beforeSettings string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT endpoints, settings FROM channels WHERE id = ?", created.ID).Scan(&beforeEndpoints, &beforeSettings))
	// When
	read, err := client.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, w, (&Channel{Channel: read}).ResolveEndpoints()[0].APIFormat)
	require.Equal(t, []string{w}, (&Channel{Channel: read}).ForcedAPIFormats("test-model"))
	var afterEndpoints, afterSettings string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT endpoints, settings FROM channels WHERE id = ?", created.ID).Scan(&afterEndpoints, &afterSettings))
	// Then
	require.Equal(t, beforeEndpoints, afterEndpoints)
	require.Equal(t, beforeSettings, afterSettings)
	require.JSONEq(t, `[{"api_format":"openai/responses","transport":"websocket","base_url":"wss://other.test/custom#"}]`, afterEndpoints)
}

func TestResponsesPolicyRoutabilityAcrossEndpointEdits(t *testing.T) {
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	t.Run("inherited websocket base URL is used on create and save", func(t *testing.T) {
		ctx, svc := setupRoutabilityDB(t)
		created, err := svc.createChannel(ctx, ent.CreateChannelInput{
			Type: channel.TypeOpenaiResponses, Name: "inherited-websocket", BaseURL: lo.ToPtr("wss://example.test/v1"),
			Credentials: objects.ChannelCredentials{APIKey: "synthetic-key"}, SupportedModels: []string{"test-model"}, DefaultTestModel: "test-model",
			Settings: &objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "test-model", Allow: []string{h}}}},
		})
		require.NoError(t, err)
		require.Equal(t, []string{w}, created.Settings.ModelAPIFormatPolicies[0].Allow)
		_, err = svc.entFromContext(ctx).Channel.UpdateOneID(created.ID).SetStatus(channel.StatusEnabled).Save(ctx)
		require.NoError(t, err)
		saved, err := svc.SaveChannelEndpoints(ctx, SaveChannelEndpointsInput{ChannelID: objects.GUID{ID: created.ID}, Endpoints: []objects.ChannelEndpoint{}})
		require.NoError(t, err)
		require.False(t, policyFiltersAll((&Channel{Channel: saved}).ResolveEndpoints(), saved.Settings.GetModelAPIFormatPolicy("test-model"), llm.RequestTypeChat))
		// A transport change must not turn the existing WS-only policy into an HTTP permission.
		_, err = svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{BaseURL: lo.ToPtr("https://example.test/v1")})
		require.ErrorContains(t, err, "unroutable")
	})
	t.Run("enabled HTTP-only policy rejects websocket-only replacement", func(t *testing.T) {
		ctx, svc := setupRoutabilityDB(t)
		created, err := svc.entFromContext(ctx).Channel.Create().SetType(channel.TypeCodex).SetName("enabled-http").
			SetBaseURL("https://example.test#").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
			SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetStatus(channel.StatusEnabled).
			SetEndpoints([]objects.ChannelEndpoint{{APIFormat: h}}).
			SetSettings(&objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "test-model", Allow: []string{h}}}}).Save(ctx)
		require.NoError(t, err)
		endpoints := []objects.ChannelEndpoint{{APIFormat: w}}
		_, err = svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{Endpoints: endpoints})
		require.ErrorContains(t, err, "unroutable")
		_, err = svc.SaveChannelEndpoints(ctx, SaveChannelEndpointsInput{ChannelID: objects.GUID{ID: created.ID}, Endpoints: endpoints})
		require.ErrorContains(t, err, "unroutable")
		read, err := svc.entFromContext(ctx).Channel.Get(ctx, created.ID)
		require.NoError(t, err)
		require.Equal(t, created.Endpoints, read.Endpoints)
		require.Equal(t, created.Settings, read.Settings)
	})
}
