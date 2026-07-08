# Configuration

How to load settings and log from your service `main`. Library packages take explicit config — they never read `os.Getenv` themselves.

## Environment variables

### One struct for your app

```go
type Config struct {
    Host        string        `env:"APP_HOST" default:"0.0.0.0"`
    Port        int           `env:"APP_PORT" default:"8080"`
    Shutdown    time.Duration `env:"SHUTDOWN_TIMEOUT" default:"30s"`
    DatabaseURL string        `env:"DATABASE_URL,required"`
}

func run() error {
    var cfg Config
    if err := env.Bind(&cfg); err != nil {
        return err // all missing/invalid fields reported together
    }
    // use cfg.Host, cfg.Port, …
}
```

Tags: `required`, `default:"value"`.

### Single values

```go
host := env.String("APP_HOST", "0.0.0.0")
port := env.MustInt("APP_PORT")
```

Use `Must*` only when a missing variable should stop startup immediately.

## Logging

### Root logger in main

```go
appLog := logger.New(&logger.Config{
    Name:  "orders",
    ID:    "orders-api",  // instance id — useful when running several replicas
    Label: "server",
    Debug: false,
})
defer appLog.Close()
```

Only the root logger from `New` gets `Close`. Derived loggers share the backend.

### Per-component labels

In each `runtime.Unit` factory:

```go
r.Unit(func(app *App, log runtime.Logger) runtime.Unit {
    httpLog := log.Derive("http")
    return http.New(http.Config{Logger: httpLog, Catalog: apiCatalog, ...})
})
```

Logs show `server.http` (parent.child). Never `Close` a derived logger.

### Request correlation

HTTP middleware sets `X-Request-Id` and attaches it to the logger context. Use context methods in handlers:

```go
log.InfoCtx(ctx, "order created", "order_id", id)
```

Optional client id: send `X-Client-Request-Id`; valid values echo in the response and envelope as `client_request_id`.

### Logging errors

Return catalog errors from handlers; log at the boundary when needed:

```go
if err != nil {
    log.ErrorWrap(err, "fetch order failed")
    return orderOut{}, errNotFound
}
```

Catalog errors log as structured `errors` arrays (`kind`, `code`, `message`). Plain errors log as `error` string.

## Wiring into HTTP

```go
http.New(http.Config{
    Logger:    log.Derive("http"),
    Catalog:   apiCatalog,
    Version:   cfg.Version,
    Host:      cfg.Host,
    Port:      cfg.Port,
    HealthCheck: pool.Ping,
})
```

`Version` appears on `GET /version`. `HealthCheck` gates `GET /health`.
