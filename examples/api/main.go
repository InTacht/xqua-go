// Run: go run ./examples/api
//
// Full xqua-go HTTP process: env config, Postgres, migrations, graceful
// shutdown, structured errors, and request-scoped logging.
//
// Start the dev database first:
//
//	make dev-up
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/InTacht/xqua-go/examples/api/migrations"
	"github.com/InTacht/xqua-go/examples/api/store"
	"github.com/InTacht/xqua-go/examples/api/transport"
	"github.com/InTacht/xqua-go/pkg/env"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/migrate"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config is process configuration, populated from the environment in one Bind.
type Config struct {
	Name        string        `env:"APP_NAME" default:"api-example"`
	InstanceID  string        `env:"APP_ID"`
	Host        string        `env:"APP_HOST" default:"0.0.0.0"`
	Port        int           `env:"APP_PORT" default:"8080"`
	Version     string        `env:"APP_VERSION" default:"dev"`
	DatabaseURL string        `env:"DATABASE_URL" default:"postgres://app:app@localhost:5432/app?sslmode=disable"`
	Debug       bool          `env:"DEBUG" default:"false"`
	Shutdown    time.Duration `env:"SHUTDOWN_TIMEOUT" default:"30s"`
}

// Ctx is the process bag of shared deps. Built and released in run(); runtime
// does not manage its lifecycle. Unit factories narrow it to only what each
// unit uses — pass values, not the bag.
type Ctx struct {
	Name    string
	Host    string
	Port    int
	Version string

	Pool  *pgxpool.Pool
	Users *store.Users
}

// Ping checks database connectivity for GET /health.
func (c *Ctx) Ping(ctx context.Context) error {
	if c.Pool == nil {
		return fmt.Errorf("ctx: database not initialized")
	}
	return c.Pool.Ping(ctx)
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("api example: %v", err)
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

	// Build deps here and defer cleanup — keep this in run() so os.Exit /
	// log.Fatal cannot skip teardown.
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
	}).OnShutdown(func(ctx context.Context) error {
		appLog.InfoCtx(ctx, "draining in-flight work")
		time.Sleep(200 * time.Millisecond)
		appLog.InfoCtx(ctx, "drain complete")
		return nil
	})

	// Factory closures narrow Ctx to transport.Deps — the unit package never
	// sees the process bag.
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
