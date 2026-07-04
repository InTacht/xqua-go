package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func TestDiscoverOrdering(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir,
		"000002_second.up.sql",
		"000002_second.down.sql",
		"000001_first.up.sql",
		"000001_first.down.sql",
		"000010_tenth.up.sql",
	)

	migrations, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migrations) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(migrations))
	}

	wantVersions := []int64{1, 2, 10}
	for i, want := range wantVersions {
		if migrations[i].Version != want {
			t.Fatalf("migration %d: want version %d, got %d", i, want, migrations[i].Version)
		}
	}

	if migrations[0].Name != "first" || migrations[0].UpPath != "000001_first.up.sql" {
		t.Fatalf("unexpected first migration: %+v", migrations[0])
	}
	if migrations[0].DownPath != "000001_first.down.sql" {
		t.Fatalf("expected down path for first migration, got %q", migrations[0].DownPath)
	}
	if migrations[2].DownPath != "" {
		t.Fatalf("expected no down path for tenth migration, got %q", migrations[2].DownPath)
	}
}

func TestDiscoverIgnoresNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir,
		"000001_ok.up.sql",
		"README.md",
		"1_bad.up.sql",
		"000001_ok.notsql",
		"000002_Bad_Case.up.sql",
	)

	migrations, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migrations) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(migrations))
	}
}

func TestDiscoverMissingUpFails(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "000001_only_down.down.sql")

	if _, err := Discover(dir); err == nil {
		t.Fatal("expected error for missing up file")
	}
}

func TestDiscoverConflictingNamesFails(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir,
		"000001_alpha.up.sql",
		"000001_beta.down.sql",
	)

	if _, err := Discover(dir); err == nil {
		t.Fatal("expected error for conflicting names")
	}
}

func TestFormatVersion(t *testing.T) {
	if got := FormatVersion(1); got != "000001" {
		t.Fatalf("want 000001, got %q", got)
	}
	if got := FormatVersion(123456); got != "123456" {
		t.Fatalf("want 123456, got %q", got)
	}
}

func TestSourceReadUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001_first.up.sql"), []byte("CREATE TABLE t ();"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	src := Source{Dir: dir}
	migrations, err := src.Discover()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	content, err := src.ReadUp(migrations[0])
	if err != nil {
		t.Fatalf("read up: %v", err)
	}
	if content != "CREATE TABLE t ();" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestSourceReadDownMissing(t *testing.T) {
	src := Source{Dir: t.TempDir()}
	_, err := src.ReadDown(Migration{Version: 1, Name: "x"})
	if err == nil {
		t.Fatal("expected error reading missing down file")
	}
}
