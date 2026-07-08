# HTTP transport

`pkg/http` is a headless HTTP stack on Fiber v3. It owns middleware, operational endpoints, the RES JSON envelope, and a global error handler bound to the service's **public** error catalog.

It deliberately does **not** know about REST routing or OpenAPI — attach `pkg/http/openapi` via `openapi.New(t, cfg)`.

## Create a transport

```go
t := http.New(http.Config{
    Logger:    log.Derive("http"),
    Catalog:   apiCatalog,           // required — public wire contract
    Fallbacks: http.StandardErrors(apiCatalog), // default when zero
    Version:   "1.2.3",
    Host:      "0.0.0.0",
    Port:      8080,
    HealthCheck: func(ctx context.Context) error { return pool.Ping(ctx) },
})
```

`Transport` implements `runtime.Unit` (`Serve`, `Shutdown`, `Name`).

Invalid config panics at creation time (fail fast).

## RES envelope

Success and error responses use a consistent JSON shape.

**Success** (always HTTP 200 for OpenAPI routes):

```json
{
  "status": "success",
  "message": "user fetched",
  "request_id": "...",
  "client_request_id": "...",
  "data": { }
}
```

**Error**:

```json
{
  "status": "error",
  "request_id": "...",
  "errors": [
    {"kind": "not_found", "code": "10002", "message": "user not found"}
  ]
}
```

Handlers that anonymous-embed `openapi.Response` receive transport-owned envelope fields (`status`, `request_id`, …) on success. Other return types are marshaled as raw JSON (escape hatch).

Imperative handlers use `http.RES(c)`:

```go
return http.RES(c).Message("ok").Data("user", user).Ok()
```

## Middleware

When `Config.Middleware` is nil, defaults install:

| Middleware | Behavior |
| ---------- | -------- |
| Recover | Panic recovery |
| Request ID | `X-Request-Id` generation/propagation |
| Access log | Status-aware: 5xx→error, 4xx→warn, else info; skips `/health` |

### Correlation headers

| Header | Role |
| ------ | ---- |
| `X-Request-Id` | Server request ID — response header + `request_id` in envelope + all log lines |
| `X-Client-Request-Id` | Optional caller correlation — echoed when valid (visible ASCII, ≤128 chars) |

## Operational endpoints

| Endpoint | Behavior |
| -------- | -------- |
| `GET /health` | 200 `alive` when `HealthCheck` is nil or returns nil; 503 `unavailable` on error |
| `GET /version` | `Version`, `BuildID`, `BuildTime` from config |

Fiber read/write timeouts and body-size limits are applied by default (override via `Config.FiberConfig`).

## Global error handler

The transport is the safety net for errors engines did not map:

| Case | Response |
| ---- | -------- |
| Unmatched route | `Fallbacks.NotFound` (404) |
| Plain / stdlib error | `Fallbacks.Unhandled` |
| Public catalog error, no route mapping | `Fallbacks.Unhandled` with `DefaultStatus` |
| Internal catalog error | Logged with full chain; rendered as `Fallbacks.Unhandled` |

Per-route status mapping is an **engine** concern (`openapi.Responses.Err`).

## Extension point: Fiber()

```go
t.Fiber().Get("/custom", customHandler)
```

Routes registered directly on Fiber are not OpenAPI-documented unless added via `Router.Describe`.

## Standard fallbacks

`http.StandardErrors(catalog)` defines conventional `Unhandled` and `NotFound` entries. Applied automatically when `Fallbacks` is zero.

## Further reading

- [OpenAPI engine](openapi.md) — typed handlers, binding, security
- [Errors](errors.md) — catalog discipline
- `go doc github.com/InTacht/xqua-go/pkg/http`
