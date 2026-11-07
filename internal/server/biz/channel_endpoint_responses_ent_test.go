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

func TestResponsesPolicyEchoAcrossIdentityEdits(t *testing.T) {
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	for _, tt := range []struct {
		name                     string
		typ                      channel.Type
		base, nextBase           string
		endpoints, nextEndpoints []objects.ChannelEndpoint
		allow, exclude           []string
		enabled, wantError       bool
	}{
		{name: "A split legacy WS endpoint with echoed exclusion", typ: channel.TypeOpenai, base: "https://example.test/v1", endpoints: []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket"}}, nextEndpoints: []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, exclude: []string{h}},
		{name: "B WS to HTTP with echoed allow", typ: channel.TypeOpenaiResponses, base: "wss://example.test/v1#", nextBase: "https://example.test/v1#", allow: []string{h}, enabled: true, wantError: true},
		{name: "C HTTP to WS with echoed allow", typ: channel.TypeOpenaiResponses, base: "https://example.test/v1#", nextBase: "wss://example.test/v1#", allow: []string{h}, enabled: true, wantError: true},
		{name: "disabled HTTP restriction survives WS base edit and reload", typ: channel.TypeOpenaiResponses, base: "https://example.test/v1#", nextBase: "wss://example.test/v1#", allow: []string{h}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, svc := setupRoutabilityDB(t)
			status := channel.StatusDisabled
			if tt.enabled {
				status = channel.StatusEnabled
			}
			stored, err := svc.entFromContext(ctx).Channel.Create().SetType(tt.typ).SetName("legacy").SetBaseURL(tt.base).
				SetCredentials(objects.ChannelCredentials{APIKey: "test"}).SetSupportedModels([]string{"gpt-4o"}).SetDefaultTestModel("gpt-4o").SetStatus(status).
				SetEndpoints(tt.endpoints).SetSettings(&objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "gpt-4o", Allow: tt.allow, Exclude: tt.exclude}}}).Save(ctx)
			require.NoError(t, err)
			input := &ent.UpdateChannelInput{Settings: stored.Settings, Endpoints: tt.nextEndpoints}
			if tt.nextBase != "" {
				input.BaseURL = &tt.nextBase
			}
			_, err = svc.UpdateChannel(ctx, stored.ID, input)
			if tt.wantError {
				require.ErrorContains(t, err, "unroutable")
			} else {
				require.NoError(t, err)
			}
			saved, err := svc.entFromContext(ctx).Channel.Get(ctx, stored.ID)
			require.NoError(t, err)
			if tt.wantError {
				require.Equal(t, stored.Settings, saved.Settings)
				require.Equal(t, stored.BaseURL, saved.BaseURL)
			} else {
				policy := (&Channel{Channel: saved}).ModelAPIFormatPolicy("gpt-4o")
				require.True(t, policy.AllowsAPIFormat(h), "HTTP must not silently become forbidden")
				require.False(t, policy.AllowsAPIFormat(w), "WS must not silently become permitted")
				if len(tt.exclude) > 0 {
					require.Equal(t, []string{w}, saved.Settings.ModelAPIFormatPolicies[0].Exclude)
				}
			}
		})
	}
}

func TestResponsesPolicyUpdateOnlyTranslatesUnchangedEntriesUsingOldIdentity(t *testing.T) {
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	existing := &ent.Channel{BaseURL: "wss://example.test", Settings: &objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
		{Model: "unchanged", Allow: []string{h, "openai/chat_completions"}},
		{Model: "edited", Exclude: []string{h}},
	}}}
	input := &objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{
		{Model: "unchanged", Allow: []string{"openai/chat_completions", h}},
		{Model: "edited", Allow: []string{h}},
		{Model: "new", Allow: []string{h}},
	}}
	normalized := normalizeUpdatedModelAPIFormatPolicies(input, existing, &ent.Channel{BaseURL: "https://example.test"})
	require.Equal(t, []string{"openai/chat_completions", w}, normalized.ModelAPIFormatPolicies[0].Allow)
	require.Equal(t, []string{h}, normalized.ModelAPIFormatPolicies[1].Allow)
	require.Equal(t, []string{h}, normalized.ModelAPIFormatPolicies[2].Allow)
	require.Equal(t, []string{"openai/chat_completions", h}, input.ModelAPIFormatPolicies[0].Allow, "do not mutate the submitted settings")
}

func TestResponsesSettingsOnlyEditPreservesEndpointCapabilities(t *testing.T) {
	ctx, svc := setupRoutabilityDB(t)
	h := llm.APIFormatOpenAIResponse.String()
	stored, err := svc.entFromContext(ctx).Channel.Create().SetType(channel.TypeOpenaiResponses).SetName("settings-only").
		SetBaseURL("https://example.test/v1").SetCredentials(objects.ChannelCredentials{APIKey: "test"}).
		SetSupportedModels([]string{"gpt-4o"}).SetDefaultTestModel("gpt-4o").
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket"}}).
		SetSettings(&objects.ChannelSettings{ModelAPIFormatPolicies: []objects.ModelAPIFormatPolicy{{Model: "gpt-4o", Allow: []string{h}}}}).Save(ctx)
	require.NoError(t, err)
	endpoints := (&Channel{Channel: stored}).ResolveEndpoints()
	settings := *stored.Settings
	settings.ReasoningFilter = true
	updated, err := svc.UpdateChannel(ctx, stored.ID, &ent.UpdateChannelInput{Settings: &settings})
	require.NoError(t, err)
	require.Equal(t, stored.Endpoints, updated.Endpoints)
	require.Equal(t, endpoints, (&Channel{Channel: updated}).ResolveEndpoints())
	require.Equal(t, []string{llm.APIFormatOpenAIResponseWebSocket.String()}, updated.Settings.ModelAPIFormatPolicies[0].Allow)
}
