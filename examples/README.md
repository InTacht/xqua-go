# Examples

Two tiers: start with **`hello`**, then open **`showcase`** for the full HTTP/OpenAPI surface. The rest are **separate programs** — each demonstrates one orthogonal idea (bus, ports, logging).

## Tier 1 — HTTP essentials

| Example | Run | What it teaches |
|---------|-----|-----------------|
| [`hello`](./hello) | `go run ./examples/hello` | Minimal `runtime` + typed handler + RES envelope + `/openapi.json` |
| [`showcase`](./showcase) | `make dev-up && go run ./examples/showcase` | **Everything else HTTP** in one place (see below) |

### What `showcase` covers

One process, one transport — deliberately rich:

| Area | Routes / docs | Capability |
|------|----------------|------------|
| Postgres API | `GET /api/v1/users`, `GET /api/v1/users/:id` | Migrations, store boundary `MapOr`, health check, request logging |
| Demo (in-memory) | `GET /demo/items/:id`, `POST /demo/items` | Catalog mapping, validation collections, group 422 inheritance |
| Multipart | `POST /demo/upload` | `form` tags + `*multipart.FileHeader`, OpenAPI `Requests` encoding |
| Multi-surface OpenAPI | `/openapi.json`, `/mobile/openapi.json`, `/console/openapi.json`, `/demo/openapi.json` | Prefix + tag filtering, component `$ref` schemas |
| Docs-only | `GET /demo/ws` (Describe) | 101 WebSocket in spec, no Fiber handler |
| Escape hatches | `GET /demo/leak`, `/demo/plain`, `/api/v1/boom` | Internal error leak protection, plain errors, imperative Fiber |

```bash
make dev-up && go run ./examples/showcase

curl http://127.0.0.1:8080/api/v1/users
curl http://127.0.0.1:8080/demo/items/1
curl http://127.0.0.1:8080/demo/items/99          # internal → mapped 500
curl -X POST http://127.0.0.1:8080/demo/items     # validation collection → 422
curl -F title=report -F file=@README.md http://127.0.0.1:8080/demo/upload
curl http://127.0.0.1:8080/demo/leak              # internal never leaks
curl http://127.0.0.1:8080/openapi.json
curl http://127.0.0.1:8080/demo/openapi.json
```

## Tier 2 — Separate concerns

Each program adds **one** idea not duplicated in `showcase`:

| Example | Run | What it teaches |
|---------|-----|-----------------|
| [`multiport`](./multiport) | `go run ./examples/multiport` | Two HTTP units, two ports (`:8080` + `:8081`) in one runtime |
| [`bus`](./bus) | `go run ./examples/bus` | Local bus: HTTP → `Request`, competing `QueueSubscribe` workers |
| [`split`](./split) | `go run ./examples/split` | Compute + storage as separate units, no shared pointers — bus only |
| [`logging`](./logging) | `go run ./examples/logging` | Structured error logging (no HTTP) |

```bash
go run ./examples/multiport
go run ./examples/bus
go run ./examples/split
go run ./examples/logging
```

## Suggested reading order

```text
hello → showcase → multiport → bus → split → logging
```

Skip straight to `showcase` if you already know Go services; use `hello` when you want the smallest possible process first.

## Mental model

```text
main             builds deps, owns teardown (defer)
     │
runtime          supervises Units (lifecycle only)
     │
     ├─ http.Transport     Fiber app, catalog safety net, middleware, /health
     │       └─ openapi.Generator   typed routes + generated OpenAPI docs
     ├─ worker.Unit        custom Unit (Subscribe / QueueSubscribe)
     └─ storage.Unit       another custom Unit
              │
              └── bus (app ctx)   units never call each other by pointer
```

- **Logger**: `logger.New` + `defer appLog.Close()` on the root only. Derive per-unit labels (`log.Derive("http")`); never Close children.
- **Dependencies** are built in `main` / `run()` and released with `defer`.
- **Narrowing**: unit factories pull only what they need (`worker.New(bus, log.Derive("worker"))`).
- **Errors** on the wire come only from the public catalog; map internal catalogs at boundaries.
- **OpenAPI**: `openapi.New(t, ...)`, `r.Route(path).Get(openapi.Route{...})`, `Returns().Err(...)`. Imperative routes use `t.Fiber()`.
