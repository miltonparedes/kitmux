package workspaces

import (
	"sort"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

// rebuildDetail rebuilds the right column (branches + agents) for the
// currently selected project.
func (m *Model) rebuildDetail() {
	if len(m.projects) == 0 {
		m.branches = nil
		m.agentEntries = nil
		m.detailItems = 0
		return
	}

	proj := m.projects[m.projCursor]
	m.branches = m.buildBranches(proj)
	m.agentEntries = detectAgents(m.panes, proj.Path, m.repoRoots, m.sessions)
	m.detailItems = len(m.branches) + len(m.agentEntries)
	m.detCursor = 0
	m.detScroll = 0
}

// buildBranches merges live sessions and inactive worktrees into the
// ordered list shown in the detail column.
func (m *Model) buildBranches(proj projectEntry) []branchEntry {
	// Pull stats once for this workspace.
	ws := m.wsStats[proj.Path]
	statsByPath := make(map[string]int, len(ws.Worktrees))
	for i, wt := range ws.Worktrees {
		statsByPath[wt.WorktreePath] = i
	}

	var active []branchEntry
	sessionPaths := make(map[string]bool)

	for _, s := range m.sessions {
		root := m.repoRoots[s.Name]
		if root != proj.Path {
			continue
		}
		childName := trimPrefix(s.Name, proj.Name)
		if s.Path == proj.Path {
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
			wt := ws.Worktrees[idx]
			entry.DiffAdded = wt.Added
			entry.DiffDel = wt.Deleted
			entry.IsMain = wt.IsMain
			entry.Staged = wt.Staged
			entry.Modified = wt.Modified
			entry.Untracked = wt.Untracked
			entry.Ahead = wt.Ahead
			entry.Behind = wt.Behind
		}
		if st, ok := m.stats[s.Name]; ok {
			if entry.DiffAdded == 0 {
				entry.DiffAdded = st.Added
			}
			if entry.DiffDel == 0 {
				entry.DiffDel = st.Deleted
			}
		}
		active = append(active, entry)
		sessionPaths[s.Path] = true
	}

	sort.SliceStable(active, func(i, j int) bool {
		mi := isMainBranch(active[i].Name) || active[i].IsMain
		mj := isMainBranch(active[j].Name) || active[j].IsMain
		if mi != mj {
			return mi
		}
		return false
	})

	var inactive []branchEntry
	for _, wt := range m.wtByPath[proj.Path] {
		if sessionPaths[wt.Path] {
			continue
		}
		entry := branchEntry{
			Name: wt.Branch,
			Path: wt.Path,
		}
		if idx, ok := statsByPath[wt.Path]; ok {
			ws := ws.Worktrees[idx]
			entry.DiffAdded = ws.Added
			entry.DiffDel = ws.Deleted
			entry.IsMain = ws.IsMain
			entry.Staged = ws.Staged
			entry.Modified = ws.Modified
			entry.Untracked = ws.Untracked
			entry.Ahead = ws.Ahead
			entry.Behind = ws.Behind
		} else {
			entry.IsMain = wt.IsMain
		}
		inactive = append(inactive, entry)
	}

	sort.SliceStable(inactive, func(i, j int) bool {
		mi := isMainBranch(inactive[i].Name) || inactive[i].IsMain
		mj := isMainBranch(inactive[j].Name) || inactive[j].IsMain
		if mi != mj {
			return mi
		}
		return inactive[i].Name < inactive[j].Name
	})

	result := make([]branchEntry, 0, len(active)+len(inactive))
	result = append(result, active...)
	result = append(result, inactive...)
	return result
}

// detectAgents finds running agent panes scoped to a workspace path.
func detectAgents(panes []tmux.Pane, projectPath string, repoRoots map[string]string, sessions []tmux.Session) []agentEntry {
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
		if root != projectPath {
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
