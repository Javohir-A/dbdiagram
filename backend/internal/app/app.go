// Package app is the wiring root: every module, built once, over one database
// pool.
package app

import (
	"context"
	"fmt"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/core"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type App struct {
	Cfg *config.Config
	Log logger.Logger

	Core     *core.Module
	Diagrams *diagrams.Module

	pg *db.Postgres
}

func New(cfg *config.Config, log logger.Logger) (*App, error) {
	pg, err := db.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	st := storage.New(pg, log)

	// core has no module dependencies, so it is built first and handed to the
	// others as their UserGateway.
	c := core.New(core.Deps{Storage: st, Log: log, Cfg: cfg})

	return &App{
		Cfg:      cfg,
		Log:      log,
		Core:     c,
		Diagrams: diagrams.New(diagrams.Deps{Storage: st, Log: log, Cfg: cfg, Users: c.User}),
		pg:       pg,
	}, nil
}

// Ping reports whether the database answers; the health endpoint uses it.
func (a *App) Ping(ctx context.Context) error {
	if err := a.pg.Db.PingContext(ctx); err != nil {
		return apperr.Unavailable(err, "ping database")
	}

	return nil
}

func (a *App) Close() { a.pg.Close() }
