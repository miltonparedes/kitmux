package data

import (
	"github.com/miltonparedes/kitmux/internal/tmux"
	wsreg "github.com/miltonparedes/kitmux/internal/workspaces"
)

// Snapshot is the bundle of data consumed by the dashboard on each refresh
// tick. It is intentionally dumb: just raw state, no UI concerns.
type Snapshot struct {
	Workspaces   []wsreg.Workspace
	ActivePaths  map[string]int64 // workspace_path → last activity epoch
	Sessions     []tmux.Session
	SessionRoots map[string]string // session name → repo root
	Panes        []tmux.Pane
	CachedStats  map[string]WorkspaceStats
}

// LoadSnapshot gathers a fresh snapshot of tmux + the persisted workspace
// registry + cached stats. It does not call `wt list`; that happens
// asynchronously via StatsService.
func LoadSnapshot(stats *StatsService) (Snapshot, error) {
	sessions, _ := tmux.ListSessions()
	panes, _ := tmux.ListPanes()
	sessionRoots := ResolveRepoRoots(sessions)

	activePaths := make(map[string]int64)
	for _, s := range sessions {
		root, ok := sessionRoots[s.Name]
		if !ok {
			continue
		}
		if s.Activity > activePaths[root] {
			activePaths[root] = s.Activity
		}
	}

	workspaces := wsreg.LoadRegistry()
	wsreg.SortWorkspaces(workspaces, activePaths)

	cached, err := stats.LoadAllCached()
	if err != nil {
		cached = make(map[string]WorkspaceStats)
	}

	return Snapshot{
		Workspaces:   workspaces,
		ActivePaths:  activePaths,
		Sessions:     sessions,
		SessionRoots: sessionRoots,
		Panes:        panes,
		CachedStats:  cached,
	}, nil
}
