package workspaces

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/tmux"
	wsreg "github.com/miltonparedes/kitmux/internal/workspaces"
	wsdata "github.com/miltonparedes/kitmux/internal/workspaces/data"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case dataLoadedMsg:
		m.projects = msg.projects
		m.sessions = msg.sessions
		m.repoRoots = msg.repoRoots
		m.wtByPath = msg.wtByPath
		m.panes = msg.panes
		m.clampProjCursor()

		// Seed from the persistent cache so the UI renders stats instantly.
		if m.stats_svc != nil && len(m.wsStats) == 0 {
			if cached, err := m.stats_svc.LoadAllCached(); err == nil {
				m.wsStats = cached
			}
		}
		m.applyWorkspaceSummary()
		m.rebuildDetail()

		// Kick off background refresh for every visible workspace.
		return m, refreshAllStatsCmd(m.stats_svc, m.projects)

	case statsLoadedMsg:
		if m.wsStats == nil {
			m.wsStats = make(map[string]wsdata.WorkspaceStats)
		}
		if m.wtByPath == nil {
			m.wtByPath = make(map[string][]worktree.Worktree, len(msg.wsStats))
		}
		for path, ws := range msg.wsStats {
			m.wsStats[path] = ws
			m.wtByPath[path] = worktreesFromStats(ws)
		}
		if len(msg.stats) > 0 {
			if m.stats == nil {
				m.stats = make(map[string]sessionStats)
			}
			for k, v := range msg.stats {
				m.stats[k] = v
			}
		}
		m.applyWorkspaceSummary()
		m.rebuildDetail()
		return m, nil

	case switchDoneMsg:
		return m, tea.Quit

	case actionDoneMsg:
		return m, loadDataCmd(m.stats_svc)

	case zoxideLoadedMsg:
		m.zoxide.all = msg.entries
		m.zoxide.filtered = msg.entries
		m.zoxide.cursor = 0
		m.zoxide.scroll = 0
		return m, nil

	case toastMsg:
		return m, m.pushToast(msg.text, msg.level)

	case toastClearMsg:
		if msg.seq == m.toastSeq {
			m.toast = ""
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		switch m.mode {
		case modeConfirm:
			return m.handleConfirm(msg)
		case modeFiltering:
			return m.handleFilter(msg)
		case modeProjectSearch:
			return m.handleProjectSearch(msg)
		case modeNewBranch:
			return m.handleNewBranch(msg)
		case modeAgentPicker:
			return m.handleAgentPicker(msg)
		default:
			return m.handleKey(msg)
		}
	}
	return m, nil
}

