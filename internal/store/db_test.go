package store

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
)

func TestOpen_CreatesStateDB(t *testing.T) {
	home := useTempHome(t)

	if _, err := open(); err != nil {
		t.Fatalf("open: %v", err)
	}

	if _, err := os.Stat(stateDBPath(home)); err != nil {
		t.Fatalf("stat state db: %v", err)
	}
}

func TestOpen_RunsMigrations(t *testing.T) {
	useTempHome(t)

	db, err := open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	tables := []string{
		"metadata",
		"workspaces",
		"session_snapshots",
		"repo_roots",
		"worktree_stats",
		"workspace_stats",
		"workspace_meta",
		"workspace_repo_roots",
	}
	for _, table := range tables {
		assertTableExists(t, db, table)
	}

	var version int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		t.Fatalf("query user_version: %v", err)
	}
	if version != schemaVersion() {
		t.Fatalf("expected schema version %d, got %d", schemaVersion(), version)
	}
}

func TestOpen_IsIdempotent(t *testing.T) {
	useTempHome(t)

	db1, err := open()
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	db2, err := open()
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	if db1 != db2 {
		t.Fatal("expected singleton to return the same *sql.DB instance")
	}

	var version int
	if err := db2.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		t.Fatalf("query user_version: %v", err)
	}
	if version != schemaVersion() {
		t.Fatalf("expected schema version %d, got %d", schemaVersion(), version)
	}
}

func TestMigrate_RejectsNewerVersion(t *testing.T) {
	useTempHome(t)

	// Create a DB and set user_version higher than supported.
	db, err := open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	future := schemaVersion() + 1
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d;", future)); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	// Drop the cached connection so the next open() re-runs migrations.
	ResetForTests()

	// Re-opening should fail.
	_, err = open()
	if err == nil {
		t.Fatal("expected error opening DB with newer schema version")
	}
	want := fmt.Sprintf("sqlite schema version %d is newer than supported version %d", future, schemaVersion())
	if err.Error() != want {
		t.Fatalf("unexpected error:\n got: %s\nwant: %s", err.Error(), want)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var found string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&found)
	if err != nil {
		t.Fatalf("query table %s: %v", name, err)
	}
	if found != name {
		t.Fatalf("expected table %s, got %s", name, found)
	}
}
