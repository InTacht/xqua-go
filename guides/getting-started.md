# Getting started

Build a minimal HTTP API with typed handlers, a public error catalog, and a served OpenAPI document.

## Prerequisites

- Go 1.26.4+

## Install

```bash
go get github.com/InTacht/xqua-go@latest
```

## Run the smallest example

```bash
go run ./examples/hello
```

Then open `http://127.0.0.1:8080/openapi.json` and call `GET /api/v1/ping`.

## Build your own service

### 1. Logger and runtime

Your `main` owns the logger lifecycle and process supervisor:

```go
appLog := logger.New(&logger.Config{Name: "orders", ID: "orders-api"})
defer appLog.Close() // root only — never Close derived children

r, err := runtime.New(struct{}{}, appLog)
if err != nil {
    return err
}
```

### 2. Public error catalog

Define **one catalog per service** — the only errors clients may see:

```go
var API = errors.NewCatalog("orders")

var (
    errValidation = API.Define(errors.Def{
        Kind: errors.KindValidation, Code: "10001", Message: "validation failed",
    })
    errNotFound = API.Define(errors.Def{
        Kind: errors.KindNotFound, Code: "10002", Message: "order not found",
    })
)
```

Keep store/repo catalogs private; map into `API` at the boundary. See [Errors](errors.md).

### 3. HTTP transport + OpenAPI

```go
r.Unit(func(_ struct{}, log runtime.Logger) runtime.Unit {
    t := http.New(http.Config{
        Logger:  log.Derive("http"),
        Catalog: API,
        Version: "1.0.0",
    })
    api := openapi.New(t, openapi.Config{
        Specs: []openapi.Spec{{Path: "/openapi.json", Title: "Orders API"}},
    })
    api.Routes("/api/v1", func(r *openapi.Router) {
        orders := r.Group(openapi.GroupConfig{
            Responses: openapi.Returns().Err(422, errValidation),
        })
        orders.Route("/:id").Get(openapi.Route{
            Handler:   getOrder,
            Summary:   "Fetch one order",
            Responses: openapi.Returns().Err(404, errNotFound),
        })
    })
    return t
})
return r.Run()
```

### 4. Typed handler

```go
type getOrderIn struct {
    ID int64 `path:"id"`
}

type orderOut struct {
    openapi.Response
    Data struct {
        ID   int64  `json:"id"`
        Name string `json:"name"`
    } `json:"data"`
}

func getOrder(ctx context.Context, in getOrderIn) (orderOut, error) {
    order, err := store.Find(ctx, in.ID)
    if err != nil {
        return orderOut{}, errors.MapOr(err, errNotFound,
            errors.Pair(store.ErrMissing, errNotFound),
        )
    }
    var out orderOut
    out.Message = "order fetched"
    out.Data.ID = order.ID
    out.Data.Name = order.Name
    return out, nil
}
```

Tags on `in` (`path`, `query`, `header`, `cookie`, `json`, `form`) are filled before your handler runs.

### 5. Configuration

Bind env in `main` — library packages never read the environment:

```go
type Config struct {
    Host string `env:"APP_HOST" default:"0.0.0.0"`
    Port int    `env:"APP_PORT" default:"8080"`
}
var cfg Config
if err := env.Bind(&cfg); err != nil {
    log.Fatal(err)
}
```

See [Configuration](configuration.md).

## How it fits together

```text
main → open deps (defer Close) → logger.New → runtime.New → r.Unit(...) → r.Run()
```

- **One public catalog** per HTTP service; map store errors with `errors.MapOr` / `errors.Pair`
- **OpenAPI routes**: `func(ctx, in) (out, error)` — guard → AfterAuth → bind → handler
- **Escape hatch**: `t.Fiber()` for streaming/WebSockets (not auto-documented)
- **Config-first**: `pkg/env` only in `main`; library packages take explicit `Config`

Request flow and envelopes: [HTTP](http.md). Several units or bus: [Multi-unit apps](multi-unit-apps.md).

## What to read next

| You want to… | Read |
| ------------ | ---- |
| Declare more routes, auth, multipart, several OpenAPI docs | [Building APIs](openapi.md) |
| Idempotency, rate limits, distinct 401 codes | [Common patterns](patterns.md) |
| Postgres + migrations at startup | [Database migrations](database.md) |
| HTTP worker + API in one binary | [Multi-unit apps](multi-unit-apps.md) |
| See everything in one app | [showcase](../examples/showcase) |
