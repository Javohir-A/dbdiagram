package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/repo"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

const tokenColumns = `guid, name, prefix, scope, expires_at, last_used_at, created_at`

type token struct {
	db     *db.Postgres
	runner db.Runner
	log    logger.Logger
}

// NewTokenRepo returns the `access_tokens` repository.
func NewTokenRepo(pgDb *db.Postgres, runner db.Runner, log logger.Logger) repo.TokenI {
	return &token{db: pgDb, runner: runner, log: log}
}

// active is the one definition of a usable token.
func active(now time.Time) squirrel.Sqlizer {
	return squirrel.And{squirrel.Eq{"revoked_at": nil}, squirrel.Gt{"expires_at": now}}
}

func (r *token) Create(ctx context.Context, p *models.TokenCreate) (*model.AccessToken, error) {
	var (
		guid = uuid.NewString()
	)

	_, err := r.db.Builder.Insert("access_tokens").
		Columns("guid", "user_guid", "name", "prefix", "token_hash", "scope", "expires_at").
		Values(guid, p.UserGuid, p.Name, p.Prefix, p.TokenHash, string(p.Scope), p.ExpiresAt).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "create token")
	}

	res, err := scanToken(r.db.Builder.Select(tokenColumns).From("access_tokens").
		Where(squirrel.Eq{"guid": guid}).RunWith(r.runner).QueryRowContext(ctx))
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get token")
	}

	return res, nil
}

func (r *token) ListActive(ctx context.Context, userGuid string, now time.Time) ([]*model.AccessToken, error) {
	rows, err := r.db.Builder.Select(tokenColumns).From("access_tokens").
		Where(squirrel.Eq{"user_guid": userGuid}).Where(active(now)).
		OrderBy("created_at desc, guid desc").
		RunWith(r.runner).QueryContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "list tokens")
	}
	defer rows.Close()

	out := []*model.AccessToken{}
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, HandleDatabaseError(err, r.log, "scan token")
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, HandleDatabaseError(err, r.log, "rows error token")
	}

	return out, nil
}

func (r *token) CountActive(ctx context.Context, userGuid string, now time.Time) (int, error) {
	var n int

	err := r.db.Builder.Select("count(1)").From("access_tokens").
		Where(squirrel.Eq{"user_guid": userGuid}).Where(active(now)).
		RunWith(r.runner).QueryRowContext(ctx).Scan(&n)
	if err != nil {
		return 0, HandleDatabaseError(err, r.log, "count tokens")
	}

	return n, nil
}

func (r *token) GetActiveByHash(ctx context.Context, tokenHash string, now time.Time) (*models.TokenAuth, error) {
	var (
		res      = &models.TokenAuth{}
		scope    string
		lastUsed sql.NullTime
	)

	// The user must still exist: a token outlives nothing it was issued for.
	err := r.db.Builder.Select("t.guid", "t.user_guid", "t.scope", "t.last_used_at").
		From("access_tokens t JOIN users u ON u.guid = t.user_guid AND u.deleted_at IS NULL").
		Where(squirrel.Eq{"t.token_hash": tokenHash, "t.revoked_at": nil}).
		Where(squirrel.Gt{"t.expires_at": now}).
		RunWith(r.runner).QueryRowContext(ctx).Scan(&res.Guid, &res.UserGuid, &scope, &lastUsed)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get token by hash")
	}

	res.Scope = model.TokenScope(scope)
	res.LastUsedAt = lastUsed.Time

	return res, nil
}

func (r *token) Touch(ctx context.Context, guid string, now, before time.Time) error {
	_, err := r.db.Builder.Update("access_tokens").Set("last_used_at", now).
		Where(squirrel.Eq{"guid": guid}).
		Where(squirrel.Or{squirrel.Eq{"last_used_at": nil}, squirrel.Lt{"last_used_at": before}}).
		RunWith(r.runner).ExecContext(ctx)

	return HandleDatabaseError(err, r.log, "touch token")
}

func (r *token) Revoke(ctx context.Context, userGuid, guid string) error {
	result, err := r.db.Builder.Update("access_tokens").Set("revoked_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"guid": guid, "user_guid": userGuid, "revoked_at": nil}).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return HandleDatabaseError(err, r.log, "revoke token")
	}
	if n, err := result.RowsAffected(); err == nil && n == 0 {
		return apperr.NotFound("token not found")
	}

	return nil
}

func scanToken(row rowScanner) (*model.AccessToken, error) {
	var (
		res                          = &model.AccessToken{}
		scope                        string
		expires, lastUsed, createdAt sql.NullTime
	)

	if err := row.Scan(&res.Guid, &res.Name, &res.Prefix, &scope, &expires, &lastUsed, &createdAt); err != nil {
		return nil, err
	}

	res.Scope = model.TokenScope(scope)
	res.ExpiresAt, res.LastUsedAt, res.CreatedAt = TimeString(expires), TimeString(lastUsed), TimeString(createdAt)

	return res, nil
}
