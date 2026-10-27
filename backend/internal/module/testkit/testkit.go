// Package testkit wires the modules over a storage the test supplies, with a
// clock the test moves by hand and a helper that registers people.
package testkit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/core"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

const Secret = "test-secret-test-secret-test-secret-123456"

type World struct {
	T        *testing.T
	Cfg      *config.Config
	Storage  storage.StorageI
	Core     *core.Module
	Diagrams *diagrams.Module
	// Now is the clock both modules read; tests advance it.
	Now *time.Time
}

// Config is a develop-mode config with cheap hashing, for tests.
func Config() *config.Config {
	return &config.Config{
		Environment: "develop", JWTSecret: Secret, JWTAccessTTLMinutes: 15, JWTRefreshTTLHours: 720,
		RegistrationEnabled: true, BcryptCost: 4, AuthRateLimitPerMin: 1000, CtxTimeout: 5,
	}
}

func New(t *testing.T, st storage.StorageI) *World {
	t.Helper()

	var (
		cfg = Config()
		log = logger.New("error", "test")
		now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		clk = func() time.Time { return now }
		c   = core.New(core.Deps{Storage: st, Log: log, Cfg: cfg, Now: clk})
	)

	return &World{
		T: t, Cfg: cfg, Storage: st, Core: c, Now: &now,
		Diagrams: diagrams.New(diagrams.Deps{Storage: st, Log: log, Cfg: cfg, Users: c.User, Now: clk}),
	}
}

// Person registers an account and returns its guid.
func (w *World) Person(name string) string {
	w.T.Helper()

	res, err := w.Core.Auth.Register(context.Background(), core.RegisterParams{
		Email: name + "@example.test", Name: name, Password: "correct horse 9",
	})
	require.NoError(w.T, err)

	return res.User.Guid
}
