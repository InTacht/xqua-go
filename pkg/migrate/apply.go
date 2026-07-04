package migrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// appliedVersions returns the set of versions recorded in schema_migrations. A
// missing schema or table (fresh database) yields an empty set.
func (m *Migrator) appliedVersions(ctx context.Context, conn *pgxpool.Conn) (map[int64]bool, error) {
	rows, err := conn.Query(ctx, fmt.Sprintf(`SELECT version FROM %s`, m.tblApplied))
	if err != nil {
		if isMissingObject(err) {
			return map[int64]bool{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

// applyUp runs a single up migration and records it in schema_migrations.
//
// An empty (whitespace-only) up file records the version without executing any
// DDL. A non-empty file runs the SQL and inserts the tracking row in one
// transaction so a failure leaves nothing partially applied.
func (m *Migrator) applyUp(ctx context.Context, conn *pgxpool.Conn, mig Migration) error {
	content, err := m.cfg.Source.ReadUp(mig)
	if err != nil {
		return err
	}

	sql := strings.TrimSpace(content)
	if sql == "" {
		_, err := conn.Exec(ctx, m.insertAppliedSQL(), mig.Version, mig.Name)
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, m.insertAppliedSQL(), mig.Version, mig.Name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// applyDown rolls back a single migration and removes its tracking row in one
// transaction. An empty down file removes the tracking row without running DDL.
func (m *Migrator) applyDown(ctx context.Context, conn *pgxpool.Conn, mig Migration) error {
	content, err := m.cfg.Source.ReadDown(mig)
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if sql := strings.TrimSpace(content); sql != "" {
		if _, err := tx.Exec(ctx, sql); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE version = $1`, m.tblApplied), mig.Version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (m *Migrator) insertAppliedSQL() string {
	return fmt.Sprintf(`INSERT INTO %s (version, name, kind) VALUES ($1, $2, 'schema')`, m.tblApplied)
}

func pendingMigrations(all []Migration, applied map[int64]bool) []Migration {
	var pending []Migration
	for _, mig := range all {
		if !applied[mig.Version] {
			pending = append(pending, mig)
		}
	}
	return pending
}
