package store

import (
	"fmt"
	"time"
)

// PathRepoRoot is a path→repo_root mapping backed by the repo_roots table.
// We repurpose the session_name column as a stable path key so we don't need
// yet another table; the column already has a PK, which is what we want.
type PathRepoRoot struct {
	Path        string
	RepoRoot    string
	RefreshedAt time.Time
}

// LoadRepoRootCache returns every cached path→root mapping.
func LoadRepoRootCache() (map[string]PathRepoRoot, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT session_name, repo_root, refreshed_at FROM repo_roots`)
	if err != nil {
		return nil, fmt.Errorf("query repo_root cache: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]PathRepoRoot)
	for rows.Next() {
		var path, root string
		var refreshed int64
		if err := rows.Scan(&path, &root, &refreshed); err != nil {
			return nil, fmt.Errorf("scan repo_root cache: %w", err)
		}
		entry := PathRepoRoot{Path: path, RepoRoot: root}
		if refreshed > 0 {
			entry.RefreshedAt = time.Unix(0, refreshed)
		}
		out[path] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo_root cache: %w", err)
	}
	return out, nil
}

// SaveRepoRoots upserts a batch of mappings, stamping them as refreshed
// at `at`.
func SaveRepoRoots(mappings map[string]string, at time.Time) error {
	if len(mappings) == 0 {
		return nil
	}
	db, err := open()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin save repo_roots: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`INSERT INTO repo_roots(session_name, repo_root, refreshed_at)
		VALUES(?, ?, ?)
		ON CONFLICT(session_name) DO UPDATE SET
			repo_root = excluded.repo_root,
			refreshed_at = excluded.refreshed_at`)
	if err != nil {
		return fmt.Errorf("prepare upsert repo_root: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	ts := at.UnixNano()
	for path, root := range mappings {
		if path == "" || root == "" {
			continue
		}
		if _, err := stmt.Exec(path, root, ts); err != nil {
			return fmt.Errorf("upsert repo_root %q: %w", path, err)
		}
	}
	return tx.Commit()
}
