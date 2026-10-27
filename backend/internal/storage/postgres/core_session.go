package postgres

import (
	"context"
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/repo"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type session struct {
	db     *db.Postgres
	runner db.Runner
	log    logger.Logger
}

// NewSessionRepo returns the `sessions` repository.
func NewSessionRepo(pgDb *db.Postgres, runner db.Runner, log logger.Logger) repo.SessionI {
	return &session{db: pgDb, runner: runner, log: log}
}

func (r *session) Create(ctx context.Context, p *models.SessionCreate) error {
	_, err := r.db.Builder.Insert("sessions").
		Columns("guid", "user_guid", "family_guid", "token_hash", "user_agent", "expires_at").
		Values(uuid.NewString(), p.UserGuid, p.FamilyGuid, p.TokenHash, p.UserAgent, p.ExpiresAt).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return HandleDatabaseError(err, r.log, "create session")
	}

	return nil
}

func (r *session) GetByHash(ctx context.Context, tokenHash string) (*models.Session, error) {
	var (
		res       = &models.Session{}
		revokedAt sql.NullTime
	)

	query := r.db.Builder.Select("guid", "user_guid", "family_guid", "expires_at", "revoked_at").From("sessions").
		Where(squirrel.Eq{"token_hash": tokenHash})
	if err := query.RunWith(r.runner).QueryRowContext(ctx).Scan(&res.Guid, &res.UserGuid, &res.FamilyGuid, &res.ExpiresAt, &revokedAt); err != nil {
		return nil, HandleDatabaseError(err, r.log, "get session")
	}

	res.Revoked = revokedAt.Valid

	return res, nil
}

func (r *session) revoke(ctx context.Context, where squirrel.Sqlizer, message string) (int64, error) {
	result, err := r.db.Builder.Update("sessions").Set("revoked_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"revoked_at": nil}).Where(where).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return 0, HandleDatabaseError(err, r.log, message)
	}

	return result.RowsAffected()
}

func (r *session) Revoke(ctx context.Context, guid string) (bool, error) {
	n, err := r.revoke(ctx, squirrel.Eq{"guid": guid}, "revoke session")

	return n > 0, err
}

func (r *session) RevokeFamily(ctx context.Context, familyGuid string) error {
	_, err := r.revoke(ctx, squirrel.Eq{"family_guid": familyGuid}, "revoke session family")

	return err
}

func (r *session) RevokeAllForUser(ctx context.Context, userGuid, exceptFamily string) error {
	where := squirrel.And{squirrel.Eq{"user_guid": userGuid}}
	if exceptFamily != "" {
		where = append(where, squirrel.NotEq{"family_guid": exceptFamily})
	}

	_, err := r.revoke(ctx, where, "revoke user sessions")

	return err
}