// applyWorkspaceSummary populates the aggregate fields on each projectEntry
// (diff summary, dirty worktree count, worktree count) from m.wsStats.
func (m *Model) applyWorkspaceSummary() {
	for i := range m.projects {
		ws, ok := m.wsStats[m.projects[i].Path]
		if !ok {
			m.projects[i].Added = 0
			m.projects[i].Deleted = 0
			m.projects[i].Worktrees = 0
			m.projects[i].DirtyCount = 0
			continue
		}
		added, deleted := ws.TotalDiff()
		dirty := 0
		for _, wt := range ws.Worktrees {
			if wt.Dirty() {
				dirty++
			}
		}
		m.projects[i].Added = added
		m.projects[i].Deleted = deleted
		m.projects[i].Worktrees = len(ws.Worktrees)
		m.projects[i].DirtyCount = dirty
	}
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.focus == colProjects {
			m.projCursor--
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		} else {
			m.detCursor--
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if m.focus == colProjects {
			m.projCursor++
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		} else {
			m.detCursor++
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionRelease {
			return m, nil
		}
		return m.handleLeftClick(msg.X, msg.Y)
	}
	return m, nil
}

// handleLeftClick maps the (x, y) of a release event to either a project on
// the left column or a branch/agent on the right column. Clicking on the
// already-selected row "activates" it (moves focus to detail or opens it),
// matching the Enter-key semantics. Clicking elsewhere just moves the cursor.
func (m Model) handleLeftClick(x, y int) (tea.Model, tea.Cmd) {
	left := m.leftWidth()
	rightStart := left + 3 // gutter " │ "

	// Headers + separator take 2 lines at the top of each column.
	itemY := y - 2
	if itemY < 0 {
		return m, nil
	}
	// Each row takes 2 visual lines (item + hairline separator).
	row := itemY / 2

	switch {
	case x < left:
		return m.clickProject(row)
	case x >= rightStart:
		return m.clickDetail(row)
	}
	return m, nil
}

func (m Model) clickProject(row int) (tea.Model, tea.Cmd) {
	if len(m.projects) == 0 {
		return m, nil
	}
	idx := m.projScroll + row
	if idx < 0 || idx >= len(m.projects) {
		return m, nil
	}
	if idx == m.projCursor && m.focus == colProjects {
		// Second click on the already-selected project: dive into detail.
		if m.detailItems > 0 {
			m.focus = colDetail
		}
		return m, nil
	}
	m.focus = colProjects
	m.projCursor = idx
	m.ensureProjVisible()
	m.rebuildDetail()
	if m.stats_svc != nil {
		path := m.projects[m.projCursor].Path
		return m, refreshStatsCmd(m.stats_svc, path)
	}
	return m, nil
}

// clickDetail resolves a (visual row index) within the right column to an
// entry in m.branches or m.agentEntries, accounting for the section headers
// between them. The right column lays rows out as:
//
//	row 0       : "Branches" header
//	row 1..B    : branch rows
//	(if agents) blank
//	            : "Agents" header
//	            : agent rows
func (m Model) clickDetail(visualRow int) (tea.Model, tea.Cmd) {
	if m.detailItems == 0 {
		return m, nil
	}
	// First visible row of the right column is the "Branches" header.
	row := visualRow - 1
	if row < 0 {
		// Click on the "Branches" header — treat as focus shift only.
		m.focus = colDetail
		return m, nil
	}

	branchVisible := len(m.branches) - m.detScroll
	if branchVisible < 0 {
		branchVisible = 0
	}

	var idx int
	switch {
	case row < branchVisible:
		idx = m.detScroll + row
	default:
		// Past the branches list: account for the blank line + "Agents" header.
		offset := row - branchVisible
		// Need at least 2 lines of header padding before agents start.
		if offset < 2 {
			m.focus = colDetail
			return m, nil
		}
		agentRow := offset - 2
		if agentRow < 0 || agentRow >= len(m.agentEntries) {
			return m, nil
		}
		idx = len(m.branches) + agentRow
	}

	if idx < 0 || idx >= m.detailItems {
		return m, nil
	}
	if idx == m.detCursor && m.focus == colDetail {
		// Second click on the same detail row: activate it.
		return m.activateDetailItem()
	}
	m.focus = colDetail
	m.detCursor = idx
	m.ensureDetVisible()
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		return m.moveCursor(+1)

	case "k", "up":
		return m.moveCursor(-1)

	case "g", "home":
		if m.focus == colProjects {
			m.projCursor = 0
			m.projScroll = 0
			m.rebuildDetail()
		} else {
			m.detCursor = 0
			m.detScroll = 0
		}
		return m, nil

	case "G", "end":
		if m.focus == colProjects {
			m.projCursor = len(m.projects) - 1
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		} else {
			m.detCursor = m.detailItems - 1
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil

	case "l", "right":
		if m.focus == colProjects && m.detailItems > 0 {
			m.focus = colDetail
		}
		return m, nil

	case "h", "left":
		if m.focus == colDetail {
			m.focus = colProjects
		}
		return m, nil

	case keyEnter:
		if m.focus == colProjects {
			if m.detailItems > 0 {
				m.focus = colDetail
			}
			return m, nil
		}
		return m.activateDetailItem()

	case "/":
		m.mode = modeFiltering
		m.filter.SetValue("")
		m.filter.Focus()
		return m, textinput.Blink

	case "n":
		if m.focus == colProjects {
			m.mode = modeProjectSearch
			m.zoxide.input.SetValue("")
			m.zoxide.input.Focus()
			m.zoxide.all = nil
			m.zoxide.filtered = nil
			m.zoxide.cursor = 0
			m.zoxide.scroll = 0
			return m, tea.Batch(textinput.Blink, loadZoxide())
		}
		return m, nil

	case "f":
		// Global zoxide search (any dir, not just workspaces).
		m.mode = modeProjectSearch
		m.zoxide.input.SetValue("")
		m.zoxide.input.Focus()
		m.zoxide.all = nil
		m.zoxide.filtered = nil
		m.zoxide.cursor = 0
		m.zoxide.scroll = 0
		return m, tea.Batch(textinput.Blink, loadZoxide())

	case "a":
		if m.focus == colDetail || (m.focus == colProjects && len(m.projects) > 0) {
			m.mode = modeAgentPicker
			m.agentPicker.cursor = 0
		}
		return m, nil

	case "d":
		if m.focus == colProjects && len(m.projects) > 0 {
			p := m.projects[m.projCursor]
			m.confirmName = p.Name
			m.confirmPath = p.Path
			m.mode = modeConfirm
		}
		return m, nil

	case "c":
		if m.focus == colDetail && len(m.projects) > 0 {
			m.newBranchProj = m.projects[m.projCursor]
			m.mode = modeNewBranch
			m.newBranch.SetValue("")
			m.newBranch.Focus()
			return m, textinput.Blink
		}
		return m, nil

	case "r":
		// Force refresh: purge cache for visible workspaces and reload.
		if m.stats_svc != nil {
			for _, p := range m.projects {
				_ = m.stats_svc.Invalidate(p.Path)
			}
		}
		return m, loadDataCmd(m.stats_svc)

	case "q", "esc":
		if m.focus == colDetail {
			m.focus = colProjects
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) moveCursor(delta int) (tea.Model, tea.Cmd) {
	if m.focus == colProjects {
		m.projCursor += delta
		m.clampProjCursor()
		m.ensureProjVisible()
		m.rebuildDetail()
		// Debounce: refresh stats for the new project in background.
		if m.stats_svc != nil && len(m.projects) > 0 {
			path := m.projects[m.projCursor].Path
			return m, refreshStatsCmd(m.stats_svc, path)
		}
		return m, nil
	}
	m.detCursor += delta
	m.clampDetCursor()
	m.ensureDetVisible()
	return m, nil
}

func (m Model) activateDetailItem() (tea.Model, tea.Cmd) {
	if m.detCursor < len(m.branches) {
		b := m.branches[m.detCursor]
		if b.IsSession && b.SessionName != "" {
			return m, m.switchTo(b.SessionName)
		}
		if len(m.projects) > 0 {
			proj := m.projects[m.projCursor]
			return m, m.openWorktreeSession(proj.Name, b)
		}
		return m, nil
	}

	agentIdx := m.detCursor - len(m.branches)
	if agentIdx >= 0 && agentIdx < len(m.agentEntries) {
		ae := m.agentEntries[agentIdx]
		if ae.IsLauncher {
			m.mode = modeAgentPicker
			m.agentPicker.cursor = 0
			return m, nil
		}
		target := fmt.Sprintf("%s:%d.%d", ae.SessionName, ae.WindowIndex, ae.PaneIndex)
		return m, m.switchTo(target)
	}
	return m, nil
}

// -------- Modal handlers --------

func (m Model) handleFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.filter.SetValue("")
		return m, nil
	case keyEnter:
		m.mode = modeNormal
		m.filter.Blur()
		// Accept: if a single filtered row matches, snap the cursor to it.
		matches := filteredProjectIndices(m.projects, m.filter.Value())
		if len(matches) > 0 {
			m.projCursor = matches[0]
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		}
		m.filter.SetValue("")
		return m, nil
	case "up", "ctrl+k":
		m.moveFilteredCursor(-1)
		return m, nil
	case "down", "ctrl+j":
		m.moveFilteredCursor(+1)
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.clampFilteredCursor()
	return m, cmd
}

func filteredProjectIndices(projects []projectEntry, query string) []int {
	if query == "" {
		out := make([]int, len(projects))
		for i := range projects {
			out[i] = i
		}
		return out
	}
	names := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.Name
	}
	matches := fuzzy.Find(query, names)
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Index)
	}
	return out
}

