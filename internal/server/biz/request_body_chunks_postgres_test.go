// 职责: 在独立 PostgreSQL 测试库验证旧正文迁移、去重、ID 返回和并发清理。
// 关联文件:
//
//	internal/server/biz/request_body_chunks.go
//	internal/server/biz/request_body_chunks_test.go
//	internal/ent/schema/request_body_chunk_ref.go
//
// 注意事项: 仅接受显式配置的本机 axonhub_payload_test 数据库，不连接生产。

package biz

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestbodychunk"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/log"
)

func TestChunkedRequestBodyPostgresConcurrentGC(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_PAYLOAD_PG_DSN")
	if dsn == "" {
		t.Skip("AXONHUB_TEST_PAYLOAD_PG_DSN is not configured")
	}
	endpoint, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, endpoint.Hostname())
	require.Equal(t, "/axonhub_payload_test", endpoint.Path, "this destructive fixture is restricted to its dedicated test database")
	bootstrap, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bootstrap.Close() })
	schemaName := fmt.Sprintf("payload_test_%d", time.Now().UnixNano())
	_, err = bootstrap.ExecContext(t.Context(), `CREATE SCHEMA "`+schemaName+`"`)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := bootstrap.ExecContext(cleanupCtx, `DROP SCHEMA "`+schemaName+`" CASCADE`)
		require.NoError(t, err)
	})
	query := endpoint.Query()
	query.Set("search_path", schemaName)
	endpoint.RawQuery = query.Encode()
	db, err := sql.Open("pgx", endpoint.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(16)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(authz.WithTestBypass(t.Context()), 45*time.Second)
	defer cancel()
	var database string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database))
	require.Equal(t, "axonhub_payload_test", database)
	require.NoError(t, client.Schema.Create(ctx, migrate.WithForeignKeys(false)))
	var foreignKeys int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM pg_constraint WHERE contype='f' AND connamespace=current_schema()::regnamespace").Scan(&foreignKeys))
	require.Zero(t, foreignKeys, "match the production migration policy")
	project, err := client.Project.Create().SetName(fmt.Sprintf("payload-pg-%d", time.Now().UnixNano())).Save(ctx)
	require.NoError(t, err)
	projectID := project.ID
	body := chunkTestBody()

	create := func(ctx context.Context, client *ent.Client, body []byte) (*ent.Request, error) {
		plan := PrepareChunkedRequestBody(body)
		owner, err := client.Request.Create().SetProjectID(projectID).SetModelID("model").SetRequestBody(plan.Manifest).
			SetRequestBodyChunked(true).SetStatus(request.StatusProcessing).Save(ctx)
		if err != nil {
			return nil, err
		}
		if err := plan.Save(ctx, client, projectID, owner.ID, 0); err != nil {
			return nil, err
		}
		return owner, nil
	}
	load := func(owner *ent.Request, want []byte) error {
		got, err := LoadChunkedRequestBody(ctx, client, projectID, owner.ID, 0, owner.RequestBody)
		if err == nil && !bytes.Equal(got, want) {
			return fmt.Errorf("request %d failed exact byte reconstruction", owner.ID)
		}
		return err
	}
	service := AbstractService{db: client}
	t.Run("upsert returns IDs without readback", func(t *testing.T) {
		driver := &chunkSaveQueryDriver{Driver: client.Driver()}
		writer := ent.NewClient(ent.Driver(driver))
		service := AbstractService{db: writer}
		var owner *ent.Request
		var execution *ent.RequestExecution
		require.NoError(t, service.RunInTransaction(ctx, func(txCtx context.Context) error {
			tx := ent.FromContext(txCtx)
			var err error
			owner, err = create(txCtx, tx, body)
			if err != nil {
				return err
			}
			plan := PrepareChunkedRequestBody(body)
			execution, err = tx.RequestExecution.Create().SetRequestID(owner.ID).SetProjectID(projectID).SetModelID("model").
				SetRequestBody(plan.Manifest).SetRequestBodyChunked(true).SetStatus(requestexecution.StatusProcessing).Save(txCtx)
			if err != nil {
				return err
			}
			return plan.Save(txCtx, tx, projectID, 0, execution.ID)
		}))
		upserts := 0
		for _, query := range driver.queries {
			if strings.HasPrefix(query, "SELECT ") {
				require.NotContains(t, query, request.Table)
				require.NotContains(t, query, requestexecution.Table)
				require.NotContains(t, query, requestbodychunk.Table, "RETURNING must supply the chunk IDs without another query")
			}
			if strings.HasPrefix(query, `INSERT INTO "`+requestbodychunk.Table+`"`) {
				upserts++
				require.Contains(t, query, "ON CONFLICT")
				require.Contains(t, query, `RETURNING "id"`)
			}
		}
		require.GreaterOrEqual(t, upserts, 2, "capture both a new chunk insert and existing-content conflict")
		require.NoError(t, load(owner, body))
		loaded, err := LoadChunkedRequestBody(ctx, client, projectID, 0, execution.ID, execution.RequestBody)
		require.NoError(t, err)
		require.Equal(t, body, []byte(loaded))
		require.NoError(t, client.RequestExecution.DeleteOne(execution).Exec(ctx))
		require.NoError(t, client.Request.DeleteOne(owner).Exec(ctx))
	})

	t.Run("upgrade preserves old inline requests and executions", func(t *testing.T) {
		legacyBody := []byte(`{"n":9007199254740993,"input":"legacy inline history"}`)
		legacy := client.Request.Create().SetProjectID(projectID).SetModelID("legacy").
			SetRequestBody(legacyBody).SetStatus(request.StatusCompleted).SaveX(ctx)
		execution := client.RequestExecution.Create().SetRequestID(legacy.ID).SetProjectID(projectID).SetModelID("legacy").
			SetRequestBody(legacyBody).SetStatus(requestexecution.StatusCompleted).SaveX(ctx)
		// Recreate the pre-chunking schema in this isolated test schema. Merely
		// running migration twice on a fresh database cannot test an actual upgrade.
		for _, statement := range []string{
			"DROP TABLE request_body_chunk_refs",
			"DROP TABLE request_body_chunks",
			"ALTER TABLE requests DROP COLUMN request_body_chunked",
			"ALTER TABLE request_executions DROP COLUMN request_body_chunked",
		} {
			_, err := db.ExecContext(ctx, statement)
			require.NoError(t, err)
		}
		for range 2 {
			require.NoError(t, client.Schema.Create(ctx, migrate.WithForeignKeys(false)))
			loaded, err := client.Request.Get(ctx, legacy.ID)
			require.NoError(t, err)
			require.False(t, loaded.RequestBodyChunked)
			require.JSONEq(t, string(legacyBody), string(loaded.RequestBody))
			loadedExecution, err := client.RequestExecution.Get(ctx, execution.ID)
			require.NoError(t, err)
			require.False(t, loadedExecution.RequestBodyChunked)
			require.JSONEq(t, string(legacyBody), string(loadedExecution.RequestBody))
			require.Zero(t, client.RequestBodyChunk.Query().CountX(ctx))
			require.Zero(t, client.RequestBodyChunkRef.Query().CountX(ctx))
		}
		require.NoError(t, client.RequestExecution.DeleteOne(execution).Exec(ctx))
		require.NoError(t, client.Request.DeleteOne(legacy).Exec(ctx))
	})

	// Leave committed, unreferenced chunks, then attach while the sweeper observes them.
	seed := saveChunkTestRequest(t, client, ctx, projectID, body)
	require.NoError(t, client.Request.DeleteOne(seed).Exec(ctx))
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	lockedOwner, err := create(ctx, tx.Client(), body)
	require.NoError(t, err)
	sweepResult := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(ctx, "Request body test goroutine panicked", log.Any("panic", recovered))
				sweepResult <- fmt.Errorf("sweeper panic: %v", recovered)
			}
		}()
		_, err := DeleteUnreferencedRequestBodyChunks(ctx, client, 100)
		sweepResult <- err
	}()
	locked := false
	deadline := time.Now().Add(5 * time.Second)
	for !locked && time.Now().Before(deadline) {
		var waiting int
		err = db.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND query ILIKE 'UPDATE %request_body_chunks%'`).Scan(&waiting)
		require.NoError(t, err)
		locked = waiting > 0
		if !locked {
			time.Sleep(10 * time.Millisecond)
		}
	}
	require.True(t, locked, "the sweeper must reach the locked chunk before the writer commits")
	require.NoError(t, tx.Commit())
	select {
	case err := <-sweepResult:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, load(lockedOwner, body))

	// Concurrent writers use both exact duplicates and a locally edited history.
	edited := append(append(append([]byte(nil), body[:80000]...), []byte("new-tool-output")...), body[80000:]...)
	var writers sync.WaitGroup
	stopSweep := make(chan struct{})
	sweeperDone := make(chan struct{})
	errors := make(chan error, 32)
	reportPanic := func() {
		if recovered := recover(); recovered != nil {
			log.Error(ctx, "Request body test goroutine panicked", log.Any("panic", recovered))
			errors <- fmt.Errorf("payload PostgreSQL test panic: %v", recovered)
		}
	}
	go func() {
		defer close(sweeperDone)
		defer reportPanic()
		for {
			select {
			case <-stopSweep:
				return
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			default:
				if _, err := DeleteUnreferencedRequestBodyChunks(ctx, client, 100); err != nil {
					errors <- err
					return
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()
	for range 4 {
		writers.Go(func() {
			defer reportPanic()
			for iteration := range 3 {
				want := body
				if iteration%2 != 0 {
					want = edited
				}
				var owner *ent.Request
				err := service.RunInTransaction(ctx, func(txCtx context.Context) error {
					var err error
					owner, err = create(txCtx, ent.FromContext(txCtx), want)
					return err
				})
				if err == nil {
					err = load(owner, want)
				}
				if err != nil {
					errors <- err
					return
				}
			}
		})
	}
	writers.Wait()
	close(stopSweep)
	<-sweeperDone
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}

	unique := make(map[string]struct{})
	for _, raw := range [][]byte{body, edited} {
		for _, chunk := range PrepareChunkedRequestBody(raw).chunks {
			unique[requestBodyHash(chunk)] = struct{}{}
		}
	}
	count, err := client.RequestBodyChunk.Query().Where(requestbodychunk.ProjectIDEQ(projectID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, len(unique), count, "concurrent writes must not duplicate a project's content hashes")
	require.NoError(t, load(lockedOwner, body))
	_, err = client.Request.Delete().Where(request.ProjectIDEQ(projectID)).Exec(ctx)
	require.NoError(t, err)
	for {
		deleted, err := DeleteUnreferencedRequestBodyChunks(ctx, client, 100)
		require.NoError(t, err)
		if deleted == 0 {
			break
		}
	}
	count, err = client.RequestBodyChunk.Query().Where(requestbodychunk.ProjectIDEQ(projectID)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count, "last-owner deletion must make every block reclaimable")
}
