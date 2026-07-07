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
| `pkg/http` | Headless HTTP stack (Fiber v3), middleware, JSON RES envelope, and `runtime.Unit` |
| `pkg/http/openapi` | Declarative OpenAPI 3.2 engine: typed handlers, path-first registration, generated specs |
| `pkg/bus` | Inter-unit message bus (local today; same API for a future cluster backend) |
| `pkg/migrate` | SQL migrations with a multi-replica startup gate (`migrate.Postgres`) |

Packages are independent. You assemble them in your program — there is no kit or framework layer. Runtime does not own your dependencies: build them in `main` (open the pool, create the bus with `bus.NewLocal(bus.LocalConfig{})`, wire stores) and release them there with `defer`. Pass the assembled context to `runtime.New` and narrow it to what each unit needs at the factory. Bus delivery is bounded per subscription (FIFO worker + backpressure), subjects support `*`/`>` wildcards, and `Drain(ctx)` finishes queued work on graceful shutdown.

## Library usage

A minimal process owns a logger, registers an HTTP unit, and runs:

```go
func run() error {
    appLog := logger.New(&logger.Config{Name: "orders", ID: "orders-api"})
    defer appLog.Close() // root only; never Close log.Derive(...) children

    r, err := runtime.New(struct{}{}, appLog)
    if err != nil { return err }

    catalog := errors.NewCatalog("orders")
    r.Unit(func(_ struct{}, log runtime.Logger) runtime.Unit {
        t := http.New(http.Config{Logger: log.Derive("http"), Catalog: catalog})
        api := openapi.New(t, openapi.Config{
            Specs: []openapi.Spec{{Path: "/openapi.json", Title: "Orders API"}},
        })
        api.Routes("/api/v1", func(r *openapi.Router) {
            users := r.Group(openapi.GroupConfig{
                Prefix:    "/users",
                Responses: openapi.Returns().Err(422, errValidation),
            })
            users.Route("/:id").Get(openapi.Route{
                Handler:   getOrder,
                Summary:   "Fetch one order",
                Responses: openapi.Returns().Err(404, errNotFound),
            })
        })
        return t
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

Handlers **return public catalog errors**; declared routes resolve HTTP status from `Responses.Err(status, ...)`. Success is always **HTTP 200**.

The OpenAPI engine uses typed handlers:

```go
func getOrder(ctx context.Context, in getOrderIn) (orderOut, error) { ... }
```

Bind tags on the input struct (`path`, `query`, `header`, `cookie`, `json`, `form`) populate `in` before the handler runs. Bind failures return **422** using catalog entries declared under `Responses.Err(422, ...)` (merged from group + route). Handlers that anonymous-embed `openapi.Response` receive transport-owned envelope fields on success; other return types are marshaled as raw JSON.

Status resolution for handler errors:

1. an explicit `Responses.Err(status, catalogEntry)` on the route or group;
2. if undeclared or non-catalog, the error bubbles to the transport global handler.

When several catalog errors are returned together on a declared route, the **highest** declared status wins. Only public-catalog entries are accepted in `Responses.Err(...)`; foreign entries panic at registration.

```go
api.Routes("/api/v1", func(r *openapi.Router) {
    users := r.Group(openapi.GroupConfig{
        Prefix:    "/users",
        Responses: openapi.Returns().Err(422, api.ErrValidation),
    })
    users.Route("/:id").Get(openapi.Route{
        Handler:   getUser,
        Summary:   "Fetch one user",
        Responses: openapi.Returns().
            Err(404, api.ErrUserNotFound).
            Err(500, api.ErrFetchUser),
    })
    users.Route("/upload").Post(openapi.Route{
        Handler:   uploadAsset,
        Summary:   "Upload an asset",
        Requests: []openapi.ContentUnit{{
            Required:    true,
            ContentType: "multipart/form-data",
            Structure:   uploadIn{}, // or &openapi.Schema{Ref: "..."}
        }},
        Responses: openapi.Returns().Err(422, api.ErrValidation),
    })
})
```

Imperative Fiber handlers (streaming, health demos, legacy paths) register on `Transport.Fiber()` or `Router.Fiber()` and are never included in OpenAPI output.

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

## Manifest & OpenAPI

Attach the engine with `openapi.New(t, cfg)` and register routes with `api.Routes(prefix, ...)`. `Generator.Manifest()` returns registration-time bookkeeping — every route with operation metadata, named path parameters, declared errors (each with its resolved HTTP status), plus the full public catalog and envelope version — as pure data for contract tooling.

`Generator.Document(spec)` renders an OpenAPI 3.2 document from the manifest. The same code path powers the served spec endpoints. Declared errors become responses under their resolved statuses (described from the catalog, machine-readable under `x-xqua-errors`), and the RES envelope is modeled once as component schemas (`Envelope`, `ErrorEnvelope`, `ErrorDetail`, `Pagination`, `Cursor`).

`openapi.Config.Specs` declares the documents the engine serves; each is auto-registered as a GET endpoint. A `nil` slice serves one document at `/openapi.json`; an empty (non-nil) slice serves none. Each `openapi.Spec` filters the manifest by path `Prefix` (segment-aware) and membership tags (`Route.Specs`), so one transport can publish several surfaces at once:

```go
t := http.New(http.Config{Logger: log, Catalog: apiCatalog})
api := openapi.New(t, openapi.Config{
    Specs: []openapi.Spec{
        {Path: "/openapi.json", Prefix: "/api/v1", Title: "Public API"},
        {Path: "/mobile/openapi.json", Prefix: "/mobile", Title: "Mobile API"},
        {Path: "/console/openapi.json", Prefix: "/console", Title: "Console API"},
    },
})
```

Request/response schemas are inferred from handler `In`/`Out` types (struct tags) or declared explicitly on `Route.Request` / `Route.Requests` / `Returns(T{})`. Multipart uploads bind at runtime when input fields use `form` tags with `*multipart.FileHeader`, `[]*multipart.FileHeader`, or `multipart.File`.

OpenAPI **3.2** extras:

- **Streaming** — `Route.Extra` with `ResponseDecl.ItemBody` for SSE/WebSocket docs; live streaming uses `Router.Fiber()`.
- **QUERY** — `Route(path).Query(...)` registers the HTTP QUERY method.
- **Security** — `Config.Schemes` registers Bearer, API key, HTTP, OAuth2, OIDC, and mTLS schemes; `Verify` hooks enforce them and attach `Identity` (`any`) to context. Use `RequireSecurity`, `RequireAnySecurity`, or `PublicSecurity` on routes/groups.
- **Multipart** — `Route.Requests` with `ContentType: "multipart/form-data"` and optional `Encoding` per part.

Secured route example:

```go
api := openapi.New(t, openapi.Config{
    Schemes: map[string]openapi.Scheme{
        "BearerAuth": openapi.BearerScheme(openapi.BearerOptions{Verify: verifyToken}),
        "ApiKey": openapi.APIKeyScheme(openapi.APIKeyOptions{
            Name: "X-API-Token", In: openapi.InHeader, Verify: verifyAPIKey,
        }),
    },
})
api.Routes("/api/v1", func(r *openapi.Router) {
    r.Route("/auth/login").Post(openapi.Route{
        Handler: login, Security: openapi.PublicSecurity(),
        Responses: openapi.Returns().Err(401, errUnauthorized),
    })
    r.Route("/me").Get(openapi.Route{
        Handler: me,
        Security: openapi.RequireAnySecurity("BearerAuth", "ApiKey"),
        Responses: openapi.Returns().Err(401, errUnauthorized),
    })
})
// In handlers: user, ok := openapi.IdentityAs[User](ctx)
```

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

Guided platter: [`examples/README.md`](examples/README.md) — start with `hello`, then `showcase` for the full HTTP/OpenAPI surface. Full showcase curl recipes: [`examples/showcase/TESTING.md`](examples/showcase/TESTING.md).

```bash
go run ./examples/hello
make dev-up && go run ./examples/showcase
go run ./examples/multiport
go run ./examples/bus
go run ./examples/split
go run ./examples/logging
```

| Example | Shows |
|---------|--------|
| `hello` | Minimal runtime + typed handler + `/openapi.json` |
| `showcase` | Postgres users, demo routes, multipart, multi-surface OpenAPI, catalog discipline ([`TESTING.md`](examples/showcase/TESTING.md)) |
| `multiport` | Two HTTP units on `:8080` and `:8081` in one process |
| `bus` | Local bus with competing queue workers |
| `split` | HTTP compute unit + storage unit over bus only |
| `logging` | Structured error logging without HTTP |

## Feedback-driven development

Adapt an example to the API you want, then we evolve the library to match.
