package data

import "time"

// WorktreeStat is a single-worktree snapshot used by the dashboard UI.
// Mirrors the columns of workspace_stats plus in-memory helpers.
type WorktreeStat struct {
	Branch       string
	WorktreePath string
	Added        int
	Deleted      int
	Staged       bool
	Modified     bool
	Untracked    bool
	Ahead        int
	Behind       int
	IsMain       bool
	CommitSHA    string
	CommitTS     int64
	UpdatedAt    time.Time
}

// Dirty reports whether the worktree has any local modifications.
func (s WorktreeStat) Dirty() bool {
	return s.Added > 0 || s.Deleted > 0 || s.Staged || s.Modified || s.Untracked
}

// WorkspaceStats holds the cached stats for a single workspace.
type WorkspaceStats struct {
	Path      string
	Worktrees []WorktreeStat
	UpdatedAt time.Time // most-recent worktree UpdatedAt (epoch if missing)
}

// TotalDiff returns summed +/- across all worktrees in the workspace.
func (w WorkspaceStats) TotalDiff() (added, deleted int) {
	for _, wt := range w.Worktrees {
		added += wt.Added
		deleted += wt.Deleted
	}
	return added, deleted
}
