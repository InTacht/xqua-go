// Run: go run ./examples/bus
//
// Demonstrates inter-unit communication over a local bus: an HTTP unit
// request/replies on demo.work, and two worker units compete via QueueSubscribe
// (same queue group → one message, one worker).
//
//	curl 'http://127.0.0.1:8080/work?q=hello'
//	# fire a few in parallel to see worker=a vs worker=b in the result:
//	for i in 1 2 3 4; do curl -s "http://127.0.0.1:8080/work?q=$i" & done; wait
package main

import (
	"context"
	"log"
	"time"

	"github.com/InTacht/xqua-go/examples/bus/transport"
	"github.com/InTacht/xqua-go/examples/bus/worker"
	"github.com/InTacht/xqua-go/pkg/bus"
	"github.com/InTacht/xqua-go/pkg/env"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

// Config is the whole process configuration, populated from the environment in
// one call. Required vars fail loudly; the rest fall back to defaults.
type Config struct {
	Name     string        `env:"APP_NAME" default:"bus-example"`
	Host     string        `env:"APP_HOST" default:"0.0.0.0"`
	Port     int           `env:"APP_PORT" default:"8080"`
	Debug    bool          `env:"DEBUG" default:"false"`
	Shutdown time.Duration `env:"SHUTDOWN_TIMEOUT" default:"30s"`
}

// Ctx holds the dependencies shared by the units in this process. The bus is
// constructed in main and drained there too; runtime does not manage it. Unit
// factories in main narrow this down to just what each unit needs.
type Ctx struct {
	Name string
	Host string
	Port int

	Bus bus.Bus
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("bus example: %v", err)
	}
}

func run() error {
	var cfg Config
	if err := env.Bind(&cfg); err != nil {
		return err
	}

	appLog := logger.New(&logger.Config{Name: cfg.Name, ID: cfg.Name, Debug: cfg.Debug})
	defer appLog.Close()

	// Bus is a dependency, not a runtime concern. Build it here and drain it on
	// the way out so in-flight work finishes. Keep this in run() — os.Exit /
	// log.Fatal skip deferred calls.
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

	// Two workers in queue "workers" — competing consumers in one binary. The
	// factory closures narrow the app context and Derive a per-unit log label.
	r.Unit(func(c *Ctx, log runtime.Logger) runtime.Unit {
		return worker.New("a", c.Bus, log.Derive("worker-a"))
	}).Unit(func(c *Ctx, log runtime.Logger) runtime.Unit {
		return worker.New("b", c.Bus, log.Derive("worker-b"))
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
