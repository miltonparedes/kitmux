package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const legacyProjectsFile = "projects.json"

// Project is the persisted project record.
type Project struct {
	Name       string
	Path       string
	AddedAt    int64
	LastSeenAt int64
}

func LoadProjects() ([]Project, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacyProjects(db); err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT name, path, added_at, last_seen_at FROM projects ORDER BY name, path`)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Name, &p.Path, &p.AddedAt, &p.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

func SaveProjects(projects []Project) error {
	db, err := open()
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin save projects: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM projects`); err != nil {
		return fmt.Errorf("clear projects: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO projects(path, name, added_at, last_seen_at) VALUES(?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert project: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, p := range projects {
		lastSeenAt := p.LastSeenAt
		if lastSeenAt == 0 {
			lastSeenAt = p.AddedAt
		}
		if _, err := stmt.Exec(p.Path, p.Name, p.AddedAt, lastSeenAt); err != nil {
			return fmt.Errorf("insert project %q: %w", p.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save projects: %w", err)
	}
	return nil
}

func AddProject(name, path string) (bool, error) {
	db, err := open()
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacyProjects(db); err != nil {
		return false, err
	}

	var exists int
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE path = ?)`, path).Scan(&exists); err != nil {
		return false, fmt.Errorf("check project path: %w", err)
	}
	if exists == 1 {
		return false, nil
	}

	now := time.Now().Unix()
	if _, err := db.Exec(`INSERT INTO projects(path, name, added_at, last_seen_at) VALUES(?, ?, ?, ?)`, path, name, now, now); err != nil {
		return false, fmt.Errorf("insert project %q: %w", path, err)
	}
	return true, nil
}

func RemoveProject(name string) (bool, error) {
	db, err := open()
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacyProjects(db); err != nil {
		return false, err
	}

	result, err := db.Exec(`DELETE FROM projects WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("delete project %q: %w", name, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected removing project %q: %w", name, err)
	}
	return rows > 0, nil
}

func HasProjectPath(path string) (bool, error) {
	db, err := open()
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacyProjects(db); err != nil {
		return false, err
	}

	var exists int
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE path = ?)`, path).Scan(&exists); err != nil {
		return false, fmt.Errorf("check project path: %w", err)
	}
	return exists == 1, nil
}

func importLegacyProjects(db *sql.DB) error {
	empty, err := tableEmpty(db, "projects")
	if err != nil || !empty {
		return err
	}

	data, err := os.ReadFile(legacyProjectsPath()) //nolint:gosec // path derives from the user's home dir
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read legacy projects: %w", err)
	}

	var payload struct {
		Projects []Project `json:"projects"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	if len(payload.Projects) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin import legacy projects: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO projects(path, name, added_at, last_seen_at) VALUES(?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare import legacy projects: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, p := range payload.Projects {
		lastSeenAt := p.LastSeenAt
		if lastSeenAt == 0 {
			lastSeenAt = p.AddedAt
		}
		if _, err := stmt.Exec(p.Path, p.Name, p.AddedAt, lastSeenAt); err != nil {
			return fmt.Errorf("import legacy project %q: %w", p.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit import legacy projects: %w", err)
	}
	return nil
}

func legacyProjectsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, legacyProjectsFile)
}
