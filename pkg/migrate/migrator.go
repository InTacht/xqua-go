package migrate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrator applies versioned SQL migrations to a Postgres database with
// multi-replica coordination. Create one with Postgres.
type Migrator struct {
	pool *pgxpool.Pool
	cfg  Config

	// Pre-quoted, schema-qualified identifiers for the bookkeeping objects.
	qSchema    string // "_xqua_"
	tblApplied string // "_xqua_".schema_migrations
	tblState   string // "_xqua_".migration_state
	tblMeta    string // "_xqua_".meta
}

// Postgres creates a Migrator backed by a Postgres connection pool. It
// validates cfg and fills in defaults for the schema, advisory lock key, and
// poll interval. The library owns its bookkeeping tables; callers do not ship a
// bootstrap migration.
func Postgres(pool *pgxpool.Pool, cfg Config) (*Migrator, error) {
	if pool == nil {
		return nil, fmt.Errorf("migrate: pool is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	if err := validateSchemaName(cfg.Schema); err != nil {
		return nil, err
	}

	q := `"` + cfg.Schema + `"`
	return &Migrator{
		pool:       pool,
		cfg:        cfg,
		qSchema:    q,
		tblApplied: q + ".schema_migrations",
		tblState:   q + ".migration_state",
		tblMeta:    q + ".meta",
	}, nil
}

// Status compares migrations discovered from the Source against those applied in
// the database, preserving discovery order within each group. It takes no lock
// and does not create the bookkeeping schema; a database that has never been
// gated reports everything as pending.
func (m *Migrator) Status(ctx context.Context) (applied, pending []Migration, err error) {
	migrations, err := m.cfg.Source.Discover()
	if err != nil {
		return nil, nil, err
	}

	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("migrate: acquire connection: %w", err)
	}
	defer conn.Release()

	appliedSet, err := m.appliedVersions(ctx, conn)
	if err != nil {
		return nil, nil, err
	}

	for _, mig := range migrations {
		if appliedSet[mig.Version] {
			applied = append(applied, mig)
		} else {
			pending = append(pending, mig)
		}
	}
	return applied, pending, nil
}

// Down rolls back the highest applied versions, up to steps of them. Each
// rollback requires the migration's .down.sql file. It acquires the migration
// advisory lock and fails if the lock is held elsewhere; it does not modify
// migration_state.
func (m *Migrator) Down(ctx context.Context, steps int) error {
	if steps <= 0 {
		return nil
	}

	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire connection: %w", err)
	}
	defer conn.Release()

	locked, err := tryAdvisoryLock(ctx, conn, m.cfg.AdvisoryLockKey)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("migrate: could not acquire migration lock")
	}
	defer advisoryUnlock(ctx, conn, m.cfg.AdvisoryLockKey)

	if err := m.ensureMeta(ctx, conn); err != nil {
		return err
	}

	migrations, err := m.cfg.Source.Discover()
	if err != nil {
		return err
	}
	appliedSet, err := m.appliedVersions(ctx, conn)
	if err != nil {
		return err
	}

	// Highest applied versions first.
	var appliedMigrations []Migration
	for i := len(migrations) - 1; i >= 0; i-- {
		if appliedSet[migrations[i].Version] {
			appliedMigrations = append(appliedMigrations, migrations[i])
		}
	}

	for i := 0; i < steps && i < len(appliedMigrations); i++ {
		mig := appliedMigrations[i]
		if mig.DownPath == "" {
			return fmt.Errorf("migrate: cannot roll back %s: no .down.sql file", mig.Label())
		}
		if err := m.applyDown(ctx, conn, mig); err != nil {
			return fmt.Errorf("migrate: roll back %s: %w", mig.Label(), err)
		}
	}

	return nil
}
