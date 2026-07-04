package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// metaSteps upgrade the library's own bookkeeping layout. Index i upgrades the
// meta layout from version i to version i+1. Steps must be append-only and
// forward-compatible: never edit or remove a released step, only add new ones.
// This is how the library migrates its own tables across versions.
//
// Each step receives the pre-quoted, schema-qualified prefix (e.g.
// "_xqua_") and returns idempotent DDL run under the advisory lock.
var metaSteps = []func(qSchema string) string{
	// v0 -> v1: create the version tracking tables.
	func(s string) string {
		return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.schema_migrations (
    version    BIGINT PRIMARY KEY,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL DEFAULT 'schema',
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.migration_state (
    id             INT PRIMARY KEY,
    status         TEXT NOT NULL,
    holder_id      TEXT,
    target_version BIGINT,
    started_at     TIMESTAMPTZ,
    finished_at    TIMESTAMPTZ,
    last_error     TEXT
);`, s)
	},
}

// currentMetaVersion is the meta layout version this build targets.
var currentMetaVersion = len(metaSteps)

// ensureMeta creates the bookkeeping schema and brings its internal layout up to
// currentMetaVersion. It is idempotent and must run under the advisory lock (so
// only the leader mutates the layout). This is the library-owned replacement for
// a user-shipped bootstrap migration, and it also upgrades the bookkeeping
// tables when a newer library version adds meta steps.
func (m *Migrator) ensureMeta(ctx context.Context, conn *pgxpool.Conn) error {
	if _, err := conn.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, m.qSchema)); err != nil {
		return fmt.Errorf("migrate: ensure schema: %w", err)
	}
	if _, err := conn.Exec(ctx, fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (id INT PRIMARY KEY, version INT NOT NULL)`, m.tblMeta,
	)); err != nil {
		return fmt.Errorf("migrate: ensure meta table: %w", err)
	}

	var version int
	err := conn.QueryRow(ctx, fmt.Sprintf(`SELECT version FROM %s WHERE id = 1`, m.tblMeta)).Scan(&version)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		version = 0
	case err != nil:
		return fmt.Errorf("migrate: read meta version: %w", err)
	}

	for v := version; v < currentMetaVersion; v++ {
		if _, err := conn.Exec(ctx, metaSteps[v](m.qSchema)); err != nil {
			return fmt.Errorf("migrate: apply meta step %d->%d: %w", v, v+1, err)
		}
	}

	if version < currentMetaVersion {
		if _, err := conn.Exec(ctx, fmt.Sprintf(
			`INSERT INTO %s (id, version) VALUES (1, $1)
			 ON CONFLICT (id) DO UPDATE SET version = EXCLUDED.version`, m.tblMeta,
		), currentMetaVersion); err != nil {
			return fmt.Errorf("migrate: record meta version: %w", err)
		}
	}

	return nil
}
