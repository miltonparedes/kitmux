package store

import (
	"database/sql"
	"fmt"
	"time"
)

// WorktreeStat is a persisted per-worktree snapshot for a workspace.
type WorktreeStat struct {
	WorkspacePath string
	Branch        string
	WorktreePath  string
	Added         int
	Deleted       int
	Staged        bool
	Modified      bool
	Untracked     bool
	Ahead         int
	Behind        int
	IsMain        bool
	CommitSHA     string
	CommitTS      int64
	UpdatedAt     time.Time
}

// WorkspaceMeta holds per-workspace housekeeping timestamps.
type WorkspaceMeta struct {
	WorkspacePath    string
	LastStatsRefresh time.Time
	LastOpenedAt     time.Time
}

// LoadWorkspaceStats returns all cached worktree stats for a workspace path.
func LoadWorkspaceStats(workspacePath string) ([]WorktreeStat, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}

	return loadWorkspaceStats(db, workspacePath)
}

// LoadAllWorkspaceStats returns cached stats for all workspaces, keyed by
// workspace path.
func LoadAllWorkspaceStats() (map[string][]WorktreeStat, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT workspace_path, branch, worktree_path, added, deleted,
		staged, modified, untracked, ahead, behind, is_main,
		commit_sha, commit_ts, updated_at
		FROM workspace_stats`)
	if err != nil {
		return nil, fmt.Errorf("query workspace stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string][]WorktreeStat)
	for rows.Next() {
		stat, err := scanWorktreeStat(rows)
		if err != nil {
			return nil, err
		}
		out[stat.WorkspacePath] = append(out[stat.WorkspacePath], stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace stats: %w", err)
	}
	return out, nil
}

// ReplaceWorkspaceStats atomically replaces all stats for a workspace and
// updates the last_stats_refresh timestamp.
func ReplaceWorkspaceStats(workspacePath string, stats []WorktreeStat, refreshedAt time.Time) error {
	db, err := open()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin replace workspace stats: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM workspace_stats WHERE workspace_path = ?`, workspacePath); err != nil {
		return fmt.Errorf("delete workspace stats: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO workspace_stats(
		workspace_path, branch, worktree_path, added, deleted,
		staged, modified, untracked, ahead, behind, is_main,
		commit_sha, commit_ts, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert workspace stats: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	updatedUnix := refreshedAt.UnixNano()
	for _, s := range stats {
		if _, err := stmt.Exec(
			workspacePath, s.Branch, s.WorktreePath, s.Added, s.Deleted,
			boolToInt(s.Staged), boolToInt(s.Modified), boolToInt(s.Untracked),
			s.Ahead, s.Behind, boolToInt(s.IsMain),
			s.CommitSHA, s.CommitTS, updatedUnix,
		); err != nil {
			return fmt.Errorf("insert workspace stat %q/%q: %w", workspacePath, s.WorktreePath, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO workspace_meta(workspace_path, last_stats_refresh)
		VALUES(?, ?)
		ON CONFLICT(workspace_path) DO UPDATE SET last_stats_refresh = excluded.last_stats_refresh`,
		workspacePath, refreshedAt.UnixNano()); err != nil {
		return fmt.Errorf("upsert workspace meta: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace workspace stats: %w", err)
	}
	return nil
}

// LoadWorkspaceMeta returns meta for a single workspace, or zero-value if
// not present.
func LoadWorkspaceMeta(workspacePath string) (WorkspaceMeta, error) {
	db, err := open()
	if err != nil {
		return WorkspaceMeta{}, err
	}

	return loadWorkspaceMeta(db, workspacePath)
}

// TouchWorkspaceOpened updates the last_opened_at column for a workspace.
func TouchWorkspaceOpened(workspacePath string, at time.Time) error {
	db, err := open()
	if err != nil {
		return err
	}

	if _, err := db.Exec(`INSERT INTO workspace_meta(workspace_path, last_opened_at)
		VALUES(?, ?)
		ON CONFLICT(workspace_path) DO UPDATE SET last_opened_at = excluded.last_opened_at`,
		workspacePath, at.UnixNano()); err != nil {
		return fmt.Errorf("touch workspace opened: %w", err)
	}
	return nil
}

// PurgeWorkspaceStats removes every cached stat and meta row for a workspace.
func PurgeWorkspaceStats(workspacePath string) error {
	db, err := open()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin purge workspace stats: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM workspace_stats WHERE workspace_path = ?`, workspacePath); err != nil {
		return fmt.Errorf("delete workspace stats: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM workspace_meta WHERE workspace_path = ?`, workspacePath); err != nil {
		return fmt.Errorf("delete workspace meta: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit purge workspace stats: %w", err)
	}
	return nil
}

func loadWorkspaceStats(db *sql.DB, workspacePath string) ([]WorktreeStat, error) {
	rows, err := db.Query(`SELECT workspace_path, branch, worktree_path, added, deleted,
		staged, modified, untracked, ahead, behind, is_main,
		commit_sha, commit_ts, updated_at
		FROM workspace_stats WHERE workspace_path = ?`, workspacePath)
	if err != nil {
		return nil, fmt.Errorf("query workspace stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []WorktreeStat
	for rows.Next() {
		stat, err := scanWorktreeStat(rows)
		if err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace stats: %w", err)
	}
	return stats, nil
}

func loadWorkspaceMeta(db *sql.DB, workspacePath string) (WorkspaceMeta, error) {
	var refresh, opened int64
	err := db.QueryRow(`SELECT last_stats_refresh, last_opened_at FROM workspace_meta WHERE workspace_path = ?`,
		workspacePath).Scan(&refresh, &opened)
	if err == sql.ErrNoRows {
		return WorkspaceMeta{WorkspacePath: workspacePath}, nil
	}
	if err != nil {
		return WorkspaceMeta{}, fmt.Errorf("query workspace meta: %w", err)
	}
	meta := WorkspaceMeta{WorkspacePath: workspacePath}
	if refresh > 0 {
		meta.LastStatsRefresh = time.Unix(0, refresh)
	}
	if opened > 0 {
		meta.LastOpenedAt = time.Unix(0, opened)
	}
	return meta, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWorktreeStat(r rowScanner) (WorktreeStat, error) {
	var s WorktreeStat
	var staged, modified, untracked, isMain int
	var updatedAt int64
	if err := r.Scan(
		&s.WorkspacePath, &s.Branch, &s.WorktreePath, &s.Added, &s.Deleted,
		&staged, &modified, &untracked, &s.Ahead, &s.Behind, &isMain,
		&s.CommitSHA, &s.CommitTS, &updatedAt,
	); err != nil {
		return WorktreeStat{}, fmt.Errorf("scan workspace stat: %w", err)
	}
	s.Staged = staged == 1
	s.Modified = modified == 1
	s.Untracked = untracked == 1
	s.IsMain = isMain == 1
	if updatedAt > 0 {
		s.UpdatedAt = time.Unix(0, updatedAt)
	}
	return s, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
