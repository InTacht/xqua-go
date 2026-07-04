//go:build integration

// Package migrate integration tests run against a real Postgres.
//
// Start the dev database and run them via the Makefile:
//
//	make dev-up
//	make test-integration
//
// Or manually, with DATABASE_URL pointing at a reachable Postgres:
//
//	DATABASE_URL=postgres://app:app@localhost:5432/app?sslmode=disable \
//	    go test -tags=integration ./pkg/migrate/
//
// Tests are skipped when DATABASE_URL is unset.
package migrate_test

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/InTacht/xqua-go/pkg/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed testdata/migrations/*.sql
var embeddedMigrations embed.FS

func databaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; start the dev database with `make dev-up`")
	}
	return url
}

// freshDB creates a new, empty database on the target server so each test starts
// from a clean slate, and returns a pool connected to it.
func freshDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := databaseURL(t)

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	defer admin.Close()

	dbName := fmt.Sprintf("t_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("create database: %v", err)
	}

	pool, err := pgxpool.New(ctx, withDBName(t, base, dbName))
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func withDBName(t *testing.T, baseURL, dbName string) string {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(baseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	c := cfg.ConnConfig
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", c.User, c.Password, c.Host, c.Port, dbName)
}

func testdataSource() migrate.Source {
	return migrate.Source{Dir: "testdata/migrations"}
}

func newMigrator(t *testing.T, pool *pgxpool.Pool, src migrate.Source) *migrate.Migrator {
	t.Helper()
	m, err := migrate.Postgres(pool, migrate.Config{
		Source:       src,
		InstanceID:   "test-instance",
		PollInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("migrate.Postgres: %v", err)
	}
	return m
}

func TestGateAppliesAllOnFreshDB(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	m := newMigrator(t, db, testdataSource())

	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("RunStartupGate: %v", err)
	}

	applied, pending, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending, got %d", len(pending))
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied, got %d", len(applied))
	}
}

func TestGateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	m := newMigrator(t, db, testdataSource())

	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("first gate: %v", err)
	}
	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("second gate: %v", err)
	}
}

func TestGateAppliesNewPending(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)

	dir := t.TempDir()
	copyTestdata(t, dir)
	m := newMigrator(t, db, migrate.Source{Dir: dir})

	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("first gate: %v", err)
	}

	writeFile(t, dir, "000003_gadgets.up.sql", "CREATE TABLE gadgets (id BIGSERIAL PRIMARY KEY);")
	writeFile(t, dir, "000003_gadgets.down.sql", "DROP TABLE gadgets;")

	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("second gate: %v", err)
	}
	applied, pending, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(pending) != 0 || len(applied) != 3 {
		t.Fatalf("expected 3 applied and 0 pending, got %d applied %d pending", len(applied), len(pending))
	}
}

func TestGateEmptyUpFileRecordsVersion(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)

	dir := t.TempDir()
	copyTestdata(t, dir)
	writeFile(t, dir, "000003_placeholder.up.sql", "   \n")
	m := newMigrator(t, db, migrate.Source{Dir: dir})

	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("gate: %v", err)
	}
	applied, _, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("expected 3 applied, got %d", len(applied))
	}
}

func TestGateFailsAndLatches(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)

	dir := t.TempDir()
	copyTestdata(t, dir)
	writeFile(t, dir, "000003_broken.up.sql", "THIS IS NOT SQL;")
	m := newMigrator(t, db, migrate.Source{Dir: dir})

	if err := m.RunStartupGate(ctx); err == nil {
		t.Fatal("expected gate to fail on broken migration")
	}
	if err := m.RunStartupGate(ctx); err == nil {
		t.Fatal("expected gate to stay failed on next start")
	}
}

func TestDownRollsBack(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	m := newMigrator(t, db, testdataSource())

	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("gate: %v", err)
	}
	if err := m.Down(ctx, 1); err != nil {
		t.Fatalf("down: %v", err)
	}
	applied, pending, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(applied) != 1 || len(pending) != 1 {
		t.Fatalf("expected 1 applied 1 pending after down, got %d applied %d pending", len(applied), len(pending))
	}
}

func TestDownMissingDownFileFails(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)

	dir := t.TempDir()
	copyTestdata(t, dir)
	writeFile(t, dir, "000003_no_down.up.sql", "CREATE TABLE no_down (id INT);")
	m := newMigrator(t, db, migrate.Source{Dir: dir})

	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("gate: %v", err)
	}
	if err := m.Down(ctx, 1); err == nil {
		t.Fatal("expected down to fail without .down.sql")
	}
}

func TestGateWithEmbeddedFS(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)
	m := newMigrator(t, db, migrate.Source{FS: embeddedMigrations, Dir: "testdata/migrations"})

	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("gate: %v", err)
	}
	applied, pending, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(applied) != 2 || len(pending) != 0 {
		t.Fatalf("expected 2 applied 0 pending, got %d applied %d pending", len(applied), len(pending))
	}
}

// TestGateRecoversFromZombieLeader simulates a leader that crashed mid-run: its
// advisory lock is gone (Postgres releases session locks when the connection
// dies) but migration_state is stuck at "running". A follower must take over by
// re-attempting the lock rather than waiting on the state row forever.
func TestGateRecoversFromZombieLeader(t *testing.T) {
	ctx := context.Background()
	pool := freshDB(t)

	const lockKey = 424242

	m, err := migrate.Postgres(pool, migrate.Config{
		Source:          testdataSource(),
		InstanceID:      "survivor",
		AdvisoryLockKey: lockKey,
		PollInterval:    50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("migrate.Postgres: %v", err)
	}

	// First gate bootstraps the bookkeeping tables and applies everything.
	if err := m.RunStartupGate(ctx); err != nil {
		t.Fatalf("initial gate: %v", err)
	}

	// Simulate the zombie: another session holds the lock and dies without
	// cleaning up migration_state.
	zombie, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire zombie conn: %v", err)
	}
	var locked bool
	if err := zombie.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&locked); err != nil || !locked {
		t.Fatalf("zombie lock: locked=%v err=%v", locked, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "_xqua_".migration_state SET status = 'running', holder_id = 'zombie' WHERE id = 1`); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	// Start the gate while the zombie still holds the lock; it must wait.
	gateErr := make(chan error, 1)
	go func() { gateErr <- m.RunStartupGate(ctx) }()

	select {
	case err := <-gateErr:
		t.Fatalf("gate finished while lock was held: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	// Kill the zombie session; Postgres frees the advisory lock, but
	// migration_state stays "running".
	if _, err := zombie.Exec(ctx, `SELECT pg_advisory_unlock($1)`, lockKey); err != nil {
		t.Fatalf("release zombie lock: %v", err)
	}
	zombie.Release()

	select {
	case err := <-gateErr:
		if err != nil {
			t.Fatalf("gate after zombie death: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("gate did not recover from zombie leader")
	}

	// The surviving leader must have reset the state row.
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM "_xqua_".migration_state WHERE id = 1`).Scan(&status); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if status != "idle" {
		t.Fatalf("expected idle after recovery, got %q", status)
	}
}

func TestLeaderFollower(t *testing.T) {
	ctx := context.Background()
	db := freshDB(t)

	leader := newMigrator(t, db, testdataSource())
	follower := newMigrator(t, db, testdataSource())

	errCh := make(chan error, 2)
	go func() { errCh <- leader.RunStartupGate(ctx) }()
	go func() { errCh <- follower.RunStartupGate(ctx) }()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("gate %d: %v", i, err)
		}
	}

	applied, pending, err := leader.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(applied) != 2 || len(pending) != 0 {
		t.Fatalf("expected 2 applied 0 pending, got %d applied %d pending", len(applied), len(pending))
	}
}

func copyTestdata(t *testing.T, dst string) {
	t.Helper()
	entries, err := os.ReadDir("testdata/migrations")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join("testdata/migrations", e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
