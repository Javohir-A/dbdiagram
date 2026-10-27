package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/repo"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

const userColumns = `u.guid, u.email, u.name, u.created_at`

type user struct {
	db     *db.Postgres
	runner db.Runner
	log    logger.Logger
}

// NewUserRepo returns the `users` repository.
func NewUserRepo(pgDb *db.Postgres, runner db.Runner, log logger.Logger) repo.UserI {
	return &user{db: pgDb, runner: runner, log: log}
}

func (r *user) Create(ctx context.Context, p *models.UserCreate) (*model.User, error) {
	var (
		guid = uuid.NewString()
	)

	_, err := r.db.Builder.Insert("users").
		Columns("guid", "email", "name", "password_hash").
		Values(guid, p.Email, p.Name, p.PasswordHash).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		res := HandleDatabaseError(err, r.log, "create user")

		var appErr *apperr.Error
		if errors.As(res, &appErr) && appErr.Kind == apperr.KindAlreadyExists {
			return nil, apperr.AlreadyExists("an account with this email already exists")
		}

		return nil, res
	}

	return r.Get(ctx, guid)
}

func (r *user) Get(ctx context.Context, guid string) (*model.User, error) {
	query := r.db.Builder.Select(userColumns).From("users u").
		Where(NotDeleted("u")).Where(squirrel.Eq{"u.guid": guid})

	res, err := scanUser(query.RunWith(r.runner).QueryRowContext(ctx))
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get user")
	}

	return res, nil
}

func (r *user) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := r.db.Builder.Select(userColumns).From("users u").
		Where(NotDeleted("u")).Where(squirrel.Expr("lower(u.email) = lower(?)", email))

	res, err := scanUser(query.RunWith(r.runner).QueryRowContext(ctx))
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get user by email")
	}

	return res, nil
}

func (r *user) GetCredentials(ctx context.Context, email string) (*models.Credentials, error) {
	var (
		res       = &models.Credentials{}
		createdAt sql.NullTime
	)

	query := r.db.Builder.Select("u.guid", "u.email", "u.name", "u.created_at", "u.password_hash").From("users u").
		Where(NotDeleted("u")).Where(squirrel.Expr("lower(u.email) = lower(?)", email))

	err := query.RunWith(r.runner).QueryRowContext(ctx).Scan(&res.User.Guid, &res.User.Email, &res.User.Name, &createdAt, &res.PasswordHash)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get credentials")
	}

	res.User.CreatedAt = TimeString(createdAt)

	return res, nil
}

func (r *user) GetPasswordHash(ctx context.Context, guid string) (string, error) {
	var hash string

	query := r.db.Builder.Select("password_hash").From("users").
		Where(squirrel.Eq{"guid": guid, "deleted_at": nil})
	if err := query.RunWith(r.runner).QueryRowContext(ctx).Scan(&hash); err != nil {
		return "", HandleDatabaseError(err, r.log, "get password hash")
	}

	return hash, nil
}

func (r *user) UpdateName(ctx context.Context, guid, name string) (*model.User, error) {
	result, err := r.db.Builder.Update("users").Set("name", name).
		Where(squirrel.Eq{"guid": guid, "deleted_at": nil}).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "update user")
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return nil, apperr.NotFound("user not found")
	}

	return r.Get(ctx, guid)
}

func (r *user) UpdatePassword(ctx context.Context, guid, passwordHash string) error {
	result, err := r.db.Builder.Update("users").Set("password_hash", passwordHash).
		Where(squirrel.Eq{"guid": guid, "deleted_at": nil}).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return HandleDatabaseError(err, r.log, "update password")
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return apperr.NotFound("user not found")
	}

	return nil
}

func scanUser(row rowScanner) (*model.User, error) {
	var (
		res       = &model.User{}
		createdAt sql.NullTime
	)

	if err := row.Scan(&res.Guid, &res.Email, &res.Name, &createdAt); err != nil {
		return nil, err
	}

	res.CreatedAt = TimeString(createdAt)

	return res, nil
}
