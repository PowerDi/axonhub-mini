package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestCreateProjectRefreshesCreatorMembershipAfterCommit(t *testing.T) {
	for _, mode := range []string{"own transaction", "outer commit", "outer rollback"} {
		t.Run(mode, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:project_membership_cache?mode=memory&_fk=1")
			defer client.Close()
			ctx := authz.WithTestBypass(context.Background())
			ctx = ent.NewContext(ctx, client)
			owner, err := client.User.Create().SetEmail("owner@example.com").SetPassword("hashed").SetIsOwner(true).Save(ctx)
			require.NoError(t, err)
			ctx = contexts.WithUser(ctx, owner)

			users := NewUserService(UserServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
			projects := NewProjectService(ProjectServiceParams{Ent: client})
			projects.invalidateUserCache = users.invalidateUserCache
			cached, err := users.GetUserByID(ctx, owner.ID)
			require.NoError(t, err)
			require.Empty(t, ConvertUserToUserInfo(ctx, cached).Projects)

			createCtx := ctx
			var tx *ent.Tx
			if mode != "own transaction" {
				tx, err = client.Tx(ctx)
				require.NoError(t, err)
				createCtx = ent.NewTxContext(ent.NewContext(ctx, tx.Client()), tx)
			}
			proj, err := projects.CreateProject(createCtx, ent.CreateProjectInput{Name: "new project"})
			require.NoError(t, err)

			if tx != nil {
				// A concurrent reader must keep its committed snapshot until commit.
				cached, err = users.GetUserByID(ctx, owner.ID)
				require.NoError(t, err)
				require.Empty(t, ConvertUserToUserInfo(ctx, cached).Projects)
				if mode == "outer rollback" {
					require.NoError(t, tx.Rollback())
				} else {
					require.NoError(t, tx.Commit())
				}
			}

			cached, err = users.GetUserByID(ctx, owner.ID)
			require.NoError(t, err)
			info := ConvertUserToUserInfo(ctx, cached)
			if mode == "outer rollback" {
				require.Empty(t, info.Projects)
				require.Zero(t, client.Project.Query().CountX(ctx))
				require.Zero(t, client.Role.Query().CountX(ctx))
			} else {
				require.Len(t, info.Projects, 1)
				require.Equal(t, proj.ID, info.Projects[0].ProjectID.ID)
				require.True(t, info.Projects[0].IsOwner)
				require.Equal(t, 3, client.Role.Query().CountX(ctx))
			}
		})
	}
}
