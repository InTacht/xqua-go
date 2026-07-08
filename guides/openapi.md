# OpenAPI engine

`pkg/http/openapi` is the declarative REST engine for xqua-go. Routes are plain Go structs registered through path-first builders and described as one or more **OpenAPI 3.2** documents served from the same declarations.

## Attach the engine

```go
t := http.New(http.Config{Logger: log, Catalog: catalog, Version: "1.2.3"})
api := openapi.New(t, openapi.Config{
    Specs: []openapi.Spec{{Path: "/openapi.json", Title: "API", Version: "1.2.3"}},
    Schemes: map[string]openapi.Scheme{ /* security schemes */ },
})
api.Routes("/api/v1", func(r *openapi.Router) {
    // register routes
})
```

### Specs tri-state

| `Config.Specs` | Behavior |
| -------------- | -------- |
| `nil` | Serve one document at `/openapi.json` |
| `[]openapi.Spec{}` (empty slice) | Do not serve a document endpoint (generation still works via `api.Document`) |
| Non-empty slice | Serve exactly those documents |

## Typed handlers

Single handler shape:

```go
func(ctx context.Context, in In) (Out, error)
```

| Phase | What happens |
| ----- | ------------ |
| Guard | Security schemes verified (if declared) |
| AfterAuth | Group middleware chain (rate limits, etc.) |
| Bind | `in` populated from struct tags |
| Handler | Your business logic |
| Write | Success envelope or declared error envelope |

### Bind tags

| Tag | Source |
| --- | ------ |
| `path` | Path parameters (`:id` → `{id}` in OpenAPI) |
| `query` | Query string |
| `header` | Request headers |
| `cookie` | Cookies |
| `json` | JSON body (default for structs with json fields) |
| `form` | Multipart / form fields |

Routes with bindable input must declare `Responses.Err(422, ...)` on the route or inherit from a group.

## Route registration

Path-first only:

```go
r.Route("/users/:id").Get(openapi.Route{...}).Put(openapi.Route{...})
r.Describe("/upload").Post(openapi.Route{...}) // docs only — Handler must be nil
```

A route is **Documented** (included in OpenAPI output) when it sets any of: `Summary`, `Description`, `OperationID`, `Request`, `Requests`, `Responses` with success or `Err` cases, or `Extra`. Bare `Handler` + empty `Returns()` alone does not document a route.

## Response contract

```go
Responses: openapi.Returns().
    Err(404, errNotFound).
    Err(500, errInternal)
```

- Group `Responses` merge into child routes
- Success type inferred from handler return type (or `Returns(T{})` on Describe routes)
- Success is always **HTTP 200**
- Declared errors appear in spec under resolved statuses with `x-xqua-errors` metadata

## Security

Register schemes once on `Config.Schemes`:

```go
"BearerAuth": openapi.BearerScheme(openapi.BearerOptions{
    Verify: func(ctx context.Context, cred openapi.Credential) (openapi.Identity, error) {
        return auth.Verify(cred.Raw)
    },
}),
```

Declare per group/route:

```go
Security: openapi.PublicSecurity()                           // explicitly open
Security: openapi.RequireSecurity("BearerAuth")            // one scheme
Security: openapi.RequireAnySecurity("BearerAuth", "ApiKey") // OR
Security: openapi.InheritSecurity()                         // inherit parent/default
```

Secured routes must declare `Err(401, ...)` and `Err(403, ...)` when scopes apply.

### Distinct 401 codes

List every verify outcome on the group's `Err(401, ...)` list. The **first** entry is the missing-credential fallback; verify failures return the matching declared error:

```go
Responses: openapi.Returns().
    Err(401, errMissingBearer, errTokenExpired, errInvalidToken)
```

Read identity in handlers: `openapi.IdentityFrom(ctx)` or `openapi.IdentityAs[T](ctx)`.

## Group middleware (AfterAuth)

Cross-cutting logic that runs **after** authentication and **before** binding belongs on `GroupConfig.AfterAuth` — not `Router.Fiber().Use()`:

```go
v1 := r.Group(openapi.GroupConfig{
    Responses: openapi.Returns().Err(429, errRateLimited),
    AfterAuth: []openapi.Middleware{
        func(c fiber.Ctx, ctx openapi.RouteContext) error {
            if limited {
                return ctx.WriteError(errRateLimited) // stops chain, writes envelope
            }
            return nil
        },
    },
})
```

Chain order: `Guard → AfterAuth (inherited) → Binder → Handler`.

Middleware errors must be declared on group/route `Responses`.

## Patterns cookbook

Idempotency, rate limits, distinct 401s, wrapped JSON — [Common patterns](patterns.md).

## Several OpenAPI documents

```go
Specs: []openapi.Spec{
    {Path: "/openapi.json", Prefix: "/api/v1", Title: "Public API"},
    {Path: "/demo/openapi.json", Prefix: "/demo", Title: "Demo API", Webhooks: wh},
}
```

Each spec filters routes by path **prefix** (segment-aware) and optional route **tags** (`Route.Specs`).

Clients fetch the JSON from the `Path` you configure (e.g. `GET /openapi.json`).

## Manifest

`api.Manifest()` returns registration-time data: routes, declared errors, catalog — useful for codegen or contract tooling in your repo.

## OpenAPI 3.2 features

| Feature | API |
| ------- | --- |
| Streaming docs | `Route.Extra` with `ResponseDecl.ItemBody` (SSE itemSchema) |
| QUERY method | `r.Route(path).Query(route)` |
| Multipart | `Route.Requests` with `ContentType: "multipart/form-data"` + `form` tags |
| Webhooks | `Spec.Webhooks` |
| Explicit schemas | `Route.Requests`, `Returns(T{})`, `*openapi.Schema{Ref: ...}` |

## Escape hatches

| Need | Approach |
| ---- | -------- |
| Streaming / WebSocket runtime | `Router.Fiber()` |
| Raw JSON success | Return type without `openapi.Response` embed |
| Docs-only operation | `Describe(path).Method(route)` with `Handler: nil` |

## Upgrading

Moving from older workarounds? See [Patterns — Replacing old workarounds](patterns.md#replacing-old-workarounds).

## Further reading

- [Common patterns](patterns.md)
- [HTTP & envelopes](http.md)
- Showcase: `/demo/patterns/*` and `GET /demo/openapi.json`
