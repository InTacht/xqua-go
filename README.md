# xqua-go

Go SDK for building high-performance, container-ready services.

The library is **stateless and config-first**: it does not read environment variables or assume deployment defaults. Your service program loads configuration, assembles dependencies, and passes them in.

## Packages

| Package | Role |
|---------|------|
| `pkg/ctx` | Application context contract (`Build` / `Destroy`) |
| `pkg/logger` | Structured zap logging with canonical error fields |
| `pkg/errors` | Canonical error shape, wrapping, and conversion |
| `pkg/service` | Service runtime with lifecycle hooks |
| `pkg/transport` | Transport lifecycle interface and factory type |
| `pkg/transport/http` | HTTP transport (Fiber v3) and JSON API envelope |

## Library usage

A minimal service wires an application context, a transport factory, and `Run`:

```go
// app context — implement pkg/ctx.Ctx
type Ctx struct { /* db, clients, config */ }

func (c *Ctx) Build(ctx context.Context) error { /* open deps */ return nil }
func (c *Ctx) Destroy(ctx context.Context) error { /* close deps */ return nil }

// transport factory — receives app context + service logger
func HTTP(c *Ctx, log *logger.Logger) transport.Transport {
    return http.New(http.Config{
        Host:        "0.0.0.0",
        Port:        8080,
        Logger:      log,
        FiberConfig: fiber.Config{ServerHeader: "orders"},
    }).Routes("/", func(r fiber.Router) {
        r.Get("/health", func(c fiber.Ctx) error {
            return http.RES(c).Message("ok").Data("status", "ok").Ok()
        })
    })
}

// main
srv := service.New(service.Config{
    Name: "orders",
    ID:   "orders-api",
}, &Ctx{}).Transport(HTTP)

srv.Run()
```

See `examples/hello` for the full layout (`main.go`, `ctx/`, `transport/`).

## Errors

Use one canonical error shape across business logic, logging, and HTTP responses. Define kinds and codes in your application; the library enforces shape only.

```go
import (
    "github.com/InTacht/xqua-go/pkg/errors"
    "github.com/InTacht/xqua-go/pkg/transport/http"
)

// Catalog errors once in your service
var errUserNotFound = errors.New(
    "not_found", "404301", "user not found", "params.id",
)

// Wrap lower-level errors while keeping the canonical shape
if err := repo.FindUser(ctx, id); err != nil {
    return errors.Wrap(err, errUserNotFound)
}

// Ad-hoc plain errors (no stdlib errors import needed)
return errors.NewPlain("temporary failure")

// Map known errors, otherwise fall back
return errors.MapOr(err, "internal", "500000", "unexpected error", func(err error) (*errors.Error, bool) {
    if errors.Is(err, sql.ErrNoRows) {
        return errUserNotFound, true
    }
    return nil, false
})

// Match catalog errors anywhere in the chain (kind + code)
if errors.Is(err, errUserNotFound) { ... }

// Log structured errors (errors array for catalog, error string for plain)
log.Error(err, "fetch user failed")

// Return in API — shape flows through automatically
return http.RES(c).Message("not found").Error(err).Ok()
```

## API responses

All endpoints return **HTTP 200** with outcome in the JSON body. Build responses with `RES`:

```go
var errExternalIDRequired = errors.New(
    "validation", "422301", "external_id is required", "body.external_id",
)

return http.RES(c).
    Message("Subscriber upserted").
    Data("subscriber", subscriber).
    Ok()

return http.RES(c).Error(errExternalIDRequired).Ok()
return http.RES(c).Message("validation failed").Error(errExternalIDRequired).Ok()
return http.RES(c).Message("internal error").Apply(err).Ok()
```

Wrapped errors include `cause` on each error entry (immediate cause only, same as logger fields).

## Transports

Implement `transport.Transport` to add new protocols (gRPC, NATS, etc.):

```go
type Transport interface {
    Name() string
    Serve(opts ServeOptions) error
    Shutdown(ctx context.Context) error
}
```

Register a factory with `service.Service.Transport`. The service orchestrates hooks and shutdown; it does not know about HTTP or Fiber directly.

## Examples

Each example is standalone and shows full dependency assembly:

```bash
go run ./examples/hello
go run ./examples/api
go run ./examples/logging
```

| Example | Shows |
|---------|--------|
| `hello` | Minimal service, app context, HTTP transport factory |
| `api` | Env config, lifecycle hooks, MapOr, response envelope, logging |
| `logging` | Structured error logging scenarios |

## Feedback-driven development

Adapt an example to the API you want, then we evolve the library to match.
