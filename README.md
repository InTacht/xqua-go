# xqua-go

Go SDK for building high-performance, container-ready services.

The library is **stateless and config-first**: it does not read environment variables or assume deployment defaults. Your service program loads configuration, assembles dependencies, and passes them in.

## Packages

| Package | Role |
|---------|------|
| `pkg/runtime` | Headless process supervisor: `Unit`, lifecycle hooks, graceful shutdown |
| `pkg/logger` | Structured zap logging with canonical error fields |
| `pkg/errors` | Catalog-driven canonical errors, wrapping, and conversion |
| `pkg/env` | Typed environment lookups (`String`/`Int`/`Bool`/`MustString`) and struct `Bind` for program `main` |
| `pkg/http` | Headless HTTP stack (Fiber v3), middleware, JSON API envelope; implements `runtime.Unit` |
| `pkg/bus` | Inter-unit message bus (local today; same API for a future cluster backend) |
| `pkg/migrate` | SQL migrations with a multi-replica startup gate (`migrate.Postgres`) |

Packages are independent. You assemble them in your program — there is no kit or framework layer. Runtime does not own your dependencies: build them in `main` (open the pool, create the bus with `bus.NewLocal(bus.LocalConfig{})`, wire stores) and release them there with `defer`. Pass the assembled context to `runtime.New` and narrow it to what each unit needs at the factory. Bus delivery is bounded per subscription (FIFO worker + backpressure), subjects support `*`/`>` wildcards, and `Drain(ctx)` finishes queued work on graceful shutdown.

## Library usage

A minimal process owns a logger, registers an HTTP unit, and runs:

```go
func run() error {
    appLog := logger.New(&logger.Config{Name: "orders", ID: "orders-api"})
    defer appLog.Close() // root only; never Close log.Derive(...) children

    // No shared deps yet — pass struct{}{}. See examples/api for a real context.
    r, err := runtime.New(struct{}{}, appLog)
    if err != nil { return err }

    catalog := errors.NewCatalog("orders")
    r.Unit(func(_ struct{}, log runtime.Logger) runtime.Unit {
        return http.New(http.Config{Logger: log.Derive("http"), Catalog: catalog}).
            Routes("/api/v1", func(r *http.Router) {
                r.Get("/orders/:id", getOrder) // kind→status defaults apply
            })
    })
    return r.Run()
}
```

See [`examples/README.md`](examples/README.md) for the full guided platter.

## Errors

Errors are defined upfront in a catalog, one per module or logical space. **Identity is the defined entry itself** — `errors.Is` matches only errors originating from the same `Define` call, so entries from different catalogs can never collide, even with equal kinds or codes. There is no global registry or process state.

