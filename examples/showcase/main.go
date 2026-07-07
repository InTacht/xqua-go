// Run: make dev-up && go run ./examples/showcase
//
// HTTP showcase — one codebase covering most of the openapi engine:
//   - Postgres users API (/api/v1/users)
//   - In-memory demo routes (/demo/...) — catalog mapping, validation, multipart
//   - Multi-surface OpenAPI (/openapi.json, /mobile/..., /console/..., /demo/...)
//   - Documentation-only Describe (/demo/ws) and imperative Fiber (/demo/leak)
//
// Try:
//
//	curl http://127.0.0.1:8080/api/v1/users
//	curl http://127.0.0.1:8080/demo/items/1
//	curl http://127.0.0.1:8080/demo/items/99
//	curl -X POST http://127.0.0.1:8080/demo/items
//	curl -F title=report -F file=@./README.md http://127.0.0.1:8080/demo/upload
//	curl http://127.0.0.1:8080/demo/leak
//	curl http://127.0.0.1:8080/openapi.json
//	curl http://127.0.0.1:8080/demo/openapi.json
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/InTacht/xqua-go/examples/showcase/migrations"
	"github.com/InTacht/xqua-go/examples/showcase/store"
	"github.com/InTacht/xqua-go/examples/showcase/transport"
	"github.com/InTacht/xqua-go/pkg/env"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/migrate"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Name        string        `env:"APP_NAME" default:"showcase"`
	InstanceID  string        `env:"APP_ID"`
	Host        string        `env:"APP_HOST" default:"0.0.0.0"`
	Port        int           `env:"APP_PORT" default:"8080"`
	Version     string        `env:"APP_VERSION" default:"dev"`
	DatabaseURL string        `env:"DATABASE_URL" default:"postgres://app:app@localhost:5432/app?sslmode=disable"`
	Debug       bool          `env:"DEBUG" default:"false"`
	Shutdown    time.Duration `env:"SHUTDOWN_TIMEOUT" default:"30s"`
}

type Ctx struct {
	Name    string
	Host    string
	Port    int
	Version string
	Pool    *pgxpool.Pool
	Users   *store.Users
}

func (c *Ctx) Ping(ctx context.Context) error {
	if c.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	return c.Pool.Ping(ctx)
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("showcase: %v", err)
	}
}

func run() error {
	var cfg Config
	if err := env.Bind(&cfg); err != nil {
		return err
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = cfg.Name
	}

	appLog := logger.New(&logger.Config{Name: cfg.Name, ID: cfg.InstanceID, Debug: cfg.Debug})
	defer appLog.Close()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	migrator, err := migrate.Postgres(pool, migrate.Config{
		Source:     migrate.Source{FS: migrations.FS, Dir: "."},
		InstanceID: cfg.InstanceID,
	})
	if err != nil {
		return fmt.Errorf("migrator: %w", err)
	}

	appCtx := &Ctx{
		Name:    cfg.Name,
		Host:    cfg.Host,
		Port:    cfg.Port,
		Version: cfg.Version,
		Pool:    pool,
		Users:   store.NewUsers(pool),
	}

	r, err := runtime.New(appCtx, appLog)
	if err != nil {
		return err
	}

	r.OnStartup(func(ctx context.Context) error {
		appLog.Info("running database migrations")
		if err := migrator.RunStartupGate(ctx); err != nil {
			return err
		}
		appLog.Info("migrations complete")
		return nil
	})

	r.Unit(func(c *Ctx, log runtime.Logger) runtime.Unit {
		return transport.HTTP(transport.Deps{
			Host:    c.Host,
			Port:    c.Port,
			Version: c.Version,
			Name:    c.Name,
			Users:   c.Users,
			Ping:    c.Ping,
		}, log.Derive("http"))
	})

	return r.RunWithShutdownTimeout(cfg.Shutdown)
}
