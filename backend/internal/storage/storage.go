// Package storage is the single entry point the service layer has into the
// database.
package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/postgres"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/repo"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

// StorageI hands out the repositories. Adding one touches four places: the
// interface method, the struct field, newStorage() and the getter. Keep them
// alphabetical.
type StorageI interface {
	Access() repo.AccessI
	Diagram() repo.DiagramI
	Folder() repo.FolderI
	Member() repo.MemberI
	Session() repo.SessionI
	Token() repo.TokenI
	User() repo.UserI
	Version() repo.VersionI

	// Tx runs fn inside one transaction. Every repository reached through the
	// StorageI handed to fn writes on that transaction; an error from fn rolls
	// it back.
	Tx(ctx context.Context, fn func(StorageI) error) error
}

type storagePg struct {
	db     *db.Postgres
	runner db.Runner // *sqlx.DB, or *sqlx.Tx inside Tx
	log    logger.Logger

	access  repo.AccessI
	diagram repo.DiagramI
	folder  repo.FolderI
	member  repo.MemberI
	session repo.SessionI
	token   repo.TokenI
	user    repo.UserI
	version repo.VersionI
}

func New(pgDb *db.Postgres, log logger.Logger) StorageI {
	return newStorage(pgDb, pgDb.Db, log)
}

func newStorage(pgDb *db.Postgres, runner db.Runner, log logger.Logger) *storagePg {
	s := &storagePg{db: pgDb, runner: runner, log: log}

	s.access = postgres.NewAccessRepo(pgDb, runner, log)
	s.diagram = postgres.NewDiagramRepo(pgDb, runner, log)
	s.folder = postgres.NewFolderRepo(pgDb, runner, log)
	s.member = postgres.NewMemberRepo(pgDb, runner, log)
	s.session = postgres.NewSessionRepo(pgDb, runner, log)
	s.token = postgres.NewTokenRepo(pgDb, runner, log)
	s.user = postgres.NewUserRepo(pgDb, runner, log)
	s.version = postgres.NewVersionRepo(pgDb, runner, log)

	return s
}

func (s *storagePg) Access() repo.AccessI   { return s.access }
func (s *storagePg) Diagram() repo.DiagramI { return s.diagram }
func (s *storagePg) Folder() repo.FolderI   { return s.folder }
func (s *storagePg) Member() repo.MemberI   { return s.member }
func (s *storagePg) Session() repo.SessionI { return s.session }
func (s *storagePg) Token() repo.TokenI     { return s.token }
func (s *storagePg) User() repo.UserI       { return s.user }
func (s *storagePg) Version() repo.VersionI { return s.version }

func (s *storagePg) Tx(ctx context.Context, fn func(StorageI) error) error {
	pool, ok := s.runner.(*sqlx.DB)
	if !ok {
		// Already inside a transaction: a nested Begin would commit early, so
		// the inner call joins the outer one.
		return fn(s)
	}

	tx, err := pool.BeginTxx(ctx, nil)
	if err != nil {
		return apperr.Internal(err, "begin transaction")
	}

	if err := fn(newStorage(s.db, tx, s.log)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			s.log.Error("rollback failed", logger.Error(rbErr))
		}

		return err
	}

	if err := tx.Commit(); err != nil {
		return apperr.Internal(err, "commit transaction")
	}

	return nil
}
