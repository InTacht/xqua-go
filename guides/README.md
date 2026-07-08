# xqua-go — guides

Guides for building services with xqua-go.

| Guide | Topic |
| ----- | ----- |
| [Getting started](getting-started.md) | First API: runtime, catalog, typed route, OpenAPI |
| [Errors](errors.md) | Catalogs, boundary mapping, wire contract |
| [HTTP](http.md) | RES envelope, middleware, `/health` |
| [Building APIs](openapi.md) | Routes, binding, security, OpenAPI documents |
| [Patterns](patterns.md) | Auth, rate limits, idempotency, dynamic bodies |
| [Configuration](configuration.md) | Env binding and logging |
| [Database](database.md) | Postgres migrations |
| [Multi-unit apps](multi-unit-apps.md) | Runtime units and message bus |

## Examples

```bash
go run ./examples/hello
make dev-up && go run ./examples/showcase
```

- [examples/README.md](../examples/README.md) — layout and reading order
- [examples/showcase/TESTING.md](../examples/showcase/TESTING.md) — curl recipes

## API reference

```bash
go doc github.com/InTacht/xqua-go/pkg/http/openapi
```
