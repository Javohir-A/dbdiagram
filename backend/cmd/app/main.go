// Command app is the single entrypoint: serve | migrate | claim.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"gitlab.com/udevs/dbdiagram/backend"
	"gitlab.com/udevs/dbdiagram/backend/api"
	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/app"
	"gitlab.com/udevs/dbdiagram/backend/internal/migrator"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

// minJWTSecretLength is 256 bits of hex; `openssl rand -hex 32` makes one.
const minJWTSecretLength = 32

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1) // CI and k8s read this exit code
	}
}

func run() error {
	cmd := "serve"
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		// Two statements on purpose: in one tuple assignment, append would
		// overwrite os.Args[1] before it is read.
		cmd = os.Args[1]
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	cfg := config.Load()
	log := logger.New(cfg.LogLevel, config.AppName)
	defer func() { _ = logger.Cleanup(log) }()

	switch cmd {
	case "serve":
		return serve(&cfg, log)
	case "migrate":
		return migrate(&cfg, log)
	case "claim":
		return claim(&cfg, log)
	default:
		return fmt.Errorf("unknown command %q (serve | migrate | claim)", cmd)
	}
}

// serve runs the HTTP API.
func serve(cfg *config.Config, log logger.Logger) error {
	// An empty or short secret would let anyone mint tokens. Refuse to start.
	if len(cfg.JWTSecret) < minJWTSecretLength {
		return fmt.Errorf("JWT_SECRET must be at least %d characters — refusing to start", minJWTSecretLength)
	}
	if cfg.BcryptCost < 10 && cfg.IsProduction() {
		return fmt.Errorf("BCRYPT_COST below 10 is not allowed in production")
	}

	if cfg.IsProduction() && len(cfg.CORSAllowedOrigins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS is empty — refusing to start in production")
	}

	if cfg.MigrateOnStart {
		if err := migrate(cfg, log); err != nil {
			return err
		}
	}

	application, err := app.New(cfg, log)
	if err != nil {
		return err
	}
	defer application.Close()

	server := api.New(&api.Options{Cfg: cfg, Log: log, App: application})

	// Graceful shutdown: in-flight requests finish, the pool closes after them.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return server.Run(ctx, ":"+cfg.HttpPort)
}

// migrate is the standalone migrations tool, folded into the same binary.
func migrate(cfg *config.Config, log logger.Logger) error {
	var (
		fs      = flag.NewFlagSet("migrate", flag.ContinueOnError)
		sub     = fs.String("cmd", "up", "up | down | status | force")
		set     = fs.String("set", "", "one migration set only (empty = all, in registry order)")
		steps   = fs.Int("steps", 1, "number of steps for down")
		version = fs.Int("version", 0, "version for force")
	)

	// MigrateOnStart reaches here with serve's own flags already consumed.
	if cfg.MigrateOnStart && len(os.Args) > 1 {
		*sub = "up"
	} else if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	m, err := migrator.New(cfg, log, migrations.FS)
	if err != nil {
		return err
	}
	defer m.Close()

	targets, err := migrator.Targets(*set)
	if err != nil {
		return err
	}

	switch *sub {
	case "up":
		return m.Up(targets)
	case "status":
		return m.Status(targets)
	case "down":
		// -set is required: without it a typo would tear down the whole database.
		if *set == "" {
			return fmt.Errorf("down requires -set")
		}

		return m.Down(targets[0], *steps)
	case "force":
		if *set == "" {
			return fmt.Errorf("force requires -set")
		}

		return m.Force(targets[0], *version)
	default:
		return fmt.Errorf("unknown migrate command: %s", *sub)
	}
}

// claim gives everything created before accounts existed to one user:
// `app claim -email=you@example.com`. Run it once, after registering.
func claim(cfg *config.Config, log logger.Logger) error {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	email := fs.String("email", "", "email of the registered account that takes ownership")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *email == "" {
		return fmt.Errorf("claim requires -email")
	}

	application, err := app.New(cfg, log)
	if err != nil {
		return err
	}
	defer application.Close()

	diagrams, folders, err := application.Diagrams.Diagram.ClaimUnowned(context.Background(), *email)
	if err != nil {
		return err
	}

	fmt.Printf("%s now owns %d diagrams and %d folders that had no owner\n", *email, diagrams, folders)

	return nil
}
