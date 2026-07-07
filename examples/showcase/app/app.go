// Run: make dev-up && go run ./examples/showcase
//
// Full application layout: domain → repository → service → transport.
//
// Backends wired in app/wire.go (repository.Repo):
//   - core Postgres  — users (primary)
//   - demo Postgres  — audit trail (second database)
//   - memory         — demo items + API tokens (ephemeral / Redis-like)
//
// Try:
//
//	curl http://127.0.0.1:8080/api/v1/users
//	curl http://127.0.0.1:8080/api/v1/users/1
//	curl http://127.0.0.1:8080/api/v1/users/1/audit
//	curl http://127.0.0.1:8080/demo/items/1
//	curl http://127.0.0.1:8080/demo/items/99
//	curl -X POST http://127.0.0.1:8080/demo/items
//	curl -F title=report -F file=@./README.md http://127.0.0.1:8080/demo/upload
//	curl -X POST http://127.0.0.1:8080/demo/auth/login -d '{"username":"demo","password":"secret"}'
//	curl http://127.0.0.1:8080/demo/leak
//	curl http://127.0.0.1:8080/openapi.json
package app

import (
	"context"
	"fmt"
	"time"

	coremigrate "github.com/InTacht/xqua-go/examples/showcase/app/migrations/core"
	demomigrate "github.com/InTacht/xqua-go/examples/showcase/app/migrations/demo"
	"github.com/InTacht/xqua-go/examples/showcase/app/transport"
	"github.com/InTacht/xqua-go/pkg/env"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/migrate"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config is loaded from the environment on startup.
type Config struct {
	Name            string        `env:"APP_NAME" default:"showcase"`
	InstanceID      string        `env:"APP_ID"`
	Host            string        `env:"APP_HOST" default:"0.0.0.0"`
	Port            int           `env:"APP_PORT" default:"8080"`
	Version         string        `env:"APP_VERSION" default:"dev"`
	CoreDatabaseURL string        `env:"DATABASE_URL" default:"postgres://app:app@localhost:5432/app?sslmode=disable"`
	DemoDatabaseURL string        `env:"DEMO_DATABASE_URL" default:"postgres://app:app@localhost:5432/demo?sslmode=disable"`
	Debug           bool          `env:"DEBUG" default:"false"`
	Shutdown        time.Duration `env:"SHUTDOWN_TIMEOUT" default:"30s"`
}

type runtimeCtx struct {
	Name     string
	Host     string
	Port     int
	Version  string
	Services *Services
}

// Run loads config, wires dependencies, and blocks until shutdown.
func Run() error {
	var cfg Config
	if err := env.Bind(&cfg); err != nil {
		return err
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = cfg.Name
	}

	appLog := logger.New(&logger.Config{Name: cfg.Name, ID: cfg.InstanceID, Debug: cfg.Debug})
	defer appLog.Close()

	corePool, err := pgxpool.New(context.Background(), cfg.CoreDatabaseURL)
	if err != nil {
		return fmt.Errorf("connect core database: %w", err)
	}
	defer corePool.Close()

	demoPool, err := pgxpool.New(context.Background(), cfg.DemoDatabaseURL)
	if err != nil {
		return fmt.Errorf("connect demo database: %w", err)
	}
	defer demoPool.Close()

	coreMigrator, err := migrate.Postgres(corePool, migrate.Config{
		Source:     migrate.Source{FS: coremigrate.FS, Dir: "."},
		InstanceID: cfg.InstanceID + "-core",
	})
	if err != nil {
		return fmt.Errorf("core migrator: %w", err)
	}

	demoMigrator, err := migrate.Postgres(demoPool, migrate.Config{
		Source:     migrate.Source{FS: demomigrate.FS, Dir: "."},
		InstanceID: cfg.InstanceID + "-demo",
	})
	if err != nil {
		return fmt.Errorf("demo migrator: %w", err)
	}

	services, err := Wire(WireDeps{
		Core: corePool,
		Demo: demoPool,
	})
	if err != nil {
		return err
	}

	appCtx := &runtimeCtx{
		Name:     cfg.Name,
		Host:     cfg.Host,
		Port:     cfg.Port,
		Version:  cfg.Version,
		Services: services,
	}

	r, err := runtime.New(appCtx, appLog)
	if err != nil {
		return err
	}

	r.OnStartup(func(ctx context.Context) error {
		appLog.Info("running core database migrations")
		if err := coreMigrator.RunStartupGate(ctx); err != nil {
			return err
		}
		appLog.Info("running demo database migrations")
		if err := demoMigrator.RunStartupGate(ctx); err != nil {
			return err
		}
		appLog.Info("migrations complete")
		return nil
	})

	r.Unit(func(c *runtimeCtx, log runtime.Logger) runtime.Unit {
		return transport.HTTP(transport.Config{
			Host:    c.Host,
			Port:    c.Port,
			Version: c.Version,
			Name:    c.Name,
			Users:   c.Services.Users,
			Items:   c.Services.Items,
			Auth:    c.Services.Auth,
			Ping:    c.Services.Ping,
		}, log.Derive("http"))
	})

	return r.RunWithShutdownTimeout(cfg.Shutdown)
}
