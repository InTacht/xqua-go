package http

import (
	"context"
	"slices"
	"time"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// Fallbacks supplies required catalog entries for the global error handler.
// Unhandled backs plain/unmapped errors; NotFound backs unmatched routes.
type Fallbacks struct {
	Unhandled *errors.Error
	NotFound  *errors.Error
}

// Middleware configures HTTP middleware. Nil uses defaults (recover, request ID, access log on).
// RequestID enables the whole correlation stack: the X-Request-ID header
// (generated or honored from the request), the logger context bridge, and the
// X-Client-Request-Id echo.
type Middleware struct {
	Recover   bool
	RequestID bool
	AccessLog bool
	CORS      *cors.Config
	Pprof     bool
}

type Config struct {
	// Host defaults to "0.0.0.0" when empty.
	Host string
	// Port defaults to 8080 when zero or negative.
	Port int

	Logger runtime.Logger

	Middleware *Middleware

	// Catalog is the service's public error catalog: the only errors allowed
	// to cross the wire through route mappings and the global error handler.
	// Internal catalog errors must be mapped into it at the boundary
	// (errors.MapOr); anything else is replaced by Fallbacks.Unhandled.
	// Required.
	Catalog *errors.Catalog

	// Fallbacks must be entries of Catalog. When both Unhandled and NotFound
	// are nil, StandardErrors(Catalog) is applied.
	Fallbacks Fallbacks

	// DefaultStatus is the HTTP status used by the global error handler for
	// plain and unmapped errors. Defaults to 500 when zero or negative.
	// Engines (openapi) read it as the fallback for kinds they do not map.
	DefaultStatus int

	// HealthCheck, when set, backs GET /health: a nil error renders a 200
	// "alive" envelope; a non-nil error renders a 503 envelope. When nil, a
	// simple always-alive /health is still registered.
	HealthCheck func(ctx context.Context) error

	// Version, BuildID, and BuildTime populate the GET /version endpoint.
	// They are optional; empty values are omitted from the response.
	// Version is also the default info.version for OpenAPI documents served
	// by an attached openapi engine.
	Version   string
	BuildID   string
	BuildTime string

	FiberConfig fiber.Config
}

// MethodQuery is the HTTP QUERY method (RFC 9110 / OpenAPI 3.2). The transport
// always registers it with Fiber so the openapi engine's Router.Query works;
// OpenAPI emits it as the "query" operation key.
const MethodQuery = "QUERY"

// Operational defaults applied to FiberConfig when the corresponding field is
// left at its zero value. Every default is overridable via Config.FiberConfig.
const (
	defaultReadTimeout  = 15 * time.Second
	defaultWriteTimeout = 15 * time.Second
	defaultBodyLimit    = 4 * 1024 * 1024 // 4 MiB
)

func applyOperationalDefaults(cfg fiber.Config) fiber.Config {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = defaultReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
	if cfg.BodyLimit == 0 {
		cfg.BodyLimit = defaultBodyLimit
	}
	// Fiber only accepts methods listed in RequestMethods. Always include QUERY
	// (OpenAPI 3.2 / Router.Query) alongside the defaults or the caller's list.
	cfg.RequestMethods = withQueryMethod(cfg.RequestMethods)
	return cfg
}

// withQueryMethod returns methods with MethodQuery present. An empty input
// starts from fiber.DefaultMethods so standard verbs stay registered.
func withQueryMethod(methods []string) []string {
	if len(methods) == 0 {
		methods = fiber.DefaultMethods
	}
	if slices.Contains(methods, MethodQuery) {
		return methods
	}
	out := make([]string, len(methods), len(methods)+1)
	copy(out, methods)
	return append(out, MethodQuery)
}

func defaultMiddleware() Middleware {
	return Middleware{
		Recover:   true,
		RequestID: true,
		AccessLog: true,
	}
}

func resolveMiddleware(cfg *Middleware) Middleware {
	if cfg == nil {
		return defaultMiddleware()
	}
	return *cfg
}

// applyConfigDefaults fills zero-value fields with operational defaults.
// Catalog and Logger stay required; everything else has a sensible zero.
func applyConfigDefaults(cfg Config) Config {
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	if cfg.DefaultStatus <= 0 {
		cfg.DefaultStatus = fiber.StatusInternalServerError
	}
	if cfg.Catalog != nil && cfg.Fallbacks.Unhandled == nil && cfg.Fallbacks.NotFound == nil {
		cfg.Fallbacks = StandardErrors(cfg.Catalog)
	}
	return cfg
}

func validateErrorHandling(cfg Config) {
	if cfg.Catalog == nil {
		panic("http: Catalog is required")
	}
	if cfg.Fallbacks.Unhandled == nil || cfg.Fallbacks.NotFound == nil {
		panic("http: Fallbacks.Unhandled and Fallbacks.NotFound are required")
	}
	if !cfg.Catalog.Contains(cfg.Fallbacks.Unhandled) || !cfg.Catalog.Contains(cfg.Fallbacks.NotFound) {
		panic("http: Fallbacks must be entries of Config.Catalog")
	}
}
