# Examples

Headless packages, assembled by you. Read them in this order — each adds one idea.

| # | Example | Run | What it teaches |
|---|---------|-----|-----------------|
| 1 | [`hello`](./hello) | `go run ./examples/hello` | Minimal `runtime` + one HTTP `Unit`, catalog fallbacks, RES envelope |
| 2 | [`catalog`](./catalog) | `go run ./examples/catalog` | Public catalog, kind→status, internal errors never leak, `Pair` / validation collections |
| 3 | [`multiport`](./multiport) | `go run ./examples/multiport` | Several units in one process (public `:8080` + admin `:8081`) |
| 4 | [`bus`](./bus) | `go run ./examples/bus` | Local bus: HTTP request/replies to competing worker units |
| 5 | [`split`](./split) | `go run ./examples/split` | Compute (HTTP) and storage as separate units talking only over the bus |
| 6 | [`api`](./api) | `make dev-up && go run ./examples/api` | Postgres, migrations, hooks, store boundary mapping |
| 7 | [`logging`](./logging) | `go run ./examples/logging` | Structured error logging alone (no HTTP) |

```bash
go run ./examples/hello
go run ./examples/catalog
go run ./examples/multiport
go run ./examples/bus
go run ./examples/split
make dev-up && go run ./examples/api
go run ./examples/logging
```

## Mental model

```text
main             builds deps, owns teardown (defer)
     │
runtime          supervises Units (lifecycle only)
     │
     ├─ http.Unit      binds a port, public catalog on the wire
     ├─ worker.Unit    custom Unit (Subscribe / QueueSubscribe)
     └─ storage.Unit   another custom Unit
              │
              └── bus (app ctx)   units never call each other by pointer
```

- **Logger**: `logger.New` + `defer appLog.Close()` on the root only; `runtime.New(ctx, log)` is ctx-first. Derive per-unit labels in factories (`log.Derive("http")`); never Close children (shared zap core).
- **Dependencies** are built in `main` and released there with `defer`; runtime has no `Build`/`Destroy` contract.
- **Narrowing**: unit factories pull only what a unit needs out of the app context, so unit packages never import your context type (`worker.New(bus, log.Derive("worker"))`).
- **Bus** is a dependency built in `main` and passed via the app context, not on runtime.
- **Errors** on the wire come only from the public catalog; map internal catalogs at boundaries.
