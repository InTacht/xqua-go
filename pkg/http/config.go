package http

import (
	"context"
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
	// plain and unmapped errors, and by the Router when a returned catalog
	// error's kind is not present in KindStatuses. Defaults to 500 when zero
	// or negative.
	DefaultStatus int

	// KindStatuses maps semantic error kinds to HTTP status codes. When a
	// returned catalog error has no explicit per-route Status/Statuses
	// mapping, the Router resolves its status from this table by kind (see
	// errors.Kind* and DefaultKindStatuses). Unknown kinds fall back to
	// DefaultStatus. When nil, DefaultKindStatuses() is used.
	KindStatuses KindStatuses

	// HealthCheck, when set, backs GET /health: a nil error renders a 200
	// "alive" envelope; a non-nil error renders a 503 envelope. When nil, a
	// simple always-alive /health is still registered.
	HealthCheck func(ctx context.Context) error

	// Version, BuildID, and BuildTime populate the GET /version endpoint.
	// They are optional; empty values are omitted from the response.
	Version   string
	BuildID   string
	BuildTime string

	FiberConfig fiber.Config
}

// Operational defaults applied to FiberConfig when the corresponding field is
// left at its zero value. Every default is overridable via Config.FiberConfig.
const (
	defaultReadTimeout  = 15 * time.Second
	defaultWriteTimeout = 15 * time.Second
	defaultBodyLimit    = 4 * 1024 * 1024 // 4 MiB
)

func resolveKindStatuses(ks KindStatuses) KindStatuses {
	if ks == nil {
		return DefaultKindStatuses()
	}
	return ks
}

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
	return cfg
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
