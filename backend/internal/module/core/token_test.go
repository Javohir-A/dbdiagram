package core_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/core"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/testkit"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/pgtest"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/storagetest"
)

func TestTokensInMemory(t *testing.T) { tokens(t, testkit.New(t, storagetest.New())) }
func TestTokensPostgres(t *testing.T) {
	st, _ := pgtest.Open(t, "tokens")
	tokens(t, testkit.New(t, st))
}

func tokens(t *testing.T, w *testkit.World) {
	ctx := context.Background()
	svc := w.Core.Token
	ann, bob := w.Person("ann"), w.Person("bob")

	t.Run("the secret is shown once and only its hash is kept", func(t *testing.T) {
		made, err := svc.Create(ctx, ann, core.CreateTokenParams{Name: "  laptop ", Scope: model.ScopeWrite})
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(made.Token, core.TokenPrefix))
		require.Greater(t, len(made.Token), 40)
		require.Equal(t, "laptop", made.Name)
		require.Equal(t, made.Token[:len(core.TokenPrefix)+4], made.Prefix)
		require.NotContains(t, made.Prefix, made.Token[len(core.TokenPrefix)+4:], "the prefix is only a recognition aid")
		require.Equal(t, w.Now.Add(90*24*time.Hour).UTC().Format(time.RFC3339), made.ExpiresAt, "default lifetime is 90 days")

		list, err := svc.List(ctx, ann)
		require.NoError(t, err)
		require.Len(t, list.Items, 1)
		require.Equal(t, made.Guid, list.Items[0].Guid)

		p, err := svc.Authenticate(ctx, made.Token)
		require.NoError(t, err)
		require.Equal(t, ann, p.UserID)
		require.Equal(t, model.ScopeWrite, p.Scope)
	})

	t.Run("validation", func(t *testing.T) {
		for name, p := range map[string]core.CreateTokenParams{
			"no name":     {Name: " ", Scope: model.ScopeRead},
			"long name":   {Name: strings.Repeat("x", config.MaxTokenNameLength+1), Scope: model.ScopeRead},
			"bad scope":   {Name: "x", Scope: "admin"},
			"no scope":    {Name: "x"},
			"negative":    {Name: "x", Scope: model.ScopeRead, ExpiresInDays: -1},
			"too long":    {Name: "x", Scope: model.ScopeRead, ExpiresInDays: config.MaxTokenDays + 1},
		} {
			_, err := svc.Create(ctx, ann, p)
			require.Equal(t, apperr.KindInvalidArgument, kind(t, err), name)
		}
	})

	t.Run("every failure to authenticate looks the same", func(t *testing.T) {
		made, err := svc.Create(ctx, bob, core.CreateTokenParams{Name: "ci", Scope: model.ScopeRead, ExpiresInDays: 1})
		require.NoError(t, err)

		_, wrongPrefix := svc.Authenticate(ctx, "ghp_"+made.Token[len(core.TokenPrefix):])
		_, unknown := svc.Authenticate(ctx, core.TokenPrefix+strings.Repeat("A", 43))
		_, huge := svc.Authenticate(ctx, core.TokenPrefix+strings.Repeat("A", 500))
		_, empty := svc.Authenticate(ctx, "")
		for _, err := range []error{wrongPrefix, unknown, huge, empty} {
			require.Equal(t, apperr.KindUnauthenticated, kind(t, err))
			require.Equal(t, unknown.Error(), err.Error())
		}

		*w.Now = w.Now.Add(25 * time.Hour)
		_, expired := svc.Authenticate(ctx, made.Token)
		require.Equal(t, unknown.Error(), expired.Error(), "an expired token is indistinguishable from an unknown one")
		list, err := svc.List(ctx, bob)
		require.NoError(t, err)
		require.Empty(t, list.Items, "expired tokens drop out of the list")
		*w.Now = w.Now.Add(-25 * time.Hour)
	})

	t.Run("revoking is immediate and only yours", func(t *testing.T) {
		made, err := svc.Create(ctx, ann, core.CreateTokenParams{Name: "tmp", Scope: model.ScopeRead})
		require.NoError(t, err)

		require.Equal(t, apperr.KindNotFound, kind(t, svc.Revoke(ctx, bob, made.Guid)), "someone else's token")
		_, err = svc.Authenticate(ctx, made.Token)
		require.NoError(t, err, "bob's failed revoke changed nothing")

		require.NoError(t, svc.Revoke(ctx, ann, made.Guid))
		_, err = svc.Authenticate(ctx, made.Token)
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err))
		require.Equal(t, apperr.KindNotFound, kind(t, svc.Revoke(ctx, ann, made.Guid)), "twice")
	})

	t.Run("a user may hold at most 20 active tokens", func(t *testing.T) {
		carl := w.Person("carl")
		for i := 0; i < config.MaxActiveTokens; i++ {
			_, err := svc.Create(ctx, carl, core.CreateTokenParams{Name: "t", Scope: model.ScopeRead})
			require.NoError(t, err)
		}
		_, err := svc.Create(ctx, carl, core.CreateTokenParams{Name: "one too many", Scope: model.ScopeRead})
		require.Equal(t, apperr.KindInvalidArgument, kind(t, err))

		_, err = svc.Create(ctx, ann, core.CreateTokenParams{Name: "other user is unaffected", Scope: model.ScopeRead})
		require.NoError(t, err)
	})

	t.Run("last use is recorded, at most once a minute", func(t *testing.T) {
		made, err := svc.Create(ctx, ann, core.CreateTokenParams{Name: "usage", Scope: model.ScopeRead})
		require.NoError(t, err)
		find := func() string {
			list, err := svc.List(ctx, ann)
			require.NoError(t, err)
			for _, i := range list.Items {
				if i.Guid == made.Guid {
					return i.LastUsedAt
				}
			}

			return "missing"
		}

		require.Empty(t, find())
		_, err = svc.Authenticate(ctx, made.Token)
		require.NoError(t, err)
		first := find()
		require.NotEmpty(t, first)

		*w.Now = w.Now.Add(30 * time.Second)
		_, err = svc.Authenticate(ctx, made.Token)
		require.NoError(t, err)
		require.Equal(t, first, find(), "inside the minute nothing is written")

		*w.Now = w.Now.Add(2 * time.Minute)
		_, err = svc.Authenticate(ctx, made.Token)
		require.NoError(t, err)
		require.NotEqual(t, first, find())
	})
}
