package store

import (
	"database/sql"
	"os"
	"testing"
)

func TestOpen_CreatesStateDB(t *testing.T) {
	home := useTempHome(t)

	db, err := open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

	tables := []string{"metadata", "projects", "session_snapshots", "repo_roots", "worktree_stats"}
	for _, table := range tables {
		assertTableExists(t, db, table)
	}

	var version int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		t.Fatalf("query user_version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("expected schema version %d, got %d", schemaVersion, version)
	}
}

func TestOpen_IsIdempotent(t *testing.T) {
	useTempHome(t)

	db1, err := open()
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_ = db1.Close()

	db2, err := open()
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer func() { _ = db2.Close() }()

	var version int
	if err := db2.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		t.Fatalf("query user_version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("expected schema version %d, got %d", schemaVersion, version)
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
