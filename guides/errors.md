# Errors

`pkg/errors` provides catalog-driven, transport-agnostic errors. HTTP status is **not** stored on the error — it is resolved where the error is surfaced (OpenAPI route declarations).

## Core concepts

| Concept | Meaning |
| ------- | ------- |
| **Catalog** | Namespace for a module or service; one per logical space |
| **Define** | Creates a canonical `*errors.Error` entry (identity = that pointer) |
| **Kind** | Semantic category (`validation`, `not_found`, …) — matched with `errors.IsKind` |
| **Code** | Unique string within a catalog — what clients see on the wire |
| **Source** | Optional field/input hint (`body.name`, `params.id`) for validation matching |

Standard kinds are exported constants: `KindValidation`, `KindNotFound`, `KindConflict`, `KindUnauthorized`, `KindForbidden`, `KindRateLimit`, `KindInternal`. The OpenAPI engine maps them to default HTTP statuses when you declare `Responses.Err(status, ...)`.

## Define a catalog

```go
var Store = errors.NewCatalog("store")

var (
    ErrUserMissing = Store.Define(errors.Def{
        Kind: errors.KindNotFound, Code: "10001",
        Message: "user not found", Source: "params.id",
    })
    ErrStale = Store.Define(errors.Def{
        Kind: errors.KindConflict, Code: "10002", Message: "stale version",
    })
)
```

`define` panics on empty or duplicate codes within a catalog.

## Public vs internal catalogs

Every service should have **one public catalog** wired into `http.Config.Catalog`. That is the only catalog allowed on the wire.

Repository and store packages keep private catalogs. Map at the HTTP/service boundary:

```go
return errors.MapOr(err, api.ErrInternal,
    errors.Pair(store.ErrUserMissing, api.ErrUserNotFound),
    errors.Pair(store.ErrStale, api.ErrConflict),
)
```

If an internal catalog error slips through undeclared, the transport logs the full chain and renders `Fallbacks.Unhandled` — implementation details never leak.

## Wrapping and collections

| API | Use when |
| --- | -------- |
| `errors.Wrap(err, template)` | Vertical chain — attach context while keeping catalog identity |
| `errors.Errors{...}` | Horizontal collection — several independent failures (e.g. validation) |
| `errors.NewPlain(msg)` | Underlying causes only — map before surfacing |

## Matching

```go
errors.Is(err, ErrUserMissing)      // exact catalog entry identity
errors.IsKind(err, "validation")    // any validation-kind error in the tree
errors.AsErrors(err)                // extract catalog entries for logging/encoding
```

## Wire decoding

`Catalog.Lookup(code)` decodes a wire code back to the catalog entry — useful for service-to-service clients.

`Catalog.Entries()` enumerates all entries for OpenAPI generation and client tooling.

## HTTP integration

Handlers return public catalog errors. Status resolution on OpenAPI routes:

1. Explicit `Responses.Err(status, entry)` on route or group
2. If undeclared or non-catalog → bubbles to global handler

When multiple errors are returned together on a declared route, the **highest** declared status wins.

Bind failures (422) use catalog entries declared under `Responses.Err(422, ...)` with optional `Source` matching.

## Logging

`pkg/logger` encodes catalog errors as a structured `errors` array (`kind`, `code`, `message`, `source`, `cause`). Plain errors log as a single `error` string.

## Further reading

- [HTTP transport](http.md) — global error handler and RES envelope
- [OpenAPI](openapi.md) — `Responses.Err`, `x-xqua-errors` in generated specs
- `go doc github.com/InTacht/xqua-go/pkg/errors`
