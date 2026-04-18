package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	// Register the pure-Go SQLite driver.
	_ "modernc.org/sqlite"
)

const (
	configDir    = ".config/kitmux"
	databaseFile = "state.db"
)

// migration is a function that applies a single schema change within a transaction.
type migration func(tx *sql.Tx) error

// migrations is the ordered list of schema migrations.
// The schema version equals len(migrations) — adding a new entry auto-bumps it.
var migrations = []migration{migrateV1, migrateV2, migrateV3, migrateV4}

func schemaVersion() int { return len(migrations) }

// DBPath returns the absolute path to the SQLite state database.
func DBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, configDir, databaseFile), nil
}

// Cached singleton connection. Opening SQLite + running migrations costs
// tens of ms on the hot path, so we reuse a single *sql.DB for the process.
// The home directory is baked into the first-open key; if tests switch HOME
// between runs they reset the singleton via ResetForTests.
var (
	dbMu      sync.Mutex
	dbInst    *sql.DB
	dbHome    string
	errOpenDB error
)

// ResetForTests clears the cached connection. Only call from tests that
// change HOME mid-process.
func ResetForTests() {
	dbMu.Lock()
	defer dbMu.Unlock()
	if dbInst != nil {
		_ = dbInst.Close()
	}
	dbInst = nil
	dbHome = ""
	errOpenDB = nil
}

func open() (*sql.DB, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home dir: %w", err)
	}
	// If HOME changed (tests), drop the cached connection and reopen.
	if dbInst != nil && dbHome != home {
		_ = dbInst.Close()
		dbInst = nil
		dbHome = ""
		errOpenDB = nil
	}
	if dbInst != nil {
		return dbInst, nil
	}
	if errOpenDB != nil {
		return nil, errOpenDB
	}

	db, err := openDB()
	if err != nil {
		errOpenDB = err
		return nil, err
	}
	dbInst = db
	dbHome = home
	return db, nil
}

func openDB() (*sql.DB, error) {
	path, err := DBPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := configure(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func configure(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA foreign_keys = ON;",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("configure sqlite (%s): %w", pragma, err)
		}
	}
	return nil
}

func migrate(db *sql.DB) error {
	target := schemaVersion()

	var current int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&current); err != nil {
		return fmt.Errorf("read sqlite schema version: %w", err)
	}
	if current > target {
		return fmt.Errorf("sqlite schema version %d is newer than supported version %d", current, target)
	}
	if current == target {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin sqlite migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i := current; i < target; i++ {
		if err := migrations[i](tx); err != nil {
			return fmt.Errorf("apply sqlite migration v%d: %w", i+1, err)
		}
	}

	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d;", target)); err != nil {
		return fmt.Errorf("set sqlite schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration: %w", err)
	}
	return nil
}

func migrateV2(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE projects RENAME TO workspaces;`,
		`DROP INDEX IF EXISTS idx_projects_name;`,
		`CREATE INDEX idx_workspaces_name ON workspaces(name);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("v2: %w", err)
		}
	}
	return nil
}

// migrateV4 introduces a dedicated table for the workspace dashboard's
// path→repo_root cache. Previously this cache was stored in the `repo_roots`
// table by repurposing its session_name column as a path key, which meant
// SaveSessionCache's blanket DELETE wiped the workspace entries alongside
// its own session-name-keyed rows.
func migrateV4(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE workspace_repo_roots (
			path TEXT PRIMARY KEY,
			repo_root TEXT NOT NULL,
			refreshed_at INTEGER NOT NULL
		);`,
		`CREATE INDEX idx_workspace_repo_roots_repo_root ON workspace_repo_roots(repo_root);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("v4: %w", err)
		}
	}
	return nil
}

func migrateV3(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE workspace_stats (
			workspace_path TEXT NOT NULL,
			branch TEXT NOT NULL,
			worktree_path TEXT NOT NULL,
			added INTEGER NOT NULL DEFAULT 0,
			deleted INTEGER NOT NULL DEFAULT 0,
			staged INTEGER NOT NULL DEFAULT 0,
			modified INTEGER NOT NULL DEFAULT 0,
			untracked INTEGER NOT NULL DEFAULT 0,
			ahead INTEGER NOT NULL DEFAULT 0,
			behind INTEGER NOT NULL DEFAULT 0,
			is_main INTEGER NOT NULL DEFAULT 0,
			commit_sha TEXT NOT NULL DEFAULT '',
			commit_ts INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (workspace_path, worktree_path)
		);`,
		`CREATE INDEX idx_workspace_stats_path ON workspace_stats(workspace_path);`,
		`CREATE TABLE workspace_meta (
			workspace_path TEXT PRIMARY KEY,
			last_stats_refresh INTEGER NOT NULL DEFAULT 0,
			last_opened_at INTEGER NOT NULL DEFAULT 0
		);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("v3: %w", err)
		}
	}
	return nil
}

func migrateV1(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE projects (
			path TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			added_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX idx_projects_name ON projects(name);`,
		`CREATE TABLE session_snapshots (
			session_name TEXT PRIMARY KEY,
			path TEXT,
			windows INTEGER NOT NULL,
			attached INTEGER NOT NULL,
			activity INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE INDEX idx_session_snapshots_path ON session_snapshots(path);`,
		`CREATE TABLE repo_roots (
			session_name TEXT PRIMARY KEY,
			repo_root TEXT NOT NULL,
			refreshed_at INTEGER NOT NULL
		);`,
		`CREATE INDEX idx_repo_roots_repo_root ON repo_roots(repo_root);`,
		`CREATE TABLE worktree_stats (
			session_name TEXT PRIMARY KEY,
			repo_root TEXT NOT NULL DEFAULT '',
			added INTEGER NOT NULL,
			deleted INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("v1: %w", err)
		}
	}
	return nil
}
