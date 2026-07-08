# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-07-09

### Added

- `GroupConfig.AfterAuth` — first-class group middleware after security guard, before binding; inherited across nested groups
- Security guard preserves declared 401 verify errors (distinct codes such as expired vs invalid token)
- Showcase `/demo/patterns` routes demonstrating AfterAuth rate limiting, replay-200 idempotency, wrapped dynamic bodies, and distinct 401 codes
- User guides under [guides/](guides/README.md)
- OpenAPI golden snapshot and breaking-change spec/runtime tests
- Showcase integration tests for `/demo/patterns/*` routes

### Changed

- **Breaking:** Idempotent duplicate requests should return **HTTP 200** with a `replayed` flag in the success envelope, not 409 with a hybrid error+data body
- **Breaking:** Dynamic JSON bodies should use a **domain-named wrapper field** (e.g. `metadata`, `attributes`) instead of a flat root map
- **Breaking:** Post-auth middleware (rate limits, etc.) must use `GroupConfig.AfterAuth` instead of `Router.Fiber().Use()`

### Fixed

- OpenAPI security guard no longer replaces declared 401 verify errors with the generic missing-credential fallback
- `ctx.WriteError` in AfterAuth middleware now stops the handler chain after writing the response

### Migration summary

| Old hack | New pattern |
| -------- | ----------- |
| Custom `UserBearer` + `PublicSecurity()` | `RequireSecurity` + distinct verify errors on group `Err(401, ...)` |
| 409 + errors + data on duplicate trigger | 200 + `data.replayed: true` |
| Flat PUT body `{"key":"val"}` | Wrapped `{"<domainField>":{"key":"val"}}` |
| `Fiber().Use(rateLimit)` | `GroupConfig.AfterAuth` |
| Custom `ErrorHandler` for catalog errors | Delete — adapter writes all declared errors |

See [guides/patterns.md](guides/patterns.md#replacing-old-workarounds) for migration guidance.

### Install

```bash
go get github.com/InTacht/xqua-go@v0.2.0
```

Requires Go 1.26.4+.

## [0.1.0] - 2026-07-07

First public release of the xqua-go SDK.

### Added

- `pkg/runtime` — headless process supervisor with graceful shutdown
- `pkg/logger` — structured zap logging with canonical error fields
- `pkg/errors` — catalog-driven errors, wrapping, and conversion
- `pkg/env` — typed environment lookups and struct binding
- `pkg/http` — Fiber v3 transport, middleware, and JSON RES envelope
- `pkg/http/openapi` — declarative OpenAPI 3.2 engine with typed handlers
- `pkg/bus` — inter-unit message bus (local backend)
- `pkg/migrate` — SQL migrations with multi-replica startup gate
- Config-first, stateless library design
- Catalog-driven public error contracts with HTTP status at the route layer
- OpenAPI 3.2 generation: security schemes, multipart, QUERY, streaming docs
- Examples: `hello`, `showcase` (Postgres), `multiport`, `bus`, `split`, `logging`
- Local dev environment via Docker Compose and PostgreSQL

### Install

```bash
go get github.com/InTacht/xqua-go@v0.1.0
```

Requires Go 1.26.4+.

[Unreleased]: https://github.com/InTacht/xqua-go/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/InTacht/xqua-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/InTacht/xqua-go/releases/tag/v0.1.0
