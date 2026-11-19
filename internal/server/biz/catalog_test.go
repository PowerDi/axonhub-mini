package biz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

func newCatalogTestService(t *testing.T) (context.Context, *SystemService, *CatalogService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	ctx := authz.WithTestBypass(context.Background())
	ctx = ent.NewContext(ctx, client)

	system := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})

	svc := NewCatalogService(system, nil)
	t.Cleanup(svc.Invalidate)
	return ctx, system, svc
}

func useCatalogUpstream(t *testing.T, ctx context.Context, system *SystemService, svc *CatalogService, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	require.NoError(t, system.SetCatalogSettings(ctx, CatalogSettings{UpstreamURL: server.URL}))
	svc.httpClient = httpclient.NewHttpClientWithClient(server.Client())
}

func TestCatalogService_FallsBackThenFetchesAfterInvalidate(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)

	payload, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceFallback, payload.Source)
	require.True(t, payload.Filtered)
	require.Contains(t, string(payload.Data), `"providers"`)

	upstream := catalogFile{Providers: map[string]catalogProvider{
		"openai": {ID: "openai", Models: []map[string]any{{"id": "gpt-hot"}}},
	}}
	body, err := json.Marshal(upstream)
	require.NoError(t, err)

	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})

	held, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceFallback, held.Source, "the held fallback is served until it is released")

	svc.Invalidate()
	fetched, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceUpstream, fetched.Source)
	require.True(t, fetched.Filtered)
	require.Contains(t, string(fetched.Data), "gpt-hot")
	require.WithinDuration(t, time.Now().UTC(), *fetched.FetchedAt, 5*time.Second)
}

func TestCatalogService_CachesFallbackAndDeduplicatesFetch(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)

	var hits atomic.Int32
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(150 * time.Millisecond)
		http.Error(w, "unavailable", http.StatusBadGateway)
	})

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		sources []string
	)
	for range 8 {
		wg.Go(func() {
			snapshot, err := svc.Snapshot(ctx, true)
			require.NoError(t, err)
			mu.Lock()
			sources = append(sources, snapshot.Source)
			mu.Unlock()
		})
	}
	wg.Wait()

	require.Len(t, sources, 8)
	for _, source := range sources {
		require.Equal(t, catalogSourceFallback, source)
	}
	require.Equal(t, int32(1), hits.Load())

	second, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceFallback, second.Source)
	require.Equal(t, int32(1), hits.Load())
}

func TestCatalogService_NothingIsResidentAfterTTL(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)
	svc.holdTTL = 200 * time.Millisecond

	body := []byte(`{"providers":{` +
		`"openai":{"id":"openai","tags":["x"],"models":[{"id":"gpt-hot","cost":{"input":1}}]},` +
		`"openrouter":{"id":"openrouter","models":[{"id":"openai/gpt-hot","cost":{"input":2}}]}` +
		`},"updated_at":"2026-11-08"}`)

	var hits atomic.Int32
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})

	heldFiltered, _ := svc.held(true)
	heldUnfiltered, _ := svc.held(false)
	require.Nil(t, heldFiltered)
	require.Nil(t, heldUnfiltered)

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			snapshot, err := svc.Snapshot(ctx, false)
			require.NoError(t, err)
			require.False(t, snapshot.Filtered)
			require.Equal(t, catalogSourceUpstream, snapshot.Source)
			require.JSONEq(t, string(body), string(snapshot.Data))
		})
	}
	wg.Wait()
	require.Equal(t, int32(1), hits.Load(), "concurrent unfiltered requests share one fetch")

	_, err := svc.Snapshot(ctx, false)
	require.NoError(t, err)
	require.Equal(t, int32(1), hits.Load(), "unfiltered catalog is reused within its TTL")

	filtered, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, int32(2), hits.Load())
	require.Contains(t, string(filtered.Data), "gpt-hot")
	require.NotContains(t, string(filtered.Data), "openrouter")

	_, err = svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, int32(2), hits.Load(), "filtered catalog is reused within its TTL")

	require.Eventually(t, func() bool {
		filtered, _ := svc.held(true)
		unfiltered, _ := svc.held(false)
		return filtered == nil && unfiltered == nil
	}, 2*time.Second, 20*time.Millisecond, "both catalogs are released after their TTL")

	_, err = svc.Snapshot(ctx, true)
	require.NoError(t, err)
	_, err = svc.Snapshot(ctx, false)
	require.NoError(t, err)
	require.Equal(t, int32(4), hits.Load(), "released catalogs are fetched again on demand")
}

