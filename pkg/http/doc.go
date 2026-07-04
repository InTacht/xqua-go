// Package http is a headless HTTP runtime: Fiber v3 transport, router, middleware,
// and JSON API response helpers. It stands alone — wire your catalog, routes, and
// config yourself, then plug the resulting Transport into runtime (or any other
// supervisor) via the runtime.Unit interface. This package does not assemble a
// process for you.
//
// Import path: github.com/InTacht/xqua-go/pkg/http
//
// # Transport
//
// New creates a Transport from Config. Logger and Catalog are required; invalid
// config panics at creation time. Host defaults to "0.0.0.0", Port to 8080,
// DefaultStatus to 500, and Fallbacks to StandardErrors(Catalog) when left zero.
// Transport implements runtime.Unit (Serve / Shutdown / Name).
//
//	log := logger.New(&logger.Config{Name: "orders", ID: "orders-api"})
//	r, err := runtime.New(&appCtx, log)
//	if err != nil { /* … */ }
//	r.Unit(func(c *appCtx, log runtime.Logger) runtime.Unit {
//	    return http.New(http.Config{
//	        Logger:  log.Derive("http"),
//	        Catalog: apiCatalog, // public error contract; the rest defaults
//	    }).Routes("/api/v1", func(r *http.Router) {
//	        r.Get("/users/:id", getUser)
//	    })
//	})
//
// StandardErrors(catalog) defines the conventional Unhandled / NotFound
// fallbacks. New applies it automatically when Fallbacks is left zero; call it
// yourself only when you need the entries for explicit Status mappings.
//
// Default middleware installs recover, request ID, and access log when
// Middleware is nil. The request ID stack sets the X-Request-Id response
// header, bridges the ID into the logger context (RequestContext), and echoes
// a valid caller-supplied X-Client-Request-Id header back on the response and
// into the envelope's client_request_id field (ClientRequestID). The access log
// level is status-aware (5xx→error, 4xx→warn, else info) and skips /health.
//
// # Kind→status defaults
//
// Handlers return public catalog errors; the Router resolves an HTTP status
// per returned error in three tiers: an explicit per-route/group Status or
// Statuses mapping wins; otherwise the error's Kind is looked up in the
// kind→status table (Config.KindStatuses, defaulting to DefaultKindStatuses:
// validation→422, not_found→404, conflict→409, unauthorized→401, forbidden→403,
// rate_limit→429, internal→500); an unknown kind falls back to
// Config.DefaultStatus. When several errors are returned together the highest
// resolved status is used. Because kinds carry sensible defaults, most routes
// need no explicit Status options at all — add them only to override.
//
// # Operational endpoints and defaults
//
// New auto-registers GET /health and GET /version. When Config.HealthCheck is
// set, /health returns a 200 "alive" envelope on nil error and a 503
// "unavailable" envelope on error; when nil it is always alive. /version
// returns Config.Version/BuildID/BuildTime. Fiber read/write timeouts and a
// body-size limit are applied by default and overridable via Config.FiberConfig.
//
// # Handler helpers
//
// ParamInt64 and ParamInt parse path parameters, returning the ErrInvalidParam
// sentinel (a plain error) on failure for handlers to map into their catalog.
//
// # Manifest
//
// Transport.Manifest() returns registration-time bookkeeping — every route with
// its resolved per-error HTTP status, the full public catalog, and the envelope
// version (EnvelopeVersion) — as pure data for future OpenAPI/TypeScript
// generation.
//
// # API responses
//
// Config.Catalog is the service's public error contract: the only errors
// allowed to cross the wire. Handlers return public catalog errors (mapping
// internal module errors at the boundary with errors.MapOr); the Router maps
// them to an HTTP status via Status/Statuses (or OnError) options and renders
// the RES envelope. Success responses and status-less error envelopes use
// HTTP 200. Unmapped public errors bubble to ErrorHandler and use
// Config.DefaultStatus.
//
//	// success (HTTP 200)
//	return http.RES(c).Message("user fetched").Data("user", user).Ok()
//	// error: return the public catalog entry; the route resolves the status
//	return errUserNotFound
//
// Status/Statuses options accept public catalog entries only; foreign entries
// panic at route registration. ErrorHandler backs unmatched routes
// (Fallbacks.NotFound), plain errors (Fallbacks.Unhandled), public errors no
// route mapped, and internal catalog errors — which are logged with their full
// chain but rendered as Fallbacks.Unhandled so implementation details never
// leak.
package http