**Kind** is a semantic category (`validation`, `not_found`, …) shared freely across catalogs and matched with `errors.IsKind`; it defaults to the catalog name. The standard kinds are exported as constants — `errors.KindValidation`, `KindNotFound`, `KindConflict`, `KindUnauthorized`, `KindForbidden`, `KindRateLimit`, `KindInternal` — and let the HTTP transport resolve a default status from the kind alone. **Code** is freeform and unique within its catalog. The `errors` package is transport-agnostic: **HTTP status lives at the route layer, not on the error** (see [API responses](#api-responses)).

```go
// One catalog per module/space.
var Store = errors.NewCatalog("store")

var (
    ErrUserMissing = Store.Define(errors.Def{
        Kind: "not_found", Code: "10001", Message: "user not found", Source: "id",
    })
    ErrConflict = Store.Define(errors.Def{Code: "10002", Message: "stale version"})
)

// Wrap driver errors while keeping the catalog shape
if err := repo.FindUser(ctx, id); err != nil {
    return errors.Wrap(err, ErrUserMissing)
}

// Plain errors are for underlying causes only — map before surfacing.
return errors.MapOr(err, ErrConflict, func(err error) (*errors.Error, bool) {
    if errors.Is(err, sql.ErrNoRows) {
        return ErrUserMissing, true
    }
    return nil, false
})

// Re-map one catalog entry to another declaratively with Pair/Mappers.
return errors.MapOr(err, api.ErrInternal,
    errors.Pair(store.ErrUserMissing, api.ErrUserNotFound),
    errors.Pair(store.ErrConflict, api.ErrConflict),
)

// Exact matching: the specific catalog entry, anywhere in the chain
if errors.Is(err, ErrUserMissing) { ... }

// Categorical matching: any error of this kind, from any catalog
if errors.IsKind(err, "validation") { ... }

// Log structured errors (errors array for catalog, error string for plain)
log.Error(err, "fetch user failed")
```

`Catalog.Lookup(code)` decodes wire errors back to catalog identity (service-to-service clients), and `Catalog.Entries()` enumerates a catalog for OpenAPI/TypeScript client generation.

## API responses

`Config.Catalog` declares the service's **public error contract**: the only errors allowed to cross the wire. Internal catalog errors (e.g. from a `store` package) must be mapped into it at the boundary with `errors.MapOr` — this forces you to be deterministic about what your service handles and what it resurfaces. If an internal error slips through, the transport replaces it with `Fallbacks.Unhandled` (logged with the full chain, never leaked).

Handlers **return public catalog errors**; the route resolves the HTTP status. Success responses (and bare error envelopes) use **HTTP 200**.

Status resolution has three tiers (highest priority first):

1. an explicit per-route/group `http.Status` / `http.Statuses` mapping;
2. the **kind→status table** — `Config.KindStatuses`, defaulting to `http.DefaultKindStatuses()`: `validation`→422, `not_found`→404, `conflict`→409, `unauthorized`→401, `forbidden`→403, `rate_limit`→429, `internal`→500;
3. `Config.DefaultStatus` for unknown kinds.

Because kinds carry sensible defaults, **most routes need no explicit `Status` options** — add them only to override.

```go
Routes("/api/v1", func(r *http.Router) {
    // No status options needed: errUserNotFound (kind not_found) → 404,
    // errIDRequired (kind validation) → 422, all from the kind table.
    r.Get("/users/:id", getUser)

    // Override a kind default only when you need to.
    r.Post("/users", createUser, http.Status(api.ErrConflict, fiber.StatusConflict))
})

func getUser(c fiber.Ctx) error {
    id, err := http.ParamInt64(c, "id") // sentinel http.ErrInvalidParam on failure
    if err != nil {
        return api.ErrIDRequired // kind validation → 422
    }
    user, err := svc.FetchUser(c.Context(), id)
    if err != nil {
        // boundary: internal store errors → public api errors
        return errors.MapOr(err, api.ErrInternal,
            errors.Pair(store.ErrUserMissing, api.ErrUserNotFound), // → 404
        )
    }
    return http.RES(c).Message("ok").Data("user", user).Ok() // 200
}
```

Only public-catalog entries are accepted in `Status`/`Statuses`; foreign entries panic at startup. When several catalog errors are returned together, the route uses the **highest** resolved status (e.g. `500 > 422`). For full control, `http.OnError(func(c, err) error)` handles the error itself. Anything that is not a public-catalog error (plain errors, internal catalog errors) bubbles to the global handler.

## HTTP middleware

Default middleware (recover, request ID, access log) is enabled when `Middleware` is nil. The access log level is **status-aware** — 5xx logs at error, 4xx at warn, everything else at info — and the noisy `/health` endpoint is skipped. `pprof` stays opt-in.

Request correlation works on both headers and the envelope:

- **`X-Request-Id`** — the server request ID, generated per request (or honored from the incoming header). Returned as a response header and as `request_id` in the envelope, and attached to every log line.
- **`X-Client-Request-Id`** — an optional caller-supplied correlation ID. When present (visible ASCII, ≤128 chars), it is echoed back as a response header and included as `client_request_id` in the envelope; invalid values are dropped, never reflected.

The global error handler backs four cases with required config: unmatched routes render `Fallbacks.NotFound` (HTTP 404); plain errors render `Fallbacks.Unhandled` with `DefaultStatus`; public-catalog errors no route mapped are logged and rendered with `DefaultStatus`; internal catalog errors are logged with their full chain but render only `Fallbacks.Unhandled` — implementation details never leak.

## Operational endpoints & defaults

- **`GET /health`** is registered automatically. Set `Config.HealthCheck func(ctx) error` to gate it: a nil error is a 200 `alive` envelope, an error is a 503 `unavailable` envelope. With no `HealthCheck`, `/health` is always alive.
- **`GET /version`** returns `Config.Version` / `BuildID` / `BuildTime` (empty fields omitted).
- Sensible Fiber **read/write timeouts and a body-size limit** are applied by default and overridable via `Config.FiberConfig`.

## Manifest

`Transport.Manifest()` returns registration-time bookkeeping — every route with its resolved per-error HTTP status, the full public catalog, and the envelope version (`http.EnvelopeVersion`) — as pure data for future OpenAPI/TypeScript generation.

## Configuration from the environment

`pkg/env` provides typed lookups for program `main` packages (`env.String`, `env.Int`, `env.Bool`, `env.MustString`). Or declare a tagged config struct and populate it in one call with `env.Bind`, which reports every missing or invalid variable together:

```go
type Config struct {
    Host        string        `env:"APP_HOST" default:"0.0.0.0"`
    Port        int           `env:"APP_PORT" default:"8080"`
    Shutdown    time.Duration `env:"SHUTDOWN_TIMEOUT" default:"30s"`
    DatabaseURL string        `env:"DATABASE_URL,required"`
}

var cfg Config
if err := env.Bind(&cfg); err != nil { log.Fatal(err) }
```

The library core stays config-first and never reads the environment itself.

## Units

Implement `runtime.Unit` to add long-lived supervisees (HTTP, gRPC, jobs, agents, etc.):

```go
type Unit interface {
    Name() string
    Serve(opts ServeOptions) error
    Shutdown(ctx context.Context) error
}
```

Register a factory with `runtime.Runtime.Unit`. The runtime orchestrates hooks and shutdown; it does not know about HTTP or Fiber directly.

## Dependencies

Runtime carries an application context of any type — a plain struct holding your shared dependencies — and passes it to unit factories. There is no `Build`/`Destroy` contract: you construct dependencies in `main` and release them with `defer` after each successful open. Defers run LIFO, so a failure partway through construction still releases what was opened.

```go
func run() error {
    pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
    if err != nil { return err }
    defer pool.Close()

    app := &App{Pool: pool, Users: store.NewUsers(pool)}
    r, err := runtime.New(app, log)
    if err != nil { return err }

    // Narrow the context at the factory so units never import your App type.
    r.Unit(func(a *App, log runtime.Logger) runtime.Unit {
        return worker.New(a.Bus, log.Derive("worker"))
    })
    return r.Run()
}
```

Keep construction and `defer` inside a `run() error` function and return errors to `main` — `os.Exit`/`log.Fatal` skip deferred calls. Keep application context fields read-mostly and concurrent-safe: units run in their own goroutines and share the one context.

## Examples

Guided platter (read in order): [`examples/README.md`](examples/README.md).

```bash
go run ./examples/hello
go run ./examples/catalog
go run ./examples/multiport
go run ./examples/bus
go run ./examples/split
make dev-up && go run ./examples/api
go run ./examples/logging
```

| Example | Shows |
|---------|--------|
| `hello` | Minimal runtime + one HTTP unit, catalog fallbacks, RES envelope |
| `catalog` | Public catalog, kind→status, internal errors never leak, `Pair` / validation collections |
| `multiport` | Several units in one process (public `:8080` + admin `:8081`) |
| `bus` | Local bus: HTTP request/replies to competing worker units on `demo.work` |
| `split` | Compute (HTTP) and storage as separate units; talk only over the bus |
| `api` | Postgres, SQL migrations, hooks, store boundary mapping |
| `logging` | Structured error logging alone (no HTTP) |

## Feedback-driven development

Adapt an example to the API you want, then we evolve the library to match.
