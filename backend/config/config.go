package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

// Config holds every runtime setting. Values come from ENV only; secrets are
// never hardcoded as defaults.
type Config struct {
	Environment string // develop, staging, production
	LogLevel    string
	HttpPort    string
	CtxTimeout  int // seconds — the per-request deadline

	// CORSAllowedOrigins is a comma separated list. Empty allows every origin,
	// which is only accepted outside production.
	CORSAllowedOrigins []string
	// TrustedProxies are the proxy addresses whose X-Forwarded-For is believed
	// (the rate limiter keys on the client IP). Empty trusts nobody, so the
	// header cannot be forged to dodge the limit.
	TrustedProxies []string

	PostgresHost     string
	PostgresPort     int
	PostgresDatabase string
	PostgresUser     string
	PostgresPassword string
	PostgresSSLMode  string
	// PostgresSearchPath — tables live in the dbdiagram schema, helper
	// functions in shared.
	PostgresSearchPath             string
	PostgresMaxConns               int
	PostgresMaxIdleConns           int
	PostgresConnMaxLifetimeMinutes int

	// JWTSecret signs access tokens. serve refuses to start without it.
	JWTSecret           string
	JWTAccessTTLMinutes int
	JWTRefreshTTLHours  int
	RegistrationEnabled bool
	BcryptCost          int
	AuthRateLimitPerMin int // requests per IP per minute on register/login/refresh

	LockTimeoutSeconds      int
	StatementTimeoutSeconds int
	// MigrateOnStart applies pending migrations before serving. Off by
	// default: several replicas would race, and a failed migration must stop
	// the pipeline, not crash-loop a pod.
	MigrateOnStart bool
}

func (c *Config) IsProduction() bool { return c.Environment == "production" }

// Load reads ENV (a mounted /app/.env first, then a local .env).
func Load() Config {
	if err := godotenv.Load("/app/.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Print("No .env file found")
		}
	}

	c := Config{}

	c.Environment = cast.ToString(getOrReturnDefault("ENVIRONMENT", "develop"))
	c.LogLevel = cast.ToString(getOrReturnDefault("LOG_LEVEL", "debug"))
	c.HttpPort = cast.ToString(getOrReturnDefault("HTTP_PORT", "8080"))
	c.CtxTimeout = cast.ToInt(getOrReturnDefault("CTX_TIMEOUT", 15))

	for _, origin := range strings.Split(cast.ToString(getOrReturnDefault("CORS_ALLOWED_ORIGINS", "")), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			c.CORSAllowedOrigins = append(c.CORSAllowedOrigins, origin)
		}
	}

	for _, proxy := range strings.Split(cast.ToString(getOrReturnDefault("TRUSTED_PROXIES", "")), ",") {
		if proxy = strings.TrimSpace(proxy); proxy != "" {
			c.TrustedProxies = append(c.TrustedProxies, proxy)
		}
	}

	c.PostgresHost = cast.ToString(getOrReturnDefault("POSTGRES_HOST", "localhost"))
	c.PostgresPort = cast.ToInt(getOrReturnDefault("POSTGRES_PORT", 5432))
	c.PostgresDatabase = cast.ToString(getOrReturnDefault("POSTGRES_DATABASE", "dbdiagram"))
	c.PostgresUser = cast.ToString(getOrReturnDefault("POSTGRES_USER", "postgres"))
	c.PostgresPassword = cast.ToString(getOrReturnDefault("POSTGRES_PASSWORD", ""))
	c.PostgresSSLMode = cast.ToString(getOrReturnDefault("POSTGRES_SSLMODE", "disable"))
	c.PostgresSearchPath = cast.ToString(getOrReturnDefault("POSTGRES_SEARCH_PATH", "dbdiagram,shared,public"))
	c.PostgresMaxConns = cast.ToInt(getOrReturnDefault("POSTGRES_MAX_CONNS", 30))
	c.PostgresMaxIdleConns = cast.ToInt(getOrReturnDefault("POSTGRES_MAX_IDLE_CONNS", 10))
	c.PostgresConnMaxLifetimeMinutes = cast.ToInt(getOrReturnDefault("POSTGRES_CONN_MAX_LIFETIME_MINUTES", 30))

	c.JWTSecret = cast.ToString(getOrReturnDefault("JWT_SECRET", ""))
	c.JWTAccessTTLMinutes = cast.ToInt(getOrReturnDefault("JWT_ACCESS_TTL_MINUTES", 15))
	c.JWTRefreshTTLHours = cast.ToInt(getOrReturnDefault("JWT_REFRESH_TTL_HOURS", 720))
	c.RegistrationEnabled = cast.ToBool(getOrReturnDefault("REGISTRATION_ENABLED", true))
	c.BcryptCost = cast.ToInt(getOrReturnDefault("BCRYPT_COST", 12))
	c.AuthRateLimitPerMin = cast.ToInt(getOrReturnDefault("AUTH_RATE_LIMIT_PER_MIN", 20))

	c.LockTimeoutSeconds = cast.ToInt(getOrReturnDefault("LOCK_TIMEOUT_SECONDS", 10))
	c.StatementTimeoutSeconds = cast.ToInt(getOrReturnDefault("STATEMENT_TIMEOUT_SECONDS", 300))
	c.MigrateOnStart = cast.ToBool(getOrReturnDefault("MIGRATE_ON_START", false))

	return c
}

func getOrReturnDefault(key string, defaultValue any) any {
	if v, exists := os.LookupEnv(key); exists {
		return v
	}
	return defaultValue
}
