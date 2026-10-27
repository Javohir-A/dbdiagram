// Package migrator applies the embedded golang-migrate files, one set (and one
// Postgres schema) at a time.
package migrator

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type Migrator struct {
	cfg   *config.Config
	log   logger.Logger
	files fs.FS
	conns []*sql.DB
}

func New(cfg *config.Config, log logger.Logger, files fs.FS) (*Migrator, error) {
	return &Migrator{cfg: cfg, log: log, files: files}, nil
}

func (m *Migrator) Close() {
	for _, c := range m.conns {
		_ = c.Close()
	}
}

// dsn sends the search path and the lock timeout as startup parameters:
// golang-migrate opens its own connection, so a SET on the app pool would not
// reach it. Never log the result — it carries the password.
func (m *Migrator) dsn(s Set) string {
	q := url.Values{}
	q.Set("sslmode", m.cfg.PostgresSSLMode)
	q.Set("connect_timeout", "5")
	q.Set("search_path", s.Schema+",shared,public")

	// Bounds how long DDL waits for a table lock, and the advisory lock
	// golang-migrate takes at startup, so a second pipeline fails fast.
	if m.cfg.LockTimeoutSeconds > 0 {
		q.Set("lock_timeout", strconv.Itoa(m.cfg.LockTimeoutSeconds*1000))
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(m.cfg.PostgresUser, m.cfg.PostgresPassword),
		Host:     net.JoinHostPort(m.cfg.PostgresHost, strconv.Itoa(m.cfg.PostgresPort)),
		Path:     "/" + m.cfg.PostgresDatabase,
		RawQuery: q.Encode(),
	}

	return u.String()
}

func (m *Migrator) instance(s Set) (*migrate.Migrate, error) {
	conn, err := sql.Open("postgres", m.dsn(s))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", s.Name, err)
	}
	m.conns = append(m.conns, conn)

	// Created up front: an unqualified CREATE TABLE would otherwise land in
	// public without a word.
	if _, err := conn.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %q`, s.Schema)); err != nil {
		return nil, fmt.Errorf("create schema %s: %w", s.Schema, err)
	}

	driver, err := migratepg.WithInstance(conn, &migratepg.Config{
		SchemaName:       s.Schema,
		MigrationsTable:  "schema_migrations", // one table per schema
		StatementTimeout: time.Duration(m.cfg.StatementTimeoutSeconds) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("driver %s: %w", s.Name, err)
	}

	source, err := iofs.New(m.files, "migrations/"+s.Name)
	if err != nil {
		return nil, fmt.Errorf("source %s: %w", s.Name, err)
	}

	return migrate.NewWithInstance("iofs", source, "postgres", driver)
}

func (m *Migrator) Up(targets []Set) error {
	for _, s := range targets {
		mg, err := m.instance(s)
		if err != nil {
			return err
		}

		before, dirty, err := mg.Version()
		if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
			return fmt.Errorf("version %s: %w", s.Name, err)
		}
		if dirty {
			return fmt.Errorf("set %s is in DIRTY state at version %d — fix it manually", s.Name, before)
		}

		if err := mg.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("up %s: %w", s.Name, err)
		}

		after, _, _ := mg.Version()
		m.log.Info("migrations applied",
			logger.String("set", s.Name), logger.Int("from", int(before)), logger.Int("to", int(after)))
	}

	return nil
}

func (m *Migrator) Status(targets []Set) error {
	for _, s := range targets {
		mg, err := m.instance(s)
		if err != nil {
			return err
		}

		v, dirty, err := mg.Version()
		if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
			return fmt.Errorf("version %s: %w", s.Name, err)
		}

		fmt.Printf("%-10s version=%d dirty=%t\n", s.Name, v, dirty)
	}

	return nil
}

func (m *Migrator) Down(s Set, steps int) error {
	mg, err := m.instance(s)
	if err != nil {
		return err
	}

	if err := mg.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("down %s: %w", s.Name, err)
	}

	return nil
}

func (m *Migrator) Force(s Set, version int) error {
	mg, err := m.instance(s)
	if err != nil {
		return err
	}

	return mg.Force(version)
}
