# Examples

Two tiers: start with **`hello`**, then open **`showcase`** for the full HTTP/OpenAPI surface. The rest are **separate programs** — each demonstrates one orthogonal idea (bus, ports, logging).

## Tier 1 — HTTP essentials

| Example | Run | What it teaches |
|---------|-----|-----------------|
| [`hello`](./hello) | `go run ./examples/hello` | Minimal `runtime` + typed handler + RES envelope + `/openapi.json` |
| [`showcase`](./showcase) | `make dev-up && go run ./examples/showcase` | **Everything else HTTP** in one place (see below) |

### Layout

```text
examples/showcase/
  main.go                 entrypoint → app.Run()
  app/
    app.go                config, Run()
    wire.go               repository + service composition root
    migrations/
      core/               users schema (DATABASE_URL)
      demo/               audit schema (DEMO_DATABASE_URL)
    transport/
      errors/             public HTTP catalog
      auth/               security schemes + auth routes
      routes/
        users/            /api/v1/users + audit
        demo/             /demo/* engine demos + escape hatches
        surfaces/         /mobile, /console OpenAPI surfaces
      http.go             compose HTTP + OpenAPI
  pkg/
    domain/               User, Item, Session, AuditEntry
    repository/
      repository.go       interfaces + Repo struct
      repo.go             Repo facade fields + Ping
      postgres/
        core/             core DB — users
        demo/             demo DB — audit trail
      memory/             ephemeral items + API tokens
    services/             use cases (accept *repository.Repo)
```

Layering:

```text
transport  →  services  →  repository.Repo  →  backends  →  domain
     ↓                           │
  errors                    core PG · demo PG · memory
```

Backends (wired once in `app/wire.go`):

| Backend | Env | Holds |
|---------|-----|--------|
| Core Postgres | `DATABASE_URL` | Users |
| Demo Postgres | `DEMO_DATABASE_URL` | Audit trail |
| Memory | (in-process) | Demo items, API tokens |

First-time dev setup creates the `demo` database via `scripts/dev-postgres-init/`. If the volume already exists, run `make dev-reset` once.

### What `showcase` covers

One process, one transport — deliberately rich. **Full curl recipes:** [`showcase/TESTING.md`](./showcase/TESTING.md).

| Area | Routes / docs | Capability |
|------|----------------|------------|
| Application shell | `main` → `app.Run()` | Config, logging, migrations, runtime, composition root |
| Composition root | `app.Wire(WireDeps{Core, Demo})` | Repo facade + services; multi-backend ping |
| Core Postgres API | `GET/PUT /api/v1/users`, `GET /api/v1/users/:id` | REST verbs, pagination, `MapOr`, 409 stale/conflict |
| Cross-DB use case | `GET /api/v1/users/:id/audit` | Service reads core + demo Postgres (no cross-DB transaction) |
| Demo (memory) | `GET/POST/PATCH/DELETE /demo/items`, `QUERY /demo/search` | CRUD, cursor pagination, HTTP QUERY, validation collections |
| Binding extras | `GET /demo/preferences` | `header:` + `cookie:` tags |
| Auth / security | `/demo/auth/login`, `/demo/me`, `/demo/session`, `/demo/admin`, `/demo/scoped-admin` | Bearer, header key, cookie key, OAuth2 scopes → 403 |
| Multipart | `POST /demo/upload` | `form` tags + `*multipart.FileHeader`, OpenAPI `Requests` encoding |
| Multi-surface OpenAPI | `/openapi.json`, `/mobile/...`, `/console/...`, `/demo/...` | Prefix + tag filtering, `$ref` schemas, webhooks |
| Streaming / binary (docs) | `POST /demo/stream`, `GET /demo/export/:id` | SSE itemSchema in spec; imperative octet-stream export |
| Raw JSON | `GET /demo/raw` | Success without RES envelope |
| Docs-only | `GET /demo/ws` (Describe) | 101 WebSocket in spec, no Fiber handler |
| Escape hatches | `GET /demo/leak`, `/demo/plain` | Internal error leak protection, plain errors, imperative Fiber |

Quick start:

```bash
make dev-up && go run ./examples/showcase
# See showcase/TESTING.md for every curl command
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
- **Dependencies** are built in `app.Run()` and released with `defer`.
- **Narrowing**: unit factories pull only what they need (`worker.New(bus, log.Derive("worker"))`).
- **Errors** on the wire come only from the public catalog; map internal catalogs at boundaries.
- **OpenAPI**: `openapi.New(t, ...)`, `r.Route(path).Get(openapi.Route{...})`, `Returns().Err(...)`. Imperative routes use `t.Fiber()`.
