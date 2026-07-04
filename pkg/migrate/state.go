package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	statusIdle    = "idle"
	statusRunning = "running"
	statusFailed  = "failed"
)

// readStatus returns the current migration_state status. A missing schema or
// table, or a missing singleton row, is treated as idle so a fresh database
// (before the library bootstraps its tables) does not block startup.
func (m *Migrator) readStatus(ctx context.Context, conn *pgxpool.Conn) (string, error) {
	var status string
	err := conn.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s WHERE id = 1`, m.tblState)).Scan(&status)
	switch {
	case err == nil:
		return status, nil
	case errors.Is(err, pgx.ErrNoRows):
		return statusIdle, nil
	case isMissingObject(err):
		return statusIdle, nil
	default:
		return "", err
	}
}

// setStateRunning marks the singleton state row as running for the given holder
// and target version. Callers must have run ensureMeta first.
func (m *Migrator) setStateRunning(ctx context.Context, conn *pgxpool.Conn, holderID string, target int64) error {
	_, err := conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, status, holder_id, target_version, started_at, finished_at, last_error)
		VALUES (1, 'running', $1, $2, now(), NULL, NULL)
		ON CONFLICT (id) DO UPDATE SET
			status = 'running',
			holder_id = EXCLUDED.holder_id,
			target_version = EXCLUDED.target_version,
			started_at = now(),
			finished_at = NULL,
			last_error = NULL
	`, m.tblState), holderID, target)
	return err
}

// setStateIdle marks the singleton state row as idle.
func (m *Migrator) setStateIdle(ctx context.Context, conn *pgxpool.Conn) error {
	_, err := conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, status, holder_id, target_version, started_at, finished_at, last_error)
		VALUES (1, 'idle', NULL, NULL, NULL, now(), NULL)
		ON CONFLICT (id) DO UPDATE SET
			status = 'idle',
			holder_id = NULL,
			finished_at = now(),
			last_error = NULL
	`, m.tblState))
	return err
}

// setStateFailed records a failure on the singleton state row.
func (m *Migrator) setStateFailed(ctx context.Context, conn *pgxpool.Conn, message string) error {
	_, err := conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, status, holder_id, target_version, started_at, finished_at, last_error)
		VALUES (1, 'failed', NULL, NULL, NULL, now(), $1)
		ON CONFLICT (id) DO UPDATE SET
			status = 'failed',
			finished_at = now(),
			last_error = EXCLUDED.last_error
	`, m.tblState), message)
	return err
}
