package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunStartupGate applies pending migrations with replica-safe coordination and
// must be called once per process before serving traffic.
//
// One replica acquires a Postgres advisory lock and becomes the leader,
// applying pending migrations in order; the others follow, retrying the lock
// every PollInterval until they can confirm the database is at head. If a
// previous run left migration_state in the failed state, the gate refuses to
// start.
//
// Liveness comes from the advisory lock, not migration_state: session-level
// advisory locks are released by Postgres when the holder's connection dies, so
// a leader that crashes mid-migration (even leaving migration_state stuck at
// "running") is recovered from automatically — the next replica to retry
// acquires the freed lock and finishes the remaining migrations. There is no
// internal wall-clock timeout; bound the gate with a context deadline when the
// deployment needs one:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
//	defer cancel()
//	err := m.RunStartupGate(ctx)
func (m *Migrator) RunStartupGate(ctx context.Context) error {
	for {
		done, err := m.attemptGate(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.cfg.PollInterval):
		}
	}
}

// attemptGate runs a single leader/follower attempt. It returns done=true when
// the database is confirmed at head, or done=false when another replica holds
// the lock and the caller should retry after PollInterval.
//
// Followers deliberately do not wait on migration_state: a crashed leader can
// leave status at "running" forever, while its advisory lock is always released
// by Postgres. Re-attempting the lock on every poll makes a follower take over
// from a dead leader instead of waiting on a state row that will never change.
func (m *Migrator) attemptGate(ctx context.Context) (bool, error) {
	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("migrate: acquire connection: %w", err)
	}
	defer conn.Release()

	status, err := m.readStatus(ctx, conn)
	if err != nil {
		return false, err
	}
	if status == statusFailed {
		return false, fmt.Errorf("migrate: refusing to start: migration_state is %q", statusFailed)
	}

	locked, err := tryAdvisoryLock(ctx, conn, m.cfg.AdvisoryLockKey)
	if err != nil {
		return false, err
	}
	if !locked {
		// A live leader holds the lock; poll again shortly.
		return false, nil
	}
	defer advisoryUnlock(ctx, conn, m.cfg.AdvisoryLockKey)

	if err := m.runLeader(ctx, conn); err != nil {
		return false, err
	}
	return true, nil
}

// runLeader bootstraps the library's bookkeeping schema, then discovers pending
// migrations and applies them in order under the advisory lock, updating
// migration_state as it goes.
func (m *Migrator) runLeader(ctx context.Context, conn *pgxpool.Conn) error {
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

	pending := pendingMigrations(migrations, appliedSet)
	if len(pending) == 0 {
		return m.setStateIdle(ctx, conn)
	}

	target := pending[len(pending)-1].Version
	if err := m.setStateRunning(ctx, conn, m.cfg.InstanceID, target); err != nil {
		return err
	}

	for _, mig := range pending {
		if err := m.applyUp(ctx, conn, mig); err != nil {
			applyErr := fmt.Errorf("migrate: apply %s: %w", mig.Label(), err)
			if stateErr := m.setStateFailed(ctx, conn, applyErr.Error()); stateErr != nil {
				return fmt.Errorf("%w (additionally failed to record state: %v)", applyErr, stateErr)
			}
			return applyErr
		}
	}

	return m.setStateIdle(ctx, conn)
}

func tryAdvisoryLock(ctx context.Context, conn *pgxpool.Conn, key int64) (bool, error) {
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&locked); err != nil {
		return false, fmt.Errorf("migrate: advisory lock: %w", err)
	}
	return locked, nil
}

func advisoryUnlock(ctx context.Context, conn *pgxpool.Conn, key int64) {
	_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, key)
}
