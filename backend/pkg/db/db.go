// Package db owns the Postgres pool and the squirrel builder every repository
// shares.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"gitlab.com/udevs/dbdiagram/backend/config"
)

// Runner is what a repository runs its queries on: the pool, or a transaction
// inside storage.Tx. squirrel accepts it directly through RunWith.
type Runner = squirrel.StdSqlCtx

type Postgres struct {
	Db      *sqlx.DB
	Builder squirrel.StatementBuilderType
}

// DSN builds the connection string. The search path is sent as a startup
// parameter so every pooled connection has it. Never log the result: it
// carries the password.
func DSN(cfg *config.Config) string {
	q := url.Values{}
	q.Set("sslmode", cfg.PostgresSSLMode)

	// Bounds one connect attempt; lib/pq would otherwise inherit the kernel's
	// SYN timeout.
	q.Set("connect_timeout", "5")

	if cfg.PostgresSearchPath != "" {
		q.Set("search_path", cfg.PostgresSearchPath)
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.PostgresUser, cfg.PostgresPassword),
		Host:     net.JoinHostPort(cfg.PostgresHost, strconv.Itoa(cfg.PostgresPort)),
		Path:     "/" + cfg.PostgresDatabase,
		RawQuery: q.Encode(),
	}

	return u.String()
}

func New(cfg *config.Config) (*Postgres, error) {
	pool, err := sqlx.Open("postgres", DSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	pool.SetMaxOpenConns(cfg.PostgresMaxConns)
	pool.SetMaxIdleConns(cfg.PostgresMaxIdleConns)
	pool.SetConnMaxLifetime(time.Duration(cfg.PostgresConnMaxLifetimeMinutes) * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Postgres{
		Db:      pool,
		Builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}, nil
}

func (p *Postgres) Close() { _ = p.Db.Close() }

// NullString stores "" as NULL.
func NullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
