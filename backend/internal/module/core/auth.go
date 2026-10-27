package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
	"gitlab.com/udevs/dbdiagram/backend/pkg/jwt"
)

type AuthService struct {
	deps Deps
}

type RegisterParams struct {
	Email     string
	Name      string
	Password  string
	UserAgent string
}

type LoginParams struct {
	Email     string
	Password  string
	UserAgent string
}

// dummyHash is compared against when the email is unknown, so a login for a
// missing account takes as long as one for a real account with a wrong
// password: response time does not reveal which emails are registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.MinCost)

var errBadCredentials = apperr.Unauthenticated("invalid email or password")

func (s *AuthService) Register(ctx context.Context, p RegisterParams) (*model.AuthResult, error) {
	if !s.deps.Cfg.RegistrationEnabled {
		return nil, apperr.PermissionDenied("sign-up is disabled")
	}

	email, err := cleanEmail(p.Email)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(p.Name)
	if name == "" || utf8.RuneCountInString(name) > config.MaxNameLength {
		return nil, apperr.InvalidArgument("name must be 1 to %d characters", config.MaxNameLength)
	}
	if err := checkPassword(p.Password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), s.deps.Cfg.BcryptCost)
	if err != nil {
		return nil, apperr.Internal(err, "hash password")
	}

	var res *model.AuthResult

	err = s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		u, err := st.User().Create(ctx, &models.UserCreate{Email: email, Name: name, PasswordHash: string(hash)})
		if err != nil {
			return err
		}

		res, err = s.issue(ctx, st, u, uuid.NewString(), p.UserAgent)

		return err
	})

	return res, err
}

func (s *AuthService) Login(ctx context.Context, p LoginParams) (*model.AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(p.Email))

	cred, err := s.deps.Storage.User().GetCredentials(ctx, email)
	if err != nil {
		if isNotFound(err) {
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(p.Password))

			return nil, errBadCredentials
		}

		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(p.Password)) != nil {
		return nil, errBadCredentials
	}

	return s.issue(ctx, s.deps.Storage, &cred.User, uuid.NewString(), p.UserAgent)
}

// Refresh trades a refresh token for a new pair. The old token stops working.
// Presenting one that was already used means it leaked (or the client raced
// itself), so the whole family dies and the user logs in again.
func (s *AuthService) Refresh(ctx context.Context, refreshToken, userAgent string) (*model.AuthResult, error) {
	var (
		res          *model.AuthResult
		reusedFamily string
	)

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		sess, err := st.Session().GetByHash(ctx, hashToken(refreshToken))
		if err != nil {
			if isNotFound(err) {
				return apperr.Unauthenticated("invalid refresh token")
			}

			return err
		}
		if sess.Revoked {
			reusedFamily = sess.FamilyGuid

			return errReuse
		}
		if !s.deps.now().Before(sess.ExpiresAt) {
			return apperr.Unauthenticated("refresh token expired")
		}

		u, err := st.User().Get(ctx, sess.UserGuid)
		if err != nil {
			if isNotFound(err) {
				return apperr.Unauthenticated("invalid refresh token")
			}

			return err
		}

		// One statement decides the race: of two requests holding the same
		// token, only one revokes it; the other sees false and is treated as reuse.
		revoked, err := st.Session().Revoke(ctx, sess.Guid)
		if err != nil {
			return err
		}
		if !revoked {
			reusedFamily = sess.FamilyGuid

			return errReuse
		}

		res, err = s.issue(ctx, st, u, sess.FamilyGuid, userAgent)

		return err
	})
	if errors.Is(err, errReuse) {
		// Outside the transaction on purpose: it was rolled back, and this
		// revocation must stick.
		if rerr := s.deps.Storage.Session().RevokeFamily(ctx, reusedFamily); rerr != nil {
			return nil, rerr
		}

		return nil, apperr.Unauthenticated("refresh token was already used; sign in again")
	}

	return res, err
}

// Logout ends the login the token belongs to. An unknown token is not an error:
// the caller wanted to be logged out and they are.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	sess, err := s.deps.Storage.Session().GetByHash(ctx, hashToken(refreshToken))
	if err != nil {
		if isNotFound(err) {
			return nil
		}

		return err
	}

	return s.deps.Storage.Session().RevokeFamily(ctx, sess.FamilyGuid)
}

// issue creates the access token and a fresh refresh token in the family.
func (s *AuthService) issue(ctx context.Context, st storage.StorageI, u *model.User, family, userAgent string) (*model.AuthResult, error) {
	var (
		now = s.deps.now()
		ttl = time.Duration(s.deps.Cfg.JWTAccessTTLMinutes) * time.Minute
	)

	access, err := jwt.Sign(s.deps.Cfg.JWTSecret, u.Guid, family, ttl, now)
	if err != nil {
		return nil, apperr.Internal(err, "sign access token")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, apperr.Internal(err, "generate refresh token")
	}
	refresh := base64.RawURLEncoding.EncodeToString(raw)

	if len(userAgent) > 200 {
		userAgent = userAgent[:200]
	}
	err = st.Session().Create(ctx, &models.SessionCreate{
		UserGuid:   u.Guid,
		FamilyGuid: family,
		TokenHash:  hashToken(refresh),
		UserAgent:  userAgent,
		ExpiresAt:  now.Add(time.Duration(s.deps.Cfg.JWTRefreshTTLHours) * time.Hour),
	})
	if err != nil {
		return nil, err
	}

	return &model.AuthResult{User: u, AccessToken: access, RefreshToken: refresh, ExpiresIn: int(ttl.Seconds())}, nil
}

// hashToken is what the database holds: a stolen table cannot be replayed as
// refresh tokens. The token is 256 random bits, so a plain hash is enough.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

// cleanEmail trims, lower-cases and validates an address.
func cleanEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) > config.MaxEmailLength {
		return "", apperr.InvalidArgument("email is too long")
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", apperr.InvalidArgument("email is not valid")
	}

	return email, nil
}

func checkPassword(password string) error {
	switch {
	case utf8.RuneCountInString(password) < config.MinPasswordLength:
		return apperr.InvalidArgument("password must be at least %d characters", config.MinPasswordLength)
	case len(password) > config.MaxPasswordBytes:
		return apperr.InvalidArgument("password must be at most %d bytes", config.MaxPasswordBytes)
	}

	return nil
}

var errReuse = errors.New("refresh token reuse")

func isNotFound(err error) bool {
	var e *apperr.Error

	return errors.As(err, &e) && e.Kind == apperr.KindNotFound
}
