# Common patterns

Recipes for problems you hit when building real APIs on xqua-go. Working code lives in `examples/showcase/app/transport/routes/patterns/` and is mounted at `/demo/patterns/*`.

## Authentication with distinct 401 codes

Register a bearer scheme and return **specific catalog errors** from `Verify` — expired vs invalid vs missing are different codes on the wire.

```go
api := openapi.New(t, openapi.Config{
    Schemes: map[string]openapi.Scheme{
        "BearerAuth": openapi.BearerScheme(openapi.BearerOptions{
            Verify: func(_ context.Context, cred openapi.Credential) (openapi.Identity, error) {
                switch cred.Raw {
                case "expired-token":
                    return nil, errTokenExpired
                case "invalid-token":
                    return nil, errInvalidToken
                case "good-token":
                    return userID, nil
                default:
                    return nil, errInvalidToken
                }
            },
        }),
    },
})

v1 := r.Group(openapi.GroupConfig{
    Security: openapi.RequireSecurity("BearerAuth"),
    Responses: openapi.Returns().
        Err(401, errMissingBearer, errTokenExpired, errInvalidToken),
})
```

- **First** `Err(401, …)` entry = missing credential
- Verify failures return the matching declared error (not a generic 401)
- Read the user in handlers: `openapi.IdentityFrom(ctx)` or `openapi.IdentityAs[MyUser](ctx)`

Login routes use `Security: openapi.PublicSecurity()` on that route only.

## Rate limiting after auth

Do **not** hang rate limits on `Router.Fiber().Use()` — they run in the wrong place and bypass declared errors.

Use `GroupConfig.AfterAuth`:

```go
v1 := r.Group(openapi.GroupConfig{
    Responses: openapi.Returns().Err(429, errRateLimited),
    AfterAuth: []openapi.Middleware{
        func(c fiber.Ctx, ctx openapi.RouteContext) error {
            if overLimit(c) {
                return ctx.WriteError(errRateLimited)
            }
            return nil
        },
    },
})
```

Middleware runs after the guard, before binding. Declare `Err(429, …)` on the group so it appears in OpenAPI and maps correctly at runtime.

Nested groups **inherit** parent `AfterAuth` middleware.

## Idempotency — replay as HTTP 200

Duplicate idempotent requests should return **the same success shape**, not 409 with errors + data mixed together.

```go
type triggerOut struct {
    openapi.Response
    Data struct {
        RunID    string `json:"run_id"`
        Replayed bool   `json:"replayed,omitempty"`
    } `json:"data"`
}

func trigger(_ context.Context, in triggerIn) (triggerOut, error) {
    if runID, ok := seen[in.IdempotencyKey]; ok {
        var out triggerOut
        out.Message = "duplicate idempotency key; returning existing run"
        out.Data.RunID = runID
        out.Data.Replayed = true
        return out, nil
    }
    // … create run, store idempotency key …
    return out, nil
}
```

Clients check `data.replayed`. Reserve **409** for conflicts the client must fix (stale version, uniqueness), not idempotency replay.

## Dynamic JSON bodies — use a wrapper field

When the payload is a free-form map, wrap it under a **domain-named** property:

```json
{"metadata": {"tier": "gold", "plan": "pro"}}
```

```go
type replaceDataIn struct {
    ID       string            `path:"id"`
    Metadata map[string]string `json:"metadata" required:"true"`
}
```

Pick a name that matches your domain: `metadata`, `attributes`, `properties`, etc. Avoid `json:"data"` on requests when responses already use a `data` envelope.

## Multipart uploads

Declare the body explicitly and use `form` tags on the input struct:

```go
Requests: []openapi.ContentUnit{{
    Required:    true,
    ContentType: "multipart/form-data",
    Structure:   uploadIn{},
}},
```

```go
type uploadIn struct {
    File *multipart.FileHeader `form:"file" required:"true"`
}
```

## Several error codes on one status

Group `Responses` merge into routes. Declare shared auth/validation/rate-limit errors once on the group; add route-specific errors on the route.

## Try it

```bash
make dev-up && go run ./examples/showcase
```

```bash
# Distinct 401
curl -s -X POST http://127.0.0.1:8080/demo/patterns/trigger \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: k1' \
  -d '{"payload":"x"}' | jq .

# Wrapped metadata PUT
curl -s -X PUT http://127.0.0.1:8080/demo/patterns/subscribers/sub-1/data \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer good-token' \
  -d '{"metadata":{"tier":"gold"}}' | jq .
```

OpenAPI for these routes: `GET /demo/openapi.json`.

## Replacing old workarounds

| Instead of… | Use… |
| ----------- | ---- |
| Custom bearer on `Fiber().Use()` | `RequireSecurity` + `Verify` + group `Err(401, …)` |
| 409 + errors + data on idempotent replay | HTTP 200 + `data.replayed: true` |
| Flat PUT root map | Wrapped field: `{"metadata":{...}}` |
| `Fiber().Use(rateLimit)` | `GroupConfig.AfterAuth` + `ctx.WriteError` |
| Custom global handler for catalog errors | Delete — adapter writes declared errors |

Reference: `examples/showcase/app/transport/routes/patterns/`.
