package orchestrator

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestMaybeEvaluateChannelAPIKeyRulesOnFailure_HammerableDoesNotDisable pins
// the fork-specific cross-entry contract: the keyed auto-disable bypass used by
// the orchestrator's final-failure path must classify hammerable failures the
// same way RecordPerformance does, so a hammer channel's upstream 429 never
// feeds auto-disable counters and never disables its credential.
func TestMaybeEvaluateChannelAPIKeyRulesOnFailure_HammerableDoesNotDisable(t *testing.T) {
	ctx := authz.WithTestBypass(context.Background())

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })
	ctx = ent.NewContext(ctx, client)

	duration := 30
	hammer := &objects.ChannelHammerRetry{
		MaxRetries:    hammerIntPtr(3),
		ErrorPatterns: []objects.RetryableErrorPattern{{Pattern: "负载已经达到上限"}},
	}
	ch, err := client.Channel.Create().
		SetName("hammer-auto-disable").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetPolicies(objects.ChannelPolicies{
			APIKeyAutoDisableMode: objects.APIKeyAutoDisableModeCustom,
			APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{
				StatusCodes:            []int{429, 500},
				Times:                  1,
				Action:                 objects.APIKeyAutoDisableActionTemporary,
				DisableDurationMinutes: &duration,
			}},
		}).
		Save(ctx)
	require.NoError(t, err)

	channelService, _, _, _ := setupTestServices(t, client)
	// The hammer config lives on the in-memory channel (Settings.HammerRetry),
	// which hammerConfigForChannel reads; the persisted entity carries the
	// auto-disable rules that evaluateAutoDisableForFailure reads.
	ban := &biz.Channel{Channel: ch}
	ban.Settings = &objects.ChannelSettings{HammerRetry: hammer}
	channelService.SetEnabledChannelsForTest([]*biz.Channel{ban})

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			ChannelService: channelService,
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: ban,
			},
			Perf: &biz.PerformanceRecord{ChannelID: ch.ID, APIKey: "test-key"},
		},
	}

	keyedCtx := contexts.WithChannelAPIKey(ctx, "test-key")

	t.Run("upstream 429 on a hammer channel is skipped", func(t *testing.T) {
		maybeEvaluateChannelAPIKeyRulesOnFailure(
			keyedCtx,
			outbound,
			&httpclient.Error{StatusCode: http.StatusTooManyRequests, Body: []byte(`{"error":{"message":"rate limited"}}`)},
		)

		updated, err := client.Channel.Get(ctx, ch.ID)
		require.NoError(t, err)
		require.Empty(t, updated.DisabledAPIKeys, "hammerable failure must not disable the key")
	})

	t.Run("pattern-matched 500 on a hammer channel is skipped", func(t *testing.T) {
		maybeEvaluateChannelAPIKeyRulesOnFailure(
			keyedCtx,
			outbound,
			newUpstream500("当前模型 gpt-6-astra 负载已经达到上限，请稍后重试"),
		)

		updated, err := client.Channel.Get(ctx, ch.ID)
		require.NoError(t, err)
		require.Empty(t, updated.DisabledAPIKeys)
	})

	t.Run("non-hammerable 500 still triggers the rule", func(t *testing.T) {
		maybeEvaluateChannelAPIKeyRulesOnFailure(
			keyedCtx,
			outbound,
			newUpstream500("internal server error"),
		)

		updated, err := client.Channel.Get(ctx, ch.ID)
		require.NoError(t, err)
		require.Len(t, updated.DisabledAPIKeys, 1)
		require.Equal(t, "test-key", updated.DisabledAPIKeys[0].Key)
		assert.NotNil(t, updated.DisabledAPIKeys[0].ExpiresAt)
	})
}