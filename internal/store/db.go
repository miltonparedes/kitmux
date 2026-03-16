package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	// Register the pure-Go SQLite driver.
	_ "modernc.org/sqlite"
)

const (
	configDir     = ".config/kitmux"
	databaseFile  = "state.db"
	schemaVersion = 1
)

func open() (*sql.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home dir: %w", err)
	}

	path := filepath.Join(home, configDir, databaseFile)
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
	var version int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		return fmt.Errorf("read sqlite schema version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("sqlite schema version %d is newer than supported version %d", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin sqlite migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS projects (
			path TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			added_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);`,
		`CREATE TABLE IF NOT EXISTS session_snapshots (
			session_name TEXT PRIMARY KEY,
			path TEXT,
			windows INTEGER NOT NULL,
			attached INTEGER NOT NULL,
			activity INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_session_snapshots_path ON session_snapshots(path);`,
		`CREATE TABLE IF NOT EXISTS repo_roots (
			session_name TEXT PRIMARY KEY,
			repo_root TEXT NOT NULL,
			refreshed_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_roots_repo_root ON repo_roots(repo_root);`,
		`CREATE TABLE IF NOT EXISTS worktree_stats (
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
			return fmt.Errorf("apply sqlite migration: %w", err)
		}
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d;", schemaVersion)); err != nil {
		return fmt.Errorf("set sqlite schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration: %w", err)
	}
	return nil
}
