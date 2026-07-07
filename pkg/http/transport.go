package http

import (
	"context"
	"fmt"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/pprof"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// Transport is an HTTP transport powered by Fiber v3. It is protocol-neutral:
// it owns the Fiber app, middleware, operational endpoints, the RES envelope,
// and the global error handler bound to the service's public catalog. REST
// routing and OpenAPI generation live in the pkg/http/openapi engine, which
// attaches to a transport via openapi.New(t, ...) and reads the accessors
// below. Future engines (websocket, graphql, ...) follow the same pattern.
type Transport struct {
	host string
	port int

	app           *fiber.App
	log           runtime.Logger
	catalog       *errors.Catalog
	fallbacks     Fallbacks
	defaultStatus int
	healthCheck   func(ctx context.Context) error
	build         buildInfo
}

// buildInfo carries the values surfaced by GET /version.
type buildInfo struct {
	Version   string
	BuildID   string
	BuildTime string
}

// New creates an HTTP transport from explicit configuration and dependencies.
func New(cfg Config) *Transport {
	cfg = applyConfigDefaults(cfg)

	if cfg.Logger == nil {
		panic("http: logger is required")
	}

	validateErrorHandling(cfg)

	fiberCfg := applyOperationalDefaults(cfg.FiberConfig)
	if fiberCfg.ErrorHandler == nil {
		fiberCfg.ErrorHandler = ErrorHandler(cfg.Logger, cfg.Catalog, cfg.Fallbacks, cfg.DefaultStatus)
	}

	t := &Transport{
		host:          cfg.Host,
		port:          cfg.Port,
		app:           fiber.New(fiberCfg),
		log:           cfg.Logger,
		catalog:       cfg.Catalog,
		fallbacks:     cfg.Fallbacks,
		defaultStatus: cfg.DefaultStatus,
		healthCheck:   cfg.HealthCheck,
		build:         buildInfo{Version: cfg.Version, BuildID: cfg.BuildID, BuildTime: cfg.BuildTime},
	}

	t.installMiddleware(resolveMiddleware(cfg.Middleware))
	t.installOps()
	return t
}

// Name identifies the transport implementation.
func (t *Transport) Name() string {
	return "http"
}

// Fiber returns the underlying Fiber application. It is the single extension
// point: engines (openapi, and future websocket/graphql) and any custom
// handlers register on it directly.
func (t *Transport) Fiber() *fiber.App {
	return t.app
}

// Catalog returns the service's public error catalog — the one wire contract
// the global error handler enforces. Engines read it to map declared errors to
// their protocol representation.
func (t *Transport) Catalog() *errors.Catalog {
	return t.catalog
}

// Logger returns the transport's logger. Engines derive sub-loggers from it.
func (t *Transport) Logger() runtime.Logger {
	return t.log
}

// Version returns Config.Version. Engines use it as the default document
// version (e.g. OpenAPI info.version) when a spec does not set its own.
func (t *Transport) Version() string {
	return t.build.Version
}

// DefaultStatus returns the HTTP status the global error handler uses for
// plain and unmapped errors. Engines fall back to it when a kind has no mapping.
func (t *Transport) DefaultStatus() int {
	return t.defaultStatus
}

// Serve starts the HTTP server and blocks until Shutdown is called or an error occurs.
// Transport implements runtime.Unit.
func (t *Transport) Serve(opts runtime.ServeOptions) error {
	addr := fmt.Sprintf("%s:%d", t.host, t.port)

	if opts.OnReady != nil {
		t.app.Hooks().OnListen(func(_ fiber.ListenData) error {
			opts.OnReady()
			t.log.Info("unit ready", t.Name(), addr)
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

func (t *Transport) installMiddleware(mw Middleware) {
	if mw.Recover {
		t.app.Use(recover.New())
	}
	if mw.RequestID {
		t.app.Use(requestid.New())
		t.app.Use(RequestContext())
		t.app.Use(ClientRequestID())
	}
	if mw.AccessLog {
		t.app.Use(AccessLog(t.log))
	}
	if mw.CORS != nil {
		t.app.Use(cors.New(*mw.CORS))
	}
	if mw.Pprof {
		t.app.Use(pprof.New())
	}
}
