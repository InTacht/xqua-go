// Package http is a headless, protocol-neutral HTTP transport built on Fiber v3.
// It owns the Fiber app, middleware, operational endpoints, the RES response
// envelope, and one global error handler bound to the service's public error
// catalog. It deliberately does not know about REST routing or OpenAPI: those
// live in the pkg/http/openapi engine, which attaches to a transport via
// openapi.New(t, ...). Future engines (websocket, graphql, socket.io, ...)
// follow the same attachment pattern, each layering its protocol on the shared
// transport while reusing the same catalog contract.
//
// Wire your catalog and config yourself, then plug the resulting Transport into
// runtime (or any supervisor) via the runtime.Unit interface. Handlers use
// http.Ctx and http.Handler (aliases of Fiber's types) so application code need
// not import Fiber for handler declarations.
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
//	    t := http.New(http.Config{
//	        Logger:  log.Derive("http"),
//	        Catalog: apiCatalog, // public error contract; the rest defaults
//	    })
//	    // Attach the OpenAPI engine for declarative REST + generated docs.
//	    openapi.New(t, openapi.Config{}).Routes("/api/v1", func(r *openapi.Router) {
//	        r.Route("/users/:id").Get(openapi.Route{
//	            Handler:   getUser,
//	            Summary:   "Fetch one user",
//	            Responses: openapi.Returns().Err(404, errNotFound),
//	        })
//	    })
//	    return t
//	})
//
// # Extension points
//
// Fiber() exposes the underlying *fiber.App — the single extension point.
// First-party engines and custom handlers register on it directly:
//
//	t.Fiber().Get("/custom", customFiberHandler)
//
// A route registered directly on Fiber is not documented unless it is added
// through an engine documentation hook (e.g. openapi Router.Describe).
//
// Engines read the transport through accessors: Catalog() (the one public wire
// contract), Logger(), Version() (default document version), and
// DefaultStatus() (status fallback for unmapped errors). StandardErrors(catalog)
// defines the conventional Unhandled / NotFound fallbacks; New applies it
// automatically when Fallbacks is left zero.
//
// # Middleware
//
// Default middleware installs recover, request ID, and access log when
// Middleware is nil. The request ID stack sets the X-Request-Id response
// header, bridges the ID into the logger context (RequestContext), and echoes
// a valid caller-supplied X-Client-Request-Id header back on the response and
// into the envelope's client_request_id field (ClientRequestID). The access log
// level is status-aware (5xx→error, 4xx→warn, else info) and skips /health.
//
// # Operational endpoints and defaults
//
// New auto-registers GET /health and GET /version. When Config.HealthCheck is
// set, /health returns a 200 "alive" envelope on nil error and a 503
// "unavailable" envelope on error; when nil it is always alive. /version
// returns Config.Version/BuildID/BuildTime. Fiber read/write timeouts and a
// body-size limit are applied by default and overridable via Config.FiberConfig.
// The transport always registers the HTTP QUERY method (MethodQuery) with Fiber
// so the openapi engine's Router.Query works.
//
// # Handler helpers
//
// ParamInt64 and ParamInt parse path parameters, returning the ErrInvalidParam
// sentinel (a plain error) on failure for handlers to map into their catalog.
//
// # API responses and the global error handler
//
// Config.Catalog is the service's public error contract: the only errors
// allowed to cross the wire. Handlers build responses with RES; success and
// status-less error envelopes use HTTP 200.
//
//	// success (HTTP 200)
//	return http.RES(c).Message("user fetched").Data("user", user).Ok()
//
// ErrorHandler is the global safety net: it backs unmatched routes
// (Fallbacks.NotFound), plain errors (Fallbacks.Unhandled), public catalog
// errors no engine mapped (rendered with Config.DefaultStatus), and internal
// catalog errors — which are logged with their full chain but rendered as
// Fallbacks.Unhandled so implementation details never leak. Per-error status
// mapping and protocol documentation are engine concerns, not the transport's.
package http
