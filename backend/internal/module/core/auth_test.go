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
	"gitlab.com/udevs/dbdiagram/backend/pkg/jwt"
)

const password = "correct horse 9"

func kind(t *testing.T, err error) apperr.Kind {
	t.Helper()

	require.Error(t, err)
	_, e := apperr.HTTPStatus(err)

	return e.Kind
}

// The account rules run on the fake and on Postgres (sessions, the unique
// email index and the atomic revoke are SQL behaviour).
func TestAccountsInMemory(t *testing.T) { accounts(t, testkit.New(t, storagetest.New())) }
func TestAccountsPostgres(t *testing.T) {
	st, _ := pgtest.Open(t, "core")
	accounts(t, testkit.New(t, st))
}

func accounts(t *testing.T, w *testkit.World) {
	ctx := context.Background()
	auth := w.Core.Auth

	reg := func(email, name, pw string) (*model.AuthResult, error) {
		return auth.Register(ctx, core.RegisterParams{Email: email, Name: name, Password: pw})
	}

	t.Run("register validates and normalises", func(t *testing.T) {
		for name, c := range map[string]struct{ email, name, pw string }{
			"no @":            {"nobody", "N", password},
			"no dot in host":  {"a@b", "N", password},
			"display name":    {"Ann <ann@example.test>", "N", password},
			"empty name":      {"x@example.test", "  ", password},
			"short password":  {"x@example.test", "N", "short"},
			"too long (73 B)": {"x@example.test", "N", strings.Repeat("a", 73)},
			"empty email":     {"", "N", password},
		} {
			_, err := reg(c.email, c.name, c.pw)
			require.Equal(t, apperr.KindInvalidArgument, kind(t, err), name)
		}

		res, err := reg("  Ann@Example.TEST ", " Ann ", password)
		require.NoError(t, err)
		require.Equal(t, "ann@example.test", res.User.Email)
		require.Equal(t, "Ann", res.User.Name)
		require.NotEmpty(t, res.RefreshToken)
		require.Equal(t, 15*60, res.ExpiresIn)

		claims, err := jwt.Parse(testkit.Secret, res.AccessToken, *w.Now)
		require.NoError(t, err)
		require.Equal(t, res.User.Guid, claims.UserID())

		_, err = reg("ANN@example.test", "Other", password)
		require.Equal(t, apperr.KindAlreadyExists, kind(t, err), "the email is unique ignoring case")
	})

	t.Run("sign-up can be switched off", func(t *testing.T) {
		w.Cfg.RegistrationEnabled = false
		defer func() { w.Cfg.RegistrationEnabled = true }()

		_, err := reg("closed@example.test", "C", password)
		require.Equal(t, apperr.KindPermissionDenied, kind(t, err))
	})

	t.Run("login gives one answer for a wrong email and a wrong password", func(t *testing.T) {
		_, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: "wrong password"})
		wrongPw := err
		_, err = auth.Login(ctx, core.LoginParams{Email: "ghost@example.test", Password: password})
		ghost := err

		require.Equal(t, apperr.KindUnauthenticated, kind(t, wrongPw))
		require.Equal(t, wrongPw.Error(), ghost.Error(), "the message must not reveal which emails exist")

		res, err := auth.Login(ctx, core.LoginParams{Email: " ANN@example.test", Password: password})
		require.NoError(t, err)
		require.Equal(t, "Ann", res.User.Name)
	})

	t.Run("a refresh token works once; the next token replaces it", func(t *testing.T) {
		login, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.NoError(t, err)

		next, err := auth.Refresh(ctx, login.RefreshToken, "")
		require.NoError(t, err)
		require.NotEqual(t, login.RefreshToken, next.RefreshToken)

		again, err := auth.Refresh(ctx, next.RefreshToken, "")
		require.NoError(t, err, "the rotated token is good")
		require.NotEmpty(t, again.RefreshToken)
	})

	t.Run("reusing a used refresh token ends the whole login", func(t *testing.T) {
		login, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.NoError(t, err)
		rotated, err := auth.Refresh(ctx, login.RefreshToken, "")
		require.NoError(t, err)

		_, err = auth.Refresh(ctx, login.RefreshToken, "")
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err), "the stolen (old) token is refused")

		_, err = auth.Refresh(ctx, rotated.RefreshToken, "")
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err), "and the legitimate holder is logged out too: the family is dead")

		other, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.NoError(t, err)
		_, err = auth.Refresh(ctx, other.RefreshToken, "")
		require.NoError(t, err, "a different login of the same person is untouched")
	})

	t.Run("garbage and expired refresh tokens are refused", func(t *testing.T) {
		_, err := auth.Refresh(ctx, "not-a-token", "")
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err))

		login, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.NoError(t, err)
		*w.Now = w.Now.Add(time.Duration(w.Cfg.JWTRefreshTTLHours)*time.Hour + time.Second)
		_, err = auth.Refresh(ctx, login.RefreshToken, "")
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err), "expired")
		*w.Now = w.Now.Add(-time.Duration(w.Cfg.JWTRefreshTTLHours)*time.Hour - time.Second)
	})

	t.Run("logout ends the login and is idempotent", func(t *testing.T) {
		login, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.NoError(t, err)

		require.NoError(t, auth.Logout(ctx, login.RefreshToken))
		_, err = auth.Refresh(ctx, login.RefreshToken, "")
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err))
		require.NoError(t, auth.Logout(ctx, login.RefreshToken), "logging out twice is fine")
		require.NoError(t, auth.Logout(ctx, "never existed"), "and so is an unknown token")
	})

	t.Run("access tokens expire and cannot be forged", func(t *testing.T) {
		login, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.NoError(t, err)

		_, err = jwt.Parse(testkit.Secret, login.AccessToken, w.Now.Add(16*time.Minute))
		require.Error(t, err, "expired after the access TTL")
		_, err = jwt.Parse("another-secret-another-secret-another-1", login.AccessToken, *w.Now)
		require.Error(t, err, "wrong key")
		_, err = jwt.Parse(testkit.Secret, "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0.", *w.Now)
		require.Error(t, err, "alg none is refused")
	})

	t.Run("changing the password needs the current one and ends other logins", func(t *testing.T) {
		u := w.Core.User
		a, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.NoError(t, err)
		b, err := auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.NoError(t, err)
		claimsA, err := jwt.Parse(testkit.Secret, a.AccessToken, *w.Now)
		require.NoError(t, err)

		require.Equal(t, apperr.KindPermissionDenied, kind(t, u.ChangePassword(ctx, a.User.Guid, "wrong", "brand new pass 1", claimsA.Family)))
		require.Equal(t, apperr.KindInvalidArgument, kind(t, u.ChangePassword(ctx, a.User.Guid, password, "short", claimsA.Family)))

		require.NoError(t, u.ChangePassword(ctx, a.User.Guid, password, "brand new pass 1", claimsA.Family))

		_, err = auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: password})
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err), "the old password is dead")
		_, err = auth.Login(ctx, core.LoginParams{Email: "ann@example.test", Password: "brand new pass 1"})
		require.NoError(t, err)

		_, err = auth.Refresh(ctx, b.RefreshToken, "")
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err), "the other login is ended")
		_, err = auth.Refresh(ctx, a.RefreshToken, "")
		require.NoError(t, err, "the login that made the change survives")
	})

	t.Run("profile", func(t *testing.T) {
		u := w.Core.User
		res, err := u.ByEmail(ctx, " ANN@example.test")
		require.NoError(t, err)

		renamed, err := u.UpdateName(ctx, res.Guid, "  Anna ")
		require.NoError(t, err)
		require.Equal(t, "Anna", renamed.Name)
		_, err = u.UpdateName(ctx, res.Guid, " ")
		require.Equal(t, apperr.KindInvalidArgument, kind(t, err))
		_, err = u.UpdateName(ctx, res.Guid, strings.Repeat("x", config.MaxNameLength+1))
		require.Equal(t, apperr.KindInvalidArgument, kind(t, err))

		_, err = u.Get(ctx, "00000000-0000-0000-0000-000000000000")
		require.Equal(t, apperr.KindUnauthenticated, kind(t, err), "a token for a vanished account is simply not valid")
	})
}