func (m *Model) moveFilteredCursor(delta int) {
	idxs := filteredProjectIndices(m.projects, m.filter.Value())
	if len(idxs) == 0 {
		return
	}
	pos := 0
	for i, idx := range idxs {
		if idx == m.projCursor {
			pos = i
			break
		}
	}
	pos += delta
	if pos < 0 {
		pos = 0
	}
	if pos >= len(idxs) {
		pos = len(idxs) - 1
	}
	m.projCursor = idxs[pos]
	m.clampProjCursor()
	m.ensureProjVisible()
	m.rebuildDetail()
}

func (m *Model) clampFilteredCursor() {
	idxs := filteredProjectIndices(m.projects, m.filter.Value())
	if len(idxs) == 0 {
		return
	}
	for _, idx := range idxs {
		if idx == m.projCursor {
			return
		}
	}
	m.projCursor = idxs[0]
	m.clampProjCursor()
	m.ensureProjVisible()
	m.rebuildDetail()
}

// -------- Project search (zoxide) --------

func (m Model) handleProjectSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case keyEnter:
		if sel := m.zoxide.selected(); sel != nil {
			path := sel.Path
			name := filepath.Base(path)
			added := wsreg.AddWorkspace(name, path)
			m.mode = modeNormal
			if !added {
				// Path already tracked; just focus it.
				return m, tea.Batch(loadDataCmd(m.stats_svc), func() tea.Msg {
					return toastMsg{text: "already registered: " + name, level: toastInfo}
				})
			}
			return m, loadDataCmd(m.stats_svc)
		}
		return m, nil
	case "up", "ctrl+k":
		m.zoxide.cursor--
		m.zoxide.clampCursor()
		m.zoxide.ensureVisible(m.height - 4)
		return m, nil
	case "down", "ctrl+j":
		m.zoxide.cursor++
		m.zoxide.clampCursor()
		m.zoxide.ensureVisible(m.height - 4)
		return m, nil
	}
	var cmd tea.Cmd
	m.zoxide.input, cmd = m.zoxide.input.Update(msg)
	m.zoxide.filter()
	return m, cmd
}

