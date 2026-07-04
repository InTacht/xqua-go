// Package migrate applies SQL-first, sequential database migrations with
// multi-replica coordination. Backends are selected by constructor: Postgres
// creates a migrator backed by a pgx connection pool, and future backends can
// add their own constructors without changing the package surface.
//
// # File naming
//
// Migration files live in a directory (or embedded FS) and are named:
//
//	000001_initial_schema.up.sql
//	000001_initial_schema.down.sql
//
// The name is a six-digit zero-padded version, a snake_case description, and an
// up or down direction. Numbering starts at 000001 and is entirely the
// application's own; the library does not reserve any version. Every version
// needs an up file; the down file is optional and only required to roll that
// version back. Files that do not match the convention are ignored.
//
// # Library-owned bookkeeping
//
// The runner records applied versions in schema_migrations and coordinates
// replicas through migration_state. These tables are created and maintained by
// the library itself: applications do not ship a bootstrap migration for them.
// For Postgres they live in a dedicated schema (Config.Schema, default
// "_xqua_") so they stay separate from application tables, which continue to
// target whatever schema their own SQL specifies.
//
// The library also tracks the version of its own bookkeeping layout in an
// internal meta table. On startup it upgrades that layout as needed, so moving
// to a newer library release that changes the tracking tables is automatic and
// requires no application action. Application migration SQL must never write to
// schema_migrations; the runner owns tracking inserts.
//
// # Startup gate
//
// Call RunStartupGate once per process, before serving traffic. One replica
// acquires a Postgres advisory lock, ensures the bookkeeping schema exists, and
// applies pending migrations in order (each in its own transaction alongside its
// tracking insert); other replicas retry the lock until the database is at head.
// If a prior run left migration_state in the failed state, the gate refuses to
// start so a broken deployment fails closed.
//
// A leader that dies mid-run does not wedge the gate: Postgres releases
// session-level advisory locks when the holder's connection dies, so another
// replica acquires the freed lock and finishes the remaining migrations, even if
// the crashed leader left migration_state at "running". The gate has no
// internal wall-clock timeout; pass a context with a deadline to bound how long
// startup may wait.
//
//	pool, err := pgxpool.New(ctx, databaseURL)
//	if err != nil {
//	    return err
//	}
//	m, err := migrate.Postgres(pool, migrate.Config{
//	    Source:     migrate.Source{Dir: "migrations"},
//	    InstanceID: cfg.ID,
//	})
//	if err != nil {
//	    return err
//	}
//	r.OnStartup(func(ctx context.Context) error {
//	    return m.RunStartupGate(ctx)
//	})
//
// Order in a process: create the pool, run the migrate gate, then other startup
// hooks, then serve.
//
// # Operations
//
// Status compares migrations on disk with those applied, and Down rolls back the
// highest applied versions. Both are intended for ops tooling (for example a
// "migrate status" or "migrate down" subcommand in the consumer app) rather than
// the startup path.
package migrate
