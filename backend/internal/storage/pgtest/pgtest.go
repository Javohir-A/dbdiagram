// Package pgtest opens a real, migrated, empty Postgres database for tests.
// Each caller names its own database, so test packages that run in parallel
// never truncate each other's rows. It skips the test when Postgres is not
// reachable or `go test -short` is used.
package pgtest

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"

	migrations "gitlab.com/udevs/dbdiagram/backend"
	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/migrator"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return def
}

// tables are truncated before every test, children first.
const truncate = `TRUNCATE dbdiagram.access_tokens, dbdiagram.diagram_versions, dbdiagram.diagram_members, dbdiagram.folder_members,
	dbdiagram.sessions, dbdiagram.diagrams, dbdiagram.folders, dbdiagram.users`

// Config returns the connection settings of the test database called name
// (without the "dbdiagram_test_" prefix). TEST_POSTGRES_* override the server.
func Config(name string) *config.Config {
	return &config.Config{
		PostgresHost:                   env("TEST_POSTGRES_HOST", "localhost"),
		PostgresPort:                   5432,
		PostgresDatabase:               "dbdiagram_test_" + name,
		PostgresUser:                   env("TEST_POSTGRES_USER", env("USER", "postgres")),
		PostgresPassword:               env("TEST_POSTGRES_PASSWORD", ""),
		PostgresSSLMode:                "disable",
		PostgresSearchPath:             "dbdiagram,shared,public",
		PostgresMaxConns:               5,
		PostgresMaxIdleConns:           2,
		PostgresConnMaxLifetimeMinutes: 5,
		LockTimeoutSeconds:             10,
		StatementTimeoutSeconds:        60,
		BcryptCost:                     4,
	}
}

// Open returns the storage over a freshly truncated database. The pool closes
// when the test ends.
func Open(t testing.TB, name string) (storage.StorageI, *db.Postgres) {
	t.Helper()

	if testing.Short() {
		t.Skip("requires postgres")
	}

	cfg := Config(name)
	if err := ensureDatabase(cfg); err != nil {
		t.Skipf("test database unavailable: %v", err)
	}

	log := logger.New("error", "test")

	m, err := migrator.New(cfg, log, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	targets, _ := migrator.Targets("")
	if err := m.Up(targets); err != nil {
		m.Close()
		t.Skipf("test database unavailable: %v", err)
	}
	m.Close()

	pg, err := db.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)

	Truncate(t, pg)

	return storage.New(pg, log), pg
}

// Truncate empties every table; suites call it before each test.
func Truncate(t testing.TB, pg *db.Postgres) {
	t.Helper()

	if _, err := pg.Db.Exec(truncate); err != nil {
		t.Fatal(err)
	}
}

// ensureDatabase creates the named test database on first use.
func ensureDatabase(cfg *config.Config) error {
	admin := *cfg
	admin.PostgresDatabase = "postgres"

	conn, err := sql.Open("postgres", db.DSN(&admin))
	if err != nil {
		return err
	}
	defer conn.Close()

	var exists bool
	if err := conn.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, cfg.PostgresDatabase).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Identifiers cannot be bound; the name is built from a fixed prefix and a
	// constant chosen by the test, never from user input.
	_, err = conn.Exec(fmt.Sprintf(`CREATE DATABASE %q`, cfg.PostgresDatabase))

	return err
}
