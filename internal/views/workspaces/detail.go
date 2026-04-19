package workspaces

import (
	"sort"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/tmux"
	wsdata "github.com/miltonparedes/kitmux/internal/workspaces/data"
)

// Local aliases so refactored helpers stay concise without widening imports.
type (
	WorkspaceStatsAlias = wsdata.WorkspaceStats
	WorktreeStatAlias   = wsdata.WorktreeStat
)

// rebuildDetail rebuilds the right column (branches + agents) for the
// currently selected workspace.
func (m *Model) rebuildDetail() {
	if len(m.workspaces) == 0 {
		m.branches = nil
		m.agentEntries = nil
		m.detailItems = 0
		return
	}

	ws := m.workspaces[m.wsCursor]
	m.branches = m.buildBranches(ws)
	m.agentEntries = detectAgents(m.panes, ws.Path, m.repoRoots, m.sessions)
	m.detailItems = len(m.branches) + len(m.agentEntries)
	m.detCursor = 0
	m.detScroll = 0
}

// buildBranches merges live sessions and inactive worktrees into the
// ordered list shown in the detail column.
func (m *Model) buildBranches(wsEntry workspaceEntry) []branchEntry {
	stats := m.wsStats[wsEntry.Path]
	statsByPath := indexWorktreeStats(stats)

	active, sessionPaths := m.buildActiveBranches(wsEntry, stats, statsByPath)
	sortBranchesMainFirst(active, false)

	inactive := m.buildInactiveBranches(wsEntry, stats, statsByPath, sessionPaths)
	sortBranchesMainFirst(inactive, true)

	result := make([]branchEntry, 0, len(active)+len(inactive))
	result = append(result, active...)
	result = append(result, inactive...)
	return result
}

func indexWorktreeStats(stats WorkspaceStatsAlias) map[string]int {
	out := make(map[string]int, len(stats.Worktrees))
	for i, wt := range stats.Worktrees {
		out[wt.WorktreePath] = i
	}
	return out
}

func (m *Model) buildActiveBranches(
	wsEntry workspaceEntry,
	stats WorkspaceStatsAlias,
	statsByPath map[string]int,
) ([]branchEntry, map[string]bool) {
	var active []branchEntry
	sessionPaths := make(map[string]bool)
	for _, s := range m.sessions {
		if m.repoRoots[s.Name] != wsEntry.Path {
			continue
		}
		if m.isArchived(wsEntry.Path, s.Path) {
			continue
		}
		entry := m.makeActiveBranchEntry(s, wsEntry, stats, statsByPath)
		active = append(active, entry)
		sessionPaths[s.Path] = true
	}
	return active, sessionPaths
}

func (m *Model) makeActiveBranchEntry(
	s tmux.Session,
	wsEntry workspaceEntry,
	stats WorkspaceStatsAlias,
	statsByPath map[string]int,
) branchEntry {
	childName := trimPrefix(s.Name, wsEntry.Name)
	if s.Path == wsEntry.Path {
		if branch := resolveGitBranch(s.Path); branch != "" {
			childName = branch
		}
	}
	entry := branchEntry{
		Name:        childName,
		SessionName: s.Name,
		Path:        s.Path,
		Windows:     s.Windows,
		Attached:    s.Attached,
		IsSession:   true,
	}
	if idx, ok := statsByPath[s.Path]; ok {
		applyWorktreeStat(&entry, stats.Worktrees[idx])
	}
	if st, ok := m.stats[s.Name]; ok {
		if entry.DiffAdded == 0 {
			entry.DiffAdded = st.Added
		}
		if entry.DiffDel == 0 {
			entry.DiffDel = st.Deleted
		}
	}
	return entry
}

func (m *Model) buildInactiveBranches(
	wsEntry workspaceEntry,
	stats WorkspaceStatsAlias,
	statsByPath map[string]int,
	sessionPaths map[string]bool,
) []branchEntry {
	var inactive []branchEntry
	for _, wt := range m.wtByPath[wsEntry.Path] {
		if sessionPaths[wt.Path] {
			continue
		}
		if m.isArchived(wsEntry.Path, wt.Path) {
			continue
		}
		entry := branchEntry{Name: wt.Branch, Path: wt.Path}
		if idx, ok := statsByPath[wt.Path]; ok {
			applyWorktreeStat(&entry, stats.Worktrees[idx])
		} else {
			entry.IsMain = wt.IsMain
		}
		inactive = append(inactive, entry)
	}
	return inactive
}

func (m *Model) isArchived(workspacePath, targetPath string) bool {
	if m.archived == nil {
		return false
	}
	byWs := m.archived[workspacePath]
	if byWs == nil {
		return false
	}
	return byWs[targetPath]
}

func applyWorktreeStat(entry *branchEntry, wt WorktreeStatAlias) {
	entry.DiffAdded = wt.Added
	entry.DiffDel = wt.Deleted
	entry.IsMain = wt.IsMain
	entry.Staged = wt.Staged
	entry.Modified = wt.Modified
	entry.Untracked = wt.Untracked
	entry.Ahead = wt.Ahead
	entry.Behind = wt.Behind
}

// sortBranchesMainFirst sorts in-place: main branches first. When tieBreakByName
// is true, remaining entries are sorted lexicographically; otherwise stable order wins.
func sortBranchesMainFirst(entries []branchEntry, tieBreakByName bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		mi := isMainBranch(entries[i].Name) || entries[i].IsMain
		mj := isMainBranch(entries[j].Name) || entries[j].IsMain
		if mi != mj {
			return mi
		}
		if tieBreakByName {
			return entries[i].Name < entries[j].Name
		}
		return false
	})
}

// detectAgents finds running agent panes scoped to a workspace path.
func detectAgents(panes []tmux.Pane, workspacePath string, repoRoots map[string]string, sessions []tmux.Session) []agentEntry {
	agentCommands := make(map[string]agents.Agent)
	for _, a := range agents.DefaultAgents() {
		agentCommands[a.Command] = a
	}

	sessPathMap := make(map[string]string, len(sessions))
	for _, s := range sessions {
		if root, ok := repoRoots[s.Name]; ok {
			sessPathMap[s.Name] = root
		}
	}

	var detected []agentEntry
	for _, p := range panes {
		root := sessPathMap[p.SessionName]
		if root != workspacePath {
			continue
		}
		a, ok := agentCommands[p.Command]
		if !ok {
			continue
		}
		detected = append(detected, agentEntry{
			Name:        a.Name,
			AgentID:     a.ID,
			SessionName: p.SessionName,
			WindowIndex: p.WindowIndex,
			PaneIndex:   p.PaneIndex,
		})
	}

	detected = append(detected, agentEntry{
		Name:       "+ launch agent...",
		IsLauncher: true,
	})
	return detected
}
