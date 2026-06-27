// Package http provides a Fiber v3 HTTP transport and JSON API response helpers.
//
// # Transport
//
// New creates a Transport from Config (Host, Port, Logger, FiberConfig). Host,
// Port, and Logger are required; invalid config panics at creation time.
//
// Use Fiber() for direct route registration or Routes(prefix, fn) for grouped
// routes. Serve listens on host:port; Shutdown stops the server gracefully.
//
//	tr := http.New(http.Config{
//	    Host:        "0.0.0.0",
//	    Port:        8080,
//	    Logger:      log,
//	    FiberConfig: fiber.Config{ServerHeader: "orders"},
//	}).Routes("/api/v1", func(r fiber.Router) {
//	    r.Get("/health", healthHandler)
//	})
//
// Register the factory with service.Service.Transport:
//
//	srv.Transport(func(app *Ctx, log *logger.Logger) transport.Transport {
//	    return http.New(http.Config{...}).Routes("/api/v1", registerRoutes)
//	})
//
// RequestContext is a Fiber middleware that copies request IDs into the logger
// context when requestid middleware is installed.
//
// # API responses
//
// All endpoints use HTTP 200 with outcome in the JSON body. Build responses
// with RES:
//
//	return http.RES(c).
//	    Message("user fetched").
//	    Data("user", user).
//	    Ok()
//
//	return http.RES(c).Message("not found").Error(errUserNotFound).Ok()
//	return http.RES(c).Message("validation failed").Apply(err).Ok()
//
// ErrorDetail includes kind, code, message, source, and cause (when wrapped,
// immediate cause only — same model as pkg/logger error fields). Pagination and
// cursor metadata are supported via Paginator and Cursor.
package http