// -------- New branch --------

func (m Model) handleNewBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case keyEnter:
		branch := m.newBranch.Value()
		if branch == "" {
			return m, nil
		}
		return m, m.createWorktreeAndOpen(m.newBranchProj.Name, m.newBranchProj.Path, branch)
	}
	var cmd tea.Cmd
	m.newBranch, cmd = m.newBranch.Update(msg)
	return m, cmd
}

// -------- Confirm delete workspace --------

func (m Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.mode = modeNormal
		wsreg.RemoveWorkspace(m.confirmPath)
		if m.stats_svc != nil {
			_ = m.stats_svc.Invalidate(m.confirmPath)
		}
		return m, loadDataCmd(m.stats_svc)
	default:
		m.mode = modeNormal
		return m, nil
	}
}

// -------- Agent picker --------

func (m Model) handleAgentPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case "j", "down":
		if m.agentPicker.cursor < len(m.agentPicker.agents)-1 {
			m.agentPicker.cursor++
		}
		return m, nil
	case "k", "up":
		if m.agentPicker.cursor > 0 {
			m.agentPicker.cursor--
		}
		return m, nil
	case "tab":
		if len(m.agentPicker.agents) > 0 {
			c := m.agentPicker.cursor
			a := m.agentPicker.agents[c]
			m.agentPicker.modeIndex[c] = (m.agentPicker.modeIndex[c] + 1) % len(a.Modes)
		}
		return m, nil
	case keyEnter:
		return m, m.launchSelectedAgent()
	}
	return m, nil
}

func (m Model) launchSelectedAgent() tea.Cmd {
	if len(m.agentPicker.agents) == 0 || len(m.projects) == 0 {
		return nil
	}
	a := m.agentPicker.agents[m.agentPicker.cursor]
	mode := a.Modes[m.agentPicker.modeIndex[m.agentPicker.cursor]]
	proj := m.projects[m.projCursor]
	command := a.FullCommand(mode)

	return func() tea.Msg {
		sessName := proj.Name + "-" + a.ID
		sessName = uniqueSessName(sessName)
		if err := tmux.NewSessionInDir(sessName, proj.Path); err != nil {
			return toastMsg{text: "tmux new-session failed: " + err.Error(), level: toastError}
		}
		_ = tmux.SendKeys(sessName, command)
		_ = tmux.SwitchClient(sessName)
		return switchDoneMsg{}
	}
}

// -------- Actions --------

func (m Model) switchTo(name string) tea.Cmd {
	return func() tea.Msg {
		if err := tmux.SwitchClient(name); err != nil {
			return toastMsg{text: "switch-client failed: " + err.Error(), level: toastError}
		}
		return switchDoneMsg{}
	}
}

func (m Model) openWorktreeSession(project string, b branchEntry) tea.Cmd {
	return func() tea.Msg {
		sessName := project + "-" + b.Name
		sessName = uniqueSessName(sessName)
		path := b.Path
		if path == "" {
			return toastMsg{text: "worktree missing path", level: toastWarn}
		}
		if err := tmux.NewSessionInDir(sessName, path); err != nil {
			return toastMsg{text: "tmux new-session failed: " + err.Error(), level: toastError}
		}
		_ = tmux.SwitchClient(sessName)
		return switchDoneMsg{}
	}
}

func (m Model) createWorktreeAndOpen(project, projPath, branch string) tea.Cmd {
	svc := m.stats_svc
	return func() tea.Msg {
		cmd := exec.Command("wt", "switch", "--create", branch)
		cmd.Dir = projPath
		if err := cmd.Run(); err != nil {
			return toastMsg{text: "wt create failed: " + err.Error(), level: toastError}
		}
		wts, err := worktree.ListInDir(projPath)
		if err != nil {
			return toastMsg{text: "wt list failed: " + err.Error(), level: toastError}
		}
		if svc != nil {
			_ = svc.Invalidate(projPath)
		}
		for _, wt := range wts {
			if wt.Branch == branch {
				sessName := uniqueSessName(project + "-" + branch)
				if err := tmux.NewSessionInDir(sessName, wt.Path); err != nil {
					return toastMsg{text: "tmux new-session failed: " + err.Error(), level: toastError}
				}
				_ = tmux.SwitchClient(sessName)
				return switchDoneMsg{}
			}
		}
		return actionDoneMsg{}
	}
}
