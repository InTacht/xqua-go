# Database migrations

Run versioned SQL against Postgres with safe startup when several replicas boot at once.

## Migration files

```
migrations/
  000001_initial_schema.up.sql
  000001_initial_schema.down.sql   # optional rollback
```

- `000001` — six-digit version you own end-to-end
- `snake_case_description` — human-readable name
- `.up.sql` / `.down.sql` — direction

## Embedded in your binary

```go
//go:embed migrations/*.sql
var migrationFS embed.FS
```

```go
m, err := migrate.Postgres(pool, migrate.Config{
    Source:     migrate.Source{FS: migrationFS, Dir: "migrations"},
    InstanceID: cfg.InstanceID,
})
```

## Startup gate (before traffic)

One replica applies pending migrations; others wait:

```go
pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
if err != nil { return err }
defer pool.Close()

m, err := migrate.Postgres(pool, migrate.Config{
    Source:     migrate.Source{Dir: "migrations"},
    InstanceID: cfg.InstanceID,
})
if err != nil { return err }

r.OnStartup(func(ctx context.Context) error {
    return m.RunStartupGate(ctx)
})
```

If a previous deploy left migrations in a **failed** state, the gate refuses to start — fix the DB before rolling forward.

Pass a context with deadline if startup must not wait forever.

## What the library owns

Bookkeeping tables live in schema `_xqua_` by default (`schema_migrations`, etc.). You do **not** ship a bootstrap migration for them. Your application SQL should only touch your own tables.

## Ops commands (optional CLI)

For admin tooling in your repo:

```go
applied, pending, err := m.Status(ctx)
err = m.Down(ctx, 1) // roll back one version
```

## Full example

`examples/showcase` runs core + demo migration sets on two databases. See `app/migrations/` and `app/app.go`.