func TestCatalogService_InvalidUpstreamServesHeldFallback(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)

	responses := []string{`{"providers":[]}`, `{"providers":{`}
	var hits atomic.Int32
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		i := int(hits.Add(1)) - 1
		if i >= len(responses) {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(responses[i]))
	})

	for range 2 {
		svc.Invalidate()
		snapshot, err := svc.Snapshot(ctx, false)
		require.NoError(t, err)
		require.Equal(t, catalogSourceFallback, snapshot.Source)
		require.Equal(t, string(catalogFallbackJSON), string(snapshot.Data))
	}

	_, err := svc.Snapshot(ctx, false)
	require.NoError(t, err)
	require.Equal(t, int32(2), hits.Load(), "fallback is held for the TTL instead of retrying upstream")
}

func TestCatalogService_CancelledCallerDoesNotCauseFallback(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)

	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"providers":{"openai":{"id":"openai","models":[{"id":"gpt-hot"}]}}}`))
	})

	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	snapshot, err := svc.Snapshot(cancelled, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceUpstream, snapshot.Source)
	require.Contains(t, string(snapshot.Data), "gpt-hot")
}

func TestCatalogSettings_DefaultProxyModeForLegacyConfig(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"providers":{"openai":{"id":"openai","models":[{"id":"gpt-legacy"}]}}}`))
	}))
	t.Cleanup(upstream.Close)
	legacySettings, err := json.Marshal(map[string]string{"upstream_url": upstream.URL})
	require.NoError(t, err)
	require.NoError(t, system.setSystemValue(ctx, SystemKeyCatalogSettings, string(legacySettings)))
	svc.httpClient = httpclient.NewHttpClientWithClient(upstream.Client())

	settings, err := system.CatalogSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, CatalogSettings{UpstreamURL: upstream.URL, ProxyMode: catalogProxyEnvironment}, *settings)
	snapshot, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceUpstream, snapshot.Source)
	require.Contains(t, string(snapshot.Data), "gpt-legacy")
}

