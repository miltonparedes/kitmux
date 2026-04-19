package workspaces

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/tmux"
	wsreg "github.com/miltonparedes/kitmux/internal/workspaces"
	wsdata "github.com/miltonparedes/kitmux/internal/workspaces/data"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// loadDataCmd returns the initial snapshot command.
func loadDataCmd(svc *wsdata.StatsService) tea.Cmd {
	return func() tea.Msg { return loadInitialSnapshot(svc) }
}

// loadInitialSnapshot performs a fast, non-blocking snapshot: tmux state,
// the workspace registry, and — crucially — cached worktrees/stats from
// SQLite. It does not call `wt list`. The fresh refresh happens afterwards
// via refreshAllStatsCmd, which dispatches one goroutine per workspace.
func loadInitialSnapshot(svc *wsdata.StatsService) tea.Msg {
	sess, _ := tmux.ListSessions()
	panes, _ := tmux.ListPanes()
	projs := wsreg.LoadRegistry()

	repoRoots := wsdata.ResolveRepoRoots(sess)

	activePaths := make(map[string]int64)
	for _, s := range sess {
		if root, ok := repoRoots[s.Name]; ok {
			if s.Activity > activePaths[root] {
				activePaths[root] = s.Activity
			}
		}
	}

	wsreg.SortWorkspaces(projs, activePaths)

	var cached map[string]wsdata.WorkspaceStats
	if svc != nil {
		if c, err := svc.LoadAllCached(); err == nil {
			cached = c
		}
	}
	wtByPath := worktreesFromCache(projs, cached)

	entries := make([]workspaceEntry, len(projs))
	for i, p := range projs {
		act := activePaths[p.Path]
		entries[i] = workspaceEntry{
			Name:     p.Name,
			Path:     p.Path,
			Active:   act > 0,
			Activity: act,
		}
	}

	return dataLoadedMsg{
		workspaces: entries,
		sessions:   sess,
		repoRoots:  repoRoots,
		wtByPath:   wtByPath,
		panes:      panes,
	}
}

// worktreesFromCache converts cached stats into the light worktree shape the
// UI already understands. For workspaces with no cached stats the map simply
// omits that path; the UI falls back to "just the main session" later when
// the background refresh arrives.
func worktreesFromCache(projs []wsreg.Workspace, cached map[string]wsdata.WorkspaceStats) map[string][]worktree.Worktree {
	if len(cached) == 0 {
		return make(map[string][]worktree.Worktree, len(projs))
	}
	out := make(map[string][]worktree.Worktree, len(projs))
	for _, p := range projs {
		ws, ok := cached[p.Path]
		if !ok || len(ws.Worktrees) == 0 {
			continue
		}
		out[p.Path] = worktreesFromStats(ws)
	}
	return out
}

// worktreesFromStats converts a single WorkspaceStats into []worktree.Worktree.
func worktreesFromStats(ws wsdata.WorkspaceStats) []worktree.Worktree {
	list := make([]worktree.Worktree, 0, len(ws.Worktrees))
	for _, wt := range ws.Worktrees {
		list = append(list, worktree.Worktree{
			Branch: wt.Branch,
			Path:   wt.WorktreePath,
			IsMain: wt.IsMain,
			Commit: worktree.Commit{
				SHA:       wt.CommitSHA,
				Timestamp: wt.CommitTS,
			},
			WorkingTree: worktree.WorkingTree{
				Staged:    wt.Staged,
				Modified:  wt.Modified,
				Untracked: wt.Untracked,
				Diff:      worktree.Diff{Added: wt.Added, Deleted: wt.Deleted},
			},
			Remote: worktree.Remote{Ahead: wt.Ahead, Behind: wt.Behind},
		})
	}
	return list
}

// refreshStatsCmd triggers an async refresh for a single workspace.
func refreshStatsCmd(svc *wsdata.StatsService, workspacePath string) tea.Cmd {
	if svc == nil || workspacePath == "" {
		return nil
	}
	return func() tea.Msg {
		res := svc.Refresh(workspacePath)
		return statsLoadedMsg{
			wsStats:  map[string]wsdata.WorkspaceStats{workspacePath: res.Stats},
			stats:    flattenSessionStats(res.Stats, nil),
			refresh:  time.Now(),
			workPath: workspacePath,
		}
	}
}

// refreshAllStatsCmd fans out one goroutine per workspace so `wt list` calls
// run in parallel. Results are collected behind a mutex and delivered as a
// single statsLoadedMsg — we intentionally avoid dispatching one Bubble Tea
// message per workspace because the UI re-renders on every message and that
// creates visible flicker with large workspace lists.
func refreshAllStatsCmd(svc *wsdata.StatsService, workspaces []workspaceEntry) tea.Cmd {
	if svc == nil || len(workspaces) == 0 {
		return nil
	}
	paths := make([]string, 0, len(workspaces))
	for _, w := range workspaces {
		paths = append(paths, w.Path)
	}
	return func() tea.Msg {
		var (
			mu sync.Mutex
			wg sync.WaitGroup
		)
		ws := make(map[string]wsdata.WorkspaceStats, len(paths))
		for _, path := range paths {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res := svc.Refresh(path)
				mu.Lock()
				ws[path] = res.Stats
				mu.Unlock()
			}()
		}
		wg.Wait()
		return statsLoadedMsg{
			wsStats: ws,
			stats:   flattenAllSessionStats(ws),
			refresh: time.Now(),
		}
	}
}

// flattenSessionStats converts a WorkspaceStats into a per-session map using
// the provided path→session-name index. When index is nil the helper still
// returns a populated-by-path map, which callers may key differently.
func flattenSessionStats(ws wsdata.WorkspaceStats, pathToSession map[string]string) map[string]sessionStats {
	out := make(map[string]sessionStats, len(ws.Worktrees))
	for _, wt := range ws.Worktrees {
		if wt.Added == 0 && wt.Deleted == 0 {
			continue
		}
		name := pathToSession[wt.WorktreePath]
		if name == "" {
			name = wt.WorktreePath
		}
		out[name] = sessionStats{Added: wt.Added, Deleted: wt.Deleted}
	}
	return out
}

func flattenAllSessionStats(all map[string]wsdata.WorkspaceStats) map[string]sessionStats {
	out := make(map[string]sessionStats)
	for _, ws := range all {
		for _, wt := range ws.Worktrees {
			if wt.Added == 0 && wt.Deleted == 0 {
				continue
			}
			out[wt.WorktreePath] = sessionStats{Added: wt.Added, Deleted: wt.Deleted}
		}
	}
	return out
}

// loadZoxide queries zoxide for recent directories. Errors surface as an
// empty result; the caller can decide to show a toast.
func loadZoxide() tea.Cmd {
	return func() tea.Msg {
		entries, _ := queryZoxide()
		return zoxideLoadedMsg{entries: entries}
	}
}

func queryZoxide() ([]zoxideEntry, error) {
	out, err := exec.Command("zoxide", "query", "-ls").Output()
	if err != nil {
		return nil, err
	}
	home, _ := os.UserHomeDir()
	var entries []zoxideEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		line = strings.TrimSpace(line)
		idx := strings.IndexByte(line, ' ')
		if idx < 0 {
			continue
		}
		scoreStr := line[:idx]
		path := strings.TrimSpace(line[idx+1:])
		score, _ := strconv.ParseFloat(scoreStr, 64)
		short := path
		if home != "" && strings.HasPrefix(path, home) {
			short = "~" + path[len(home):]
		}
		entries = append(entries, zoxideEntry{Score: score, Path: path, Short: short})
	}
	return entries, nil
}
