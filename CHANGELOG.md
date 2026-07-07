# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-07-08

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

[Unreleased]: https://github.com/InTacht/xqua-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/InTacht/xqua-go/releases/tag/v0.1.0
