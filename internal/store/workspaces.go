package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const legacyWorkspacesFile = "projects.json"

// Workspace is the persisted workspace record.
type Workspace struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	AddedAt    int64  `json:"added_at"`
	LastSeenAt int64  `json:"last_seen_at"`
}

func LoadWorkspaces() ([]Workspace, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacyWorkspaces(db); err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT name, path, added_at, last_seen_at FROM workspaces ORDER BY name, path`)
	if err != nil {
		return nil, fmt.Errorf("query workspaces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var workspaces []Workspace
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.Name, &w.Path, &w.AddedAt, &w.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		workspaces = append(workspaces, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}

	return workspaces, nil
}

func SaveWorkspaces(workspaces []Workspace) error {
	db, err := open()
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin save workspaces: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM workspaces`); err != nil {
		return fmt.Errorf("clear workspaces: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO workspaces(path, name, added_at, last_seen_at) VALUES(?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert workspace: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, w := range workspaces {
		lastSeenAt := w.LastSeenAt
		if lastSeenAt == 0 {
			lastSeenAt = w.AddedAt
		}
		if _, err := stmt.Exec(w.Path, w.Name, w.AddedAt, lastSeenAt); err != nil {
			return fmt.Errorf("insert workspace %q: %w", w.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save workspaces: %w", err)
	}
	return nil
}

func AddWorkspace(name, path string) (bool, error) {
	db, err := open()
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacyWorkspaces(db); err != nil {
		return false, err
	}

	var exists int
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM workspaces WHERE path = ?)`, path).Scan(&exists); err != nil {
		return false, fmt.Errorf("check workspace path: %w", err)
	}
	if exists == 1 {
		return false, nil
	}

	now := time.Now().Unix()
	if _, err := db.Exec(`INSERT INTO workspaces(path, name, added_at, last_seen_at) VALUES(?, ?, ?, ?)`, path, name, now, now); err != nil {
		return false, fmt.Errorf("insert workspace %q: %w", path, err)
	}
	return true, nil
}

func RemoveWorkspace(name string) (bool, error) {
	db, err := open()
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacyWorkspaces(db); err != nil {
		return false, err
	}

	result, err := db.Exec(`DELETE FROM workspaces WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("delete workspace %q: %w", name, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected removing workspace %q: %w", name, err)
	}
	return rows > 0, nil
}

func HasWorkspacePath(path string) (bool, error) {
	db, err := open()
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacyWorkspaces(db); err != nil {
		return false, err
	}

	var exists int
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM workspaces WHERE path = ?)`, path).Scan(&exists); err != nil {
		return false, fmt.Errorf("check workspace path: %w", err)
	}
	return exists == 1, nil
}

func importLegacyWorkspaces(db *sql.DB) error {
	empty, err := tableEmpty(db, tableWorkspaces)
	if err != nil || !empty {
		return err
	}

	data, err := os.ReadFile(legacyWorkspacesPath()) //nolint:gosec // path derives from the user's home dir
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read legacy workspaces: %w", err)
	}

	var payload struct {
		Projects []Workspace `json:"projects"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	if len(payload.Projects) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin import legacy workspaces: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO workspaces(path, name, added_at, last_seen_at) VALUES(?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare import legacy workspaces: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, p := range payload.Projects {
		lastSeenAt := p.LastSeenAt
		if lastSeenAt == 0 {
			lastSeenAt = p.AddedAt
		}
		if _, err := stmt.Exec(p.Path, p.Name, p.AddedAt, lastSeenAt); err != nil {
			return fmt.Errorf("import legacy workspace %q: %w", p.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit import legacy workspaces: %w", err)
	}
	return nil
}

func legacyWorkspacesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, legacyWorkspacesFile)
}
