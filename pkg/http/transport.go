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

// Transport is an HTTP transport powered by Fiber v3.
type Transport struct {
	host string
	port int

	app           *fiber.App
	log           runtime.Logger
	catalog       *errors.Catalog
	fallbacks     Fallbacks
	defaultStatus int
	kindStatuses  KindStatuses
	healthCheck   func(ctx context.Context) error
	build         buildInfo
	rec           *recorder
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
		kindStatuses:  resolveKindStatuses(cfg.KindStatuses),
		healthCheck:   cfg.HealthCheck,
		build:         buildInfo{Version: cfg.Version, BuildID: cfg.BuildID, BuildTime: cfg.BuildTime},
		rec:           &recorder{},
	}

	t.installMiddleware(resolveMiddleware(cfg.Middleware))
	t.installOps()
	return t
}

// Manifest returns the transport's registration-time bookkeeping: every route
// registered so far (with resolved per-error HTTP status), the full public
// catalog, and the envelope version. It is pure data for future OpenAPI/TS
// generation; call it after all routes are registered.
func (t *Transport) Manifest() Manifest {
	return Manifest{
		EnvelopeVersion: EnvelopeVersion,
		Routes:          t.rec.snapshot(),
		Catalog:         catalogSpecs(t.catalog, t.kindStatuses, t.defaultStatus),
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

// Routes registers routes under a path prefix. The Router passed to register
// wraps handlers so returned catalog errors are mapped to HTTP status codes.
func (t *Transport) Routes(path string, register func(r *Router)) *Transport {
	register(newRouter(t.app.Group(path), t.catalog, t.kindStatuses, t.defaultStatus, path, t.rec))
	return t
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
