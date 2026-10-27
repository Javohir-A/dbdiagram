package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"
	"unicode/utf8"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
)

// TokenPrefix marks a personal access token, so a leaked one is recognisable
// (and greppable by secret scanners) and the middleware can tell it from a JWT.
const TokenPrefix = "dbd_pat_"

type TokenService struct {
	deps Deps
}

type CreateTokenParams struct {
	Name          string
	Scope         model.TokenScope
	ExpiresInDays int // 0 = config.DefaultTokenDays
}

// Principal is who a PAT authenticates as.
type Principal struct {
	UserID string
	Scope  model.TokenScope
}

// Create issues a token. The secret is returned here and nowhere else: only its
// hash is kept, so it cannot be shown again.
func (s *TokenService) Create(ctx context.Context, actor string, p CreateTokenParams) (*model.CreatedAccessToken, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" || utf8.RuneCountInString(name) > config.MaxTokenNameLength {
		return nil, apperr.InvalidArgument("name must be 1 to %d characters", config.MaxTokenNameLength)
	}
	if !p.Scope.Valid() {
		return nil, apperr.InvalidArgument("scope must be read or write")
	}
	days := p.ExpiresInDays
	if days == 0 {
		days = config.DefaultTokenDays
	}
	if days < 1 || days > config.MaxTokenDays {
		return nil, apperr.InvalidArgument("expires_in_days must be 1 to %d", config.MaxTokenDays)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, apperr.Internal(err, "generate token")
	}
	secret := TokenPrefix + base64.RawURLEncoding.EncodeToString(raw)

	var res *model.CreatedAccessToken

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		now := s.deps.now()

		n, err := st.Token().CountActive(ctx, actor, now)
		if err != nil {
			return err
		}
		if n >= config.MaxActiveTokens {
			return apperr.InvalidArgument("you already have %d active tokens; revoke one first", config.MaxActiveTokens)
		}

		t, err := st.Token().Create(ctx, &models.TokenCreate{
			UserGuid: actor, Name: name, Prefix: secret[:len(TokenPrefix)+4], TokenHash: hashToken(secret),
			Scope: p.Scope, ExpiresAt: now.Add(time.Duration(days) * 24 * time.Hour),
		})
		if err != nil {
			return err
		}

		res = &model.CreatedAccessToken{AccessToken: *t, Token: secret}

		return nil
	})

	return res, err
}

func (s *TokenService) List(ctx context.Context, actor string) (*model.List[*model.AccessToken], error) {
	items, err := s.deps.Storage.Token().ListActive(ctx, actor, s.deps.now())
	if err != nil {
		return nil, err
	}

	return &model.List[*model.AccessToken]{Items: items, Count: int32(len(items))}, nil
}

func (s *TokenService) Revoke(ctx context.Context, actor, guid string) error {
	return s.deps.Storage.Token().Revoke(ctx, actor, guid)
}

// Authenticate resolves a presented secret. Every failure is the same
// Unauthenticated error, so a probe learns nothing about why it was refused.
func (s *TokenService) Authenticate(ctx context.Context, secret string) (*Principal, error) {
	bad := apperr.Unauthenticated("invalid or expired token")
	if !strings.HasPrefix(secret, TokenPrefix) || len(secret) > 128 {
		return nil, bad
	}

	now := s.deps.now()

	t, err := s.deps.Storage.Token().GetActiveByHash(ctx, hashToken(secret), now)
	if err != nil {
		if isNotFound(err) {
			return nil, bad
		}

		return nil, err
	}

	// last_used_at is for people, not for security: at most once a minute, and a
	// failure to record it must never fail the request.
	if now.Sub(t.LastUsedAt) >= time.Minute {
		if err := s.deps.Storage.Token().Touch(ctx, t.Guid, now, now.Add(-time.Minute)); err != nil {
			s.deps.Log.Warn("could not record token use")
		}
	}

	return &Principal{UserID: t.UserGuid, Scope: t.Scope}, nil
}
