package http

import (
	"context"
	"fmt"

	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/transport"

	"github.com/gofiber/fiber/v3"
)

// Transport is an HTTP transport powered by Fiber v3.
type Transport struct {
	host string
	port int

	app *fiber.App
	log *logger.Logger
}

// New creates an HTTP transport from explicit configuration and dependencies.
func New(cfg Config) *Transport {
	if cfg.Host == "" || cfg.Port <= 0 {
		panic("http: host and port are required")
	}

	if cfg.Logger == nil {
		panic("http: logger is required")
	}

	return &Transport{
		host: cfg.Host,
		port: cfg.Port,
		app:  fiber.New(cfg.FiberConfig),
		log:  cfg.Logger.Derive("transport.http"),
	}
}

// Name identifies the transport implementation.
func (t *Transport) Name() string {
	return "http"
}

// Fiber returns the underlying Fiber application for route registration.
func (t *Transport) Fiber() *fiber.App {
	return t.app
}

// Routes registers routes under a path prefix.
func (t *Transport) Routes(path string, initFn func(fiber.Router)) *Transport {
	initFn(t.app.Group(path))
	return t
}

// Serve starts the HTTP server and blocks until Shutdown is called or an error occurs.
func (t *Transport) Serve(opts transport.ServeOptions) error {
	addr := fmt.Sprintf("%s:%d", t.host, t.port)

	if opts.OnReady != nil {
		t.app.Hooks().OnListen(func(_ fiber.ListenData) error {
			opts.OnReady()
			t.log.Info("transport ready", t.Name(), addr)
			return nil
		})
	}

	if err := t.app.Listen(addr); err != nil {
		return fmt.Errorf("http: listen: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (t *Transport) Shutdown(ctx context.Context) error {
	if err := t.app.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("http: shutdown: %w", err)
	}
	return nil
}

// func (t *Transport[T]) installMiddleware() {
// 	mw := t.cfg.Middleware

// 	if mw.Recover {
// 		t.app.Use(recover.New())
// 	}
// 	if mw.RequestID {
// 		t.app.Use(requestid.New())
// 		t.app.Use(RequestContext())
// 	}
// 	if mw.AccessLog {
// 		t.app.Use(fiberzap.New(fiberzap.Config{Logger: t.deps.Logger.Zap()}))
// 	}
// 	if mw.CORS != nil {
// 		t.app.Use(cors.New(*mw.CORS))
// 	}
// 	if mw.Pprof {
// 		t.app.Use(pprof.New())
// 	}
// }