func TestCatalogService_SwitchesBetweenPresetAndDirect(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)
	var upstreamHits, proxyHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHits.Add(1)
		_, _ = w.Write([]byte(`{"providers":{"openai":{"id":"openai","models":[{"id":"gpt-direct"}]}}}`))
	}))
	t.Cleanup(upstream.Close)

	const proxyUsername, proxyPassword = "catalog", "test-pass"
	proxyAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(proxyUsername+":"+proxyPassword))
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		if r.Header.Get("Proxy-Authorization") != proxyAuth || r.URL.Host != upstream.Listener.Addr().String() {
			http.Error(w, "unexpected proxy request", http.StatusProxyAuthRequired)
			return
		}
		_, _ = w.Write([]byte(`{"providers":{"openai":{"id":"openai","models":[{"id":"gpt-from-proxy"}]}}}`))
	}))
	t.Cleanup(proxy.Close)
	require.NoError(t, system.SaveProxyPreset(ctx, ProxyPreset{URL: proxy.URL, Username: proxyUsername, Password: proxyPassword}))

	// The base client already has a proxy. Both explicit modes must override it.
	svc.httpClient = httpclient.NewHttpClientWithProxy(&httpclient.ProxyConfig{Type: httpclient.ProxyTypeURL, URL: proxy.URL})
	t.Cleanup(svc.httpClient.CloseIdleConnections)
	settings := CatalogSettings{UpstreamURL: upstream.URL, ProxyMode: catalogProxyPreset, ProxyPresetURL: proxy.URL}
	require.NoError(t, svc.SetSettings(ctx, settings))
	stored, err := system.getSystemValue(ctx, SystemKeyCatalogSettings)
	require.NoError(t, err)
	require.NotContains(t, stored, proxyPassword)

	for _, filtered := range []bool{true, false} {
		snapshot, err := svc.Snapshot(ctx, filtered)
		require.NoError(t, err)
		require.Equal(t, catalogSourceUpstream, snapshot.Source)
		require.Contains(t, string(snapshot.Data), "gpt-from-proxy")
	}
	require.Equal(t, int32(2), proxyHits.Load())
	require.Zero(t, upstreamHits.Load())

	require.NoError(t, system.DeleteProxyPreset(ctx, proxy.URL))
	require.ErrorContains(t, svc.SetSettings(ctx, settings), "selected catalog proxy preset does not exist")
	svc.Invalidate()
	for _, filtered := range []bool{true, false} {
		snapshot, err := svc.Snapshot(ctx, filtered)
		require.NoError(t, err)
		require.Equal(t, catalogSourceFallback, snapshot.Source)
	}
	require.Equal(t, int32(2), proxyHits.Load())
	require.Zero(t, upstreamHits.Load())

	settings.ProxyMode = catalogProxyDirect
	require.NoError(t, svc.SetSettings(ctx, settings))
	saved, err := system.CatalogSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, catalogProxyDirect, saved.ProxyMode)
	require.Empty(t, saved.ProxyPresetURL)
	for _, filtered := range []bool{true, false} {
		snapshot, err := svc.Snapshot(ctx, filtered)
		require.NoError(t, err)
		require.Equal(t, catalogSourceUpstream, snapshot.Source)
		require.Contains(t, string(snapshot.Data), "gpt-direct")
	}
	require.Equal(t, int32(2), upstreamHits.Load())
	require.Equal(t, int32(2), proxyHits.Load())
}

func TestCatalogService_CommittedSettingsDiscardOlderFetches(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	finishFetch := func() { releaseOnce.Do(func() { close(release) }) }
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte(`{"providers":{"openai":{"id":"openai","models":[{"id":"gpt-old"}]}}}`))
	})
	t.Cleanup(finishFetch)

	newUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"providers":{"openai":{"id":"openai","models":[{"id":"gpt-new"}]}}}`))
	}))
	t.Cleanup(newUpstream.Close)

	oldFetchDone := make(chan error, 1)
	go func() {
		defer func() {
			if panicValue := recover(); panicValue != nil {
				oldFetchDone <- fmt.Errorf("old catalog fetch panicked: %v", panicValue)
			}
		}()
		_, err := svc.Snapshot(ctx, true)
		oldFetchDone <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("old catalog fetch did not reach the upstream")
	}

	_, before := svc.held(true)
	tx, err := ent.FromContext(ctx).Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	txCtx := ent.NewContext(ent.NewTxContext(ctx, tx), tx.Client())
	require.NoError(t, svc.SetSettings(txCtx, CatalogSettings{UpstreamURL: newUpstream.URL, ProxyMode: catalogProxyDirect}))
	_, pending := svc.held(true)
	require.Equal(t, before, pending)
	require.NoError(t, tx.Commit())
	_, committed := svc.held(true)
	require.Greater(t, committed, before)

	// The old fetch stays blocked while a new request loads the committed settings.
	newSnapshot, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceUpstream, newSnapshot.Source)
	require.Contains(t, string(newSnapshot.Data), "gpt-new")
	finishFetch()
	require.NoError(t, <-oldFetchDone)

	current, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, newSnapshot, current)
}
