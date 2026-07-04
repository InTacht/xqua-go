// Run: go run ./examples/split
//
// Compute and storage as two units in one runtime. They share no pointers —
// only bus subjects (storage.get / storage.put). Same pattern scales to
// separate processes once a cluster Bus backend exists.
//
//	curl http://127.0.0.1:8080/kv/greeting
//	curl -X PUT -d 'world' http://127.0.0.1:8080/kv/greeting
//	curl http://127.0.0.1:8080/kv/greeting
package main

import (
	"context"
	"log"
	"time"

	"github.com/InTacht/xqua-go/examples/split/storage"
	"github.com/InTacht/xqua-go/examples/split/transport"
	"github.com/InTacht/xqua-go/pkg/bus"
	"github.com/InTacht/xqua-go/pkg/env"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

// Config is the whole process configuration, populated from the environment.
type Config struct {
	Name     string        `env:"APP_NAME" default:"split"`
	Host     string        `env:"APP_HOST" default:"0.0.0.0"`
	Port     int           `env:"APP_PORT" default:"8080"`
	Shutdown time.Duration `env:"SHUTDOWN_TIMEOUT" default:"30s"`
}

// Ctx holds the dependencies shared by the compute (HTTP) and storage units.
// The bus is built in main and drained there; runtime does not manage it. Unit
// factories in main narrow this down to just what each unit needs.
type Ctx struct {
	Name string
	Host string
	Port int
	Bus  bus.Bus
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("split example: %v", err)
	}
}

func run() error {
	var cfg Config
	if err := env.Bind(&cfg); err != nil {
		return err
	}

	appLog := logger.New(&logger.Config{Name: cfg.Name, ID: cfg.Name})
	defer appLog.Close()

	local := bus.NewLocal(bus.LocalConfig{
		OnError: func(msg bus.Message, err error) {
			appLog.Error(err, "bus handler error", msg.Subject)
		},
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown)
		defer cancel()
		if err := local.Drain(ctx); err != nil {
			appLog.Error(err, "bus drain failed")
		}
	}()

	appCtx := &Ctx{
		Name: cfg.Name,
		Host: cfg.Host,
		Port: cfg.Port,
		Bus:  local,
	}

	r, err := runtime.New(appCtx, appLog)
	if err != nil {
		return err
	}

	// Factory closures narrow Ctx to only what each unit needs.
	r.Unit(func(c *Ctx, log runtime.Logger) runtime.Unit {
		return storage.New(c.Bus, log.Derive("storage"))
	}).Unit(func(c *Ctx, log runtime.Logger) runtime.Unit {
		return transport.HTTP(transport.Deps{
			Name: c.Name,
			Host: c.Host,
			Port: c.Port,
			Bus:  c.Bus,
		}, log.Derive("http"))
	})

	return r.RunWithShutdownTimeout(cfg.Shutdown)
}
