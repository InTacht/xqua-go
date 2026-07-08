# xqua-go

Go SDK for building high-performance, container-ready services.

The library is **stateless and config-first**: it does not read environment variables or assume deployment defaults. Your program loads configuration, assembles dependencies, and passes them in.

## Guides

**[guides/README.md](guides/README.md)** — how to build services with xqua-go.

```bash
go run ./examples/hello
make dev-up && go run ./examples/showcase
```

## Packages

| Package | Role |
|---------|------|
| `pkg/runtime` | Process supervisor, `Unit`, graceful shutdown |
| `pkg/logger` | Structured zap logging |
| `pkg/errors` | Catalog-driven API errors |
| `pkg/env` | Env binding for `main` |
| `pkg/http` | Fiber transport, RES envelope, `/health` |
| `pkg/http/openapi` | Typed handlers + OpenAPI 3.2 generation |
| `pkg/bus` | Message bus between units |
| `pkg/migrate` | Postgres migrations with replica-safe startup |

## Try it

```bash
go get github.com/InTacht/xqua-go@latest
go run ./examples/hello
make dev-up && go run ./examples/showcase   # full reference app
```

Requires Go 1.26.4+.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [CHANGELOG.md](CHANGELOG.md).
