package data

import (
	"sync"
	"time"

	"github.com/miltonparedes/kitmux/internal/store"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// fetchFunc fetches live worktrees for a given workspace path. Injected so
// tests can avoid calling the real `wt` binary.
type fetchFunc func(workspacePath string) ([]worktree.Worktree, error)

// StatsService caches workspace stats in SQLite and refreshes them on demand
// with single-flight coalescing so concurrent callers share one `wt list` run.
type StatsService struct {
	fetch fetchFunc

	mu       sync.Mutex
	inflight map[string]*refreshCall
}

type refreshCall struct {
	done chan struct{}
	res  RefreshResult
}

// RefreshResult is returned by Refresh. Err is populated when the live
// fetch fails; cached data from a previous run may still be available via
// LoadCached.
type RefreshResult struct {
	WorkspacePath string
	Stats         WorkspaceStats
	Err           error
}

// NewStatsService builds a service backed by `wt list`.
func NewStatsService() *StatsService {
	return newStatsService(defaultFetch)
}

func newStatsService(fetch fetchFunc) *StatsService {
	return &StatsService{
		fetch:    fetch,
		inflight: make(map[string]*refreshCall),
	}
}

func defaultFetch(workspacePath string) ([]worktree.Worktree, error) {
	return worktree.ListInDir(workspacePath)
}

// LoadCached returns the persisted stats for a workspace without hitting
// the filesystem. Missing entries return a zero-value WorkspaceStats.
func (s *StatsService) LoadCached(workspacePath string) (WorkspaceStats, error) {
	rows, err := store.LoadWorkspaceStats(workspacePath)
	if err != nil {
		return WorkspaceStats{Path: workspacePath}, err
	}
	return toWorkspaceStats(workspacePath, rows), nil
}

// LoadAllCached returns cached stats for every workspace path recorded in
// SQLite. Missing workspaces yield a zero-value entry when queried via
// LoadCached instead.
func (s *StatsService) LoadAllCached() (map[string]WorkspaceStats, error) {
	all, err := store.LoadAllWorkspaceStats()
	if err != nil {
		return nil, err
	}
	out := make(map[string]WorkspaceStats, len(all))
	for path, rows := range all {
		out[path] = toWorkspaceStats(path, rows)
	}
	return out, nil
}

// Refresh runs the live fetch for a workspace, persists the result, and
// returns it. Concurrent calls for the same workspace join the same flight.
func (s *StatsService) Refresh(workspacePath string) RefreshResult {
	s.mu.Lock()
	if call, ok := s.inflight[workspacePath]; ok {
		s.mu.Unlock()
		<-call.done
		return call.res
	}
	call := &refreshCall{done: make(chan struct{})}
	s.inflight[workspacePath] = call
	s.mu.Unlock()

	call.res = s.doRefresh(workspacePath)

	s.mu.Lock()
	delete(s.inflight, workspacePath)
	s.mu.Unlock()
	close(call.done)
	return call.res
}

func (s *StatsService) doRefresh(workspacePath string) RefreshResult {
	now := time.Now()
	wts, err := s.fetch(workspacePath)
	if err != nil {
		cached, _ := s.LoadCached(workspacePath)
		return RefreshResult{WorkspacePath: workspacePath, Stats: cached, Err: err}
	}

	rows := make([]store.WorktreeStat, 0, len(wts))
	for _, wt := range wts {
		rows = append(rows, store.WorktreeStat{
			WorkspacePath: workspacePath,
			Branch:        wt.Branch,
			WorktreePath:  wt.Path,
			Added:         wt.WorkingTree.Diff.Added,
			Deleted:       wt.WorkingTree.Diff.Deleted,
			Staged:        wt.WorkingTree.Staged,
			Modified:      wt.WorkingTree.Modified,
			Untracked:     wt.WorkingTree.Untracked,
			Ahead:         wt.Remote.Ahead,
			Behind:        wt.Remote.Behind,
			IsMain:        wt.IsMain,
			CommitSHA:     wt.Commit.SHA,
			CommitTS:      wt.Commit.Timestamp,
		})
	}
	if err := store.ReplaceWorkspaceStats(workspacePath, rows, now); err != nil {
		cached, _ := s.LoadCached(workspacePath)
		return RefreshResult{WorkspacePath: workspacePath, Stats: cached, Err: err}
	}

	fresh, _ := s.LoadCached(workspacePath)
	return RefreshResult{WorkspacePath: workspacePath, Stats: fresh}
}

// Invalidate removes the cached stats for a workspace, forcing the next
// Refresh to bypass any freshness check.
func (s *StatsService) Invalidate(workspacePath string) error {
	return store.PurgeWorkspaceStats(workspacePath)
}

// LastRefresh reports when the stats for a workspace were last refreshed,
// returning zero time when unknown.
func (s *StatsService) LastRefresh(workspacePath string) (time.Time, error) {
	meta, err := store.LoadWorkspaceMeta(workspacePath)
	if err != nil {
		return time.Time{}, err
	}
	return meta.LastStatsRefresh, nil
}

func toWorkspaceStats(path string, rows []store.WorktreeStat) WorkspaceStats {
	ws := WorkspaceStats{Path: path, Worktrees: make([]WorktreeStat, 0, len(rows))}
	for _, r := range rows {
		ws.Worktrees = append(ws.Worktrees, WorktreeStat{
			Branch:       r.Branch,
			WorktreePath: r.WorktreePath,
			Added:        r.Added,
			Deleted:      r.Deleted,
			Staged:       r.Staged,
			Modified:     r.Modified,
			Untracked:    r.Untracked,
			Ahead:        r.Ahead,
			Behind:       r.Behind,
			IsMain:       r.IsMain,
			CommitSHA:    r.CommitSHA,
			CommitTS:     r.CommitTS,
			UpdatedAt:    r.UpdatedAt,
		})
		if r.UpdatedAt.After(ws.UpdatedAt) {
			ws.UpdatedAt = r.UpdatedAt
		}
	}
	return ws
}
