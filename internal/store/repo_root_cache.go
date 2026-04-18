package store

import (
	"fmt"
	"time"
)

// PathRepoRoot is a path→repo_root mapping persisted in the
// workspace_repo_roots table. Used by the workspaces dashboard so that
// resolving repo roots for cached workspaces costs zero git calls when
// nothing has changed.
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

	rows, err := db.Query(`SELECT path, repo_root, refreshed_at FROM workspace_repo_roots`)
	if err != nil {
		return nil, fmt.Errorf("query workspace_repo_roots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]PathRepoRoot)
	for rows.Next() {
		var path, root string
		var refreshed int64
		if err := rows.Scan(&path, &root, &refreshed); err != nil {
			return nil, fmt.Errorf("scan workspace_repo_roots: %w", err)
		}
		entry := PathRepoRoot{Path: path, RepoRoot: root}
		if refreshed > 0 {
			entry.RefreshedAt = time.Unix(0, refreshed)
		}
		out[path] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace_repo_roots: %w", err)
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
		return fmt.Errorf("begin save workspace_repo_roots: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`INSERT INTO workspace_repo_roots(path, repo_root, refreshed_at)
		VALUES(?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			repo_root = excluded.repo_root,
			refreshed_at = excluded.refreshed_at`)
	if err != nil {
		return fmt.Errorf("prepare upsert workspace_repo_roots: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	ts := at.UnixNano()
	for path, root := range mappings {
		if path == "" || root == "" {
			continue
		}
		if _, err := stmt.Exec(path, root, ts); err != nil {
			return fmt.Errorf("upsert workspace_repo_roots %q: %w", path, err)
		}
	}
	return tx.Commit()
}
