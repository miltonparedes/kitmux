package workspaces

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/agents"
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
		return m.handleDataLoaded(msg)
	case statsLoadedMsg:
		return m.handleStatsLoaded(msg)
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
		return m.routeKey(msg)
	}
	return m, nil
}

func (m Model) handleDataLoaded(msg dataLoadedMsg) (tea.Model, tea.Cmd) {
	m.workspaces = msg.workspaces
	m.sessions = msg.sessions
	m.repoRoots = msg.repoRoots
	m.wtByPath = msg.wtByPath
	m.panes = msg.panes
	m.archived = msg.archived
	m.clampWorkspaceCursor()
	if m.stats_svc != nil && len(m.wsStats) == 0 {
		if cached, err := m.stats_svc.LoadAllCached(); err == nil {
			m.wsStats = cached
		}
	}
	m.applyWorkspaceSummary()
	m.rebuildDetail()
	return m, refreshAllStatsCmd(m.stats_svc, m.workspaces)
}

func (m Model) handleStatsLoaded(msg statsLoadedMsg) (tea.Model, tea.Cmd) {
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
}

func (m Model) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeConfirm:
		return m.handleConfirm(msg)
	case modeActionPicker:
		return m.handleActionPicker(msg)
	case modeHelp:
		return m.handleHelp(msg)
	case modeFiltering:
		return m.handleFilter(msg)
	case modeWorkspaceSearch:
		return m.handleWorkspaceSearch(msg)
	case modeNewBranch:
		return m.handleNewBranch(msg)
	case modeNewBranchAgent:
		return m.handleNewBranchAgent(msg)
	case modeAgentAttachChoice:
		return m.handleAgentAttachChoice(msg)
	case modeAttachBranchPicker:
		return m.handleAttachBranchPicker(msg)
	case modeAgentPicker:
		return m.handleAgentPicker(msg)
	default:
		return m.handleKey(msg)
	}
}

// applyWorkspaceSummary populates the aggregate fields on each workspaceEntry
// (diff summary, dirty worktree count, worktree count) from m.wsStats.
func (m *Model) applyWorkspaceSummary() {
	for i := range m.workspaces {
		ws, ok := m.wsStats[m.workspaces[i].Path]
		if !ok {
			m.workspaces[i].Added = 0
			m.workspaces[i].Deleted = 0
			m.workspaces[i].Worktrees = 0
			m.workspaces[i].DirtyCount = 0
			continue
		}
		added, deleted := ws.TotalDiff()
		dirty := 0
		for _, wt := range ws.Worktrees {
			if wt.Dirty() {
				dirty++
			}
		}
		m.workspaces[i].Added = added
		m.workspaces[i].Deleted = deleted
		m.workspaces[i].Worktrees = len(ws.Worktrees)
		m.workspaces[i].DirtyCount = dirty
	}
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.focus == colWorkspaces {
			m.wsCursor--
			m.clampWorkspaceCursor()
			m.ensureWorkspaceVisible()
			m.rebuildDetail()
		} else {
			m.detCursor--
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if m.focus == colWorkspaces {
			m.wsCursor++
			m.clampWorkspaceCursor()
			m.ensureWorkspaceVisible()
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
		return m.clickWorkspace(row)
	case x >= rightStart:
		return m.clickDetail(row)
	}
	return m, nil
}

func (m Model) clickWorkspace(row int) (tea.Model, tea.Cmd) {
	if len(m.workspaces) == 0 {
		return m, nil
	}
	idx := m.wsScroll + row
	if idx < 0 || idx >= len(m.workspaces) {
		return m, nil
	}
	if idx == m.wsCursor && m.focus == colWorkspaces {
		// Second click on the already-selected project: dive into detail.
		if m.detailItems > 0 {
			m.focus = colDetail
		}
		return m, nil
	}
	m.focus = colWorkspaces
	m.wsCursor = idx
	m.ensureWorkspaceVisible()
	m.rebuildDetail()
	if m.stats_svc != nil {
		path := m.workspaces[m.wsCursor].Path
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
	if updated, cmd, handled := m.handleKeyNav(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.handleKeyAction(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.handleKeySystem(msg); handled {
		return updated, cmd
	}
	return m, nil
}

func (m Model) handleKeyNav(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down":
		model, cmd := m.moveCursor(+1)
		return model, cmd, true
	case "k", "up":
		model, cmd := m.moveCursor(-1)
		return model, cmd, true
	case "g", "home":
		return m.jumpHome(), nil, true
	case "G", "end":
		return m.jumpEnd(), nil, true
	case "l", "right":
		if m.focus == colWorkspaces && m.detailItems > 0 {
			m.focus = colDetail
		}
		return m, nil, true
	case "h", "left":
		if m.focus == colDetail {
			m.focus = colWorkspaces
		}
		return m, nil, true
	case keyEnter:
		if m.focus == colWorkspaces {
			if m.detailItems > 0 {
				m.focus = colDetail
			}
			return m, nil, true
		}
		model, cmd := m.activateDetailItem()
		return model, cmd, true
	}
	return m, nil, false
}

func (m Model) jumpHome() tea.Model {
	if m.focus == colWorkspaces {
		m.wsCursor = 0
		m.wsScroll = 0
		m.rebuildDetail()
	} else {
		m.detCursor = 0
		m.detScroll = 0
	}
	return m
}

func (m Model) jumpEnd() tea.Model {
	if m.focus == colWorkspaces {
		m.wsCursor = len(m.workspaces) - 1
		m.clampWorkspaceCursor()
		m.ensureWorkspaceVisible()
		m.rebuildDetail()
	} else {
		m.detCursor = m.detailItems - 1
		m.clampDetCursor()
		m.ensureDetVisible()
	}
	return m
}

func (m Model) handleKeyAction(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "/":
		m.mode = modeFiltering
		m.filter.SetValue("")
		m.filter.Focus()
		return m, textinput.Blink, true
	case "n":
		if m.focus == colWorkspaces {
			return m.startZoxideSearch(), tea.Batch(textinput.Blink, loadZoxide()), true
		}
		return m, nil, true
	case "f":
		return m.startZoxideSearch(), tea.Batch(textinput.Blink, loadZoxide()), true
	case "a":
		return m.startAgentAttach(agentTargetWindow), nil, true
	case "A":
		return m.startAgentAttach(agentTargetSplit), nil, true
	case "x", "d":
		model, cmd := m.openActionPicker()
		return model, cmd, true
	case "c":
		if len(m.workspaces) > 0 {
			m.newBranchWs = m.workspaces[m.wsCursor]
			m.mode = modeNewBranch
			m.newBranch.SetValue("")
			m.newBranch.Focus()
			return m, textinput.Blink, true
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) startZoxideSearch() Model {
	m.mode = modeWorkspaceSearch
	m.zoxide.input.SetValue("")
	m.zoxide.input.Focus()
	m.zoxide.all = nil
	m.zoxide.filtered = nil
	m.zoxide.cursor = 0
	m.zoxide.scroll = 0
	return m
}

func (m Model) handleKeySystem(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "r":
		if m.stats_svc != nil {
			for _, p := range m.workspaces {
				_ = m.stats_svc.Invalidate(p.Path)
			}
		}
		return m, loadDataCmd(m.stats_svc), true
	case "?":
		m.mode = modeHelp
		return m, nil, true
	case "q", "esc":
		if m.focus == colDetail {
			m.focus = colWorkspaces
			return m, nil, true
		}
		return m, tea.Quit, true
	}
	return m, nil, false
}

func (m Model) moveCursor(delta int) (tea.Model, tea.Cmd) {
	if m.focus == colWorkspaces {
		m.wsCursor += delta
		m.clampWorkspaceCursor()
		m.ensureWorkspaceVisible()
		m.rebuildDetail()
		// Debounce: refresh stats for the new project in background.
		if m.stats_svc != nil && len(m.workspaces) > 0 {
			path := m.workspaces[m.wsCursor].Path
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
		if len(m.workspaces) > 0 {
			proj := m.workspaces[m.wsCursor]
			return m, m.openWorktreeSession(proj.Name, b)
		}
		return m, nil
	}

	agentIdx := m.detCursor - len(m.branches)
	if agentIdx >= 0 && agentIdx < len(m.agentEntries) {
		ae := m.agentEntries[agentIdx]
		if ae.IsLauncher {
			// Launcher entry in the detail column: fall into the "where?"
			// chooser just like pressing `a` from the workspaces column.
			m.mode = modeAgentAttachChoice
			m.attachChoiceCursor = 0
			m.agentPickerTarget = agentTargetWindow
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
		matches := filteredWorkspaceIndices(m.workspaces, m.filter.Value())
		if len(matches) > 0 {
			m.wsCursor = matches[0]
			m.clampWorkspaceCursor()
			m.ensureWorkspaceVisible()
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

func filteredWorkspaceIndices(workspaces []workspaceEntry, query string) []int {
	if query == "" {
		out := make([]int, len(workspaces))
		for i := range workspaces {
			out[i] = i
		}
		return out
	}
	names := make([]string, len(workspaces))
	for i, w := range workspaces {
		names[i] = w.Name
	}
	matches := fuzzy.Find(query, names)
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Index)
	}
	return out
}

func (m *Model) moveFilteredCursor(delta int) {
	idxs := filteredWorkspaceIndices(m.workspaces, m.filter.Value())
	if len(idxs) == 0 {
		return
	}
	pos := 0
	for i, idx := range idxs {
		if idx == m.wsCursor {
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
	m.wsCursor = idxs[pos]
	m.clampWorkspaceCursor()
	m.ensureWorkspaceVisible()
	m.rebuildDetail()
}

func (m *Model) clampFilteredCursor() {
	idxs := filteredWorkspaceIndices(m.workspaces, m.filter.Value())
	if len(idxs) == 0 {
		return
	}
	for _, idx := range idxs {
		if idx == m.wsCursor {
			return
		}
	}
	m.wsCursor = idxs[0]
	m.clampWorkspaceCursor()
	m.ensureWorkspaceVisible()
	m.rebuildDetail()
}

// -------- Project search (zoxide) --------

func (m Model) handleWorkspaceSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

// -------- New worktree (+ optional agent) --------

// handleNewBranch handles the "new worktree" text input. Enter creates the
// worktree without launching an agent; Tab advances to the agent picker so
// the user can pre-select which agent to launch in the new session.
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
		return m, m.createWorktreeAndOpen(m.newBranchWs.Name, m.newBranchWs.Path, branch, nil, agents.AgentMode{})
	case "tab":
		branch := m.newBranch.Value()
		if branch == "" {
			return m, nil
		}
		m.mode = modeNewBranchAgent
		m.agentPickerIntent = agentIntentNewWorktreeAgent
		m.agentPickerTarget = agentTargetWindow
		m.agentPicker.cursor = 0
		return m, nil
	}
	var cmd tea.Cmd
	m.newBranch, cmd = m.newBranch.Update(msg)
	return m, cmd
}

// handleNewBranchAgent picks an agent to launch alongside the new worktree.
// It shares keybindings with handleAgentPicker, but confirmation here creates
// the worktree (and then the session + agent window) instead of attaching to
// an existing branch.
func (m Model) handleNewBranchAgent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Go back to the branch input so the user can still confirm without
		// an agent.
		m.mode = modeNewBranch
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
		branch := m.newBranch.Value()
		if branch == "" {
			m.mode = modeNewBranch
			return m, nil
		}
		if len(m.agentPicker.agents) == 0 {
			return m, m.createWorktreeAndOpen(m.newBranchWs.Name, m.newBranchWs.Path, branch, nil, agents.AgentMode{})
		}
		a := m.agentPicker.agents[m.agentPicker.cursor]
		mode := a.Modes[m.agentPicker.modeIndex[m.agentPicker.cursor]]
		return m, m.createWorktreeAndOpen(m.newBranchWs.Name, m.newBranchWs.Path, branch, &a, mode)
	}
	return m, nil
}

// -------- Confirm delete workspace --------

func (m Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return m.confirmYes()
	default:
		// "n", "N", "esc" and any other key cancel.
		m.resetConfirm()
		return m, nil
	}
}

func (m Model) confirmYes() (tea.Model, tea.Cmd) {
	action := m.confirmAction
	path := m.confirmPath
	branch := m.confirmBranch
	wpath := m.confirmWPath
	m.resetConfirm()

	switch action {
	case confirmActionRemoveWorkspace:
		wsreg.RemoveWorkspace(path)
		if m.stats_svc != nil {
			_ = m.stats_svc.Invalidate(path)
		}
		_ = wsreg.PurgeArchivedWorktreesForWorkspace(path)
		return m, loadDataCmd(m.stats_svc)
	case confirmActionRemoveWorktree:
		return m, m.deleteWorktree(path, wpath, branch)
	}
	return m, nil
}

func (m *Model) resetConfirm() {
	m.mode = modeNormal
	m.confirmAction = confirmActionNone
	m.confirmName = ""
	m.confirmPath = ""
	m.confirmBranch = ""
	m.confirmWPath = ""
}

func (m Model) deleteWorktree(workspacePath, worktreePath, branch string) tea.Cmd {
	svc := m.stats_svc
	return func() tea.Msg {
		if branch == "" {
			return toastMsg{text: "missing worktree branch", level: toastWarn}
		}
		if worktreePath == "" {
			return toastMsg{text: "missing worktree path", level: toastWarn}
		}
		sessions, _ := tmux.ListSessions()
		for _, s := range sessions {
			if s.Path == worktreePath {
				_ = tmux.KillSession(s.Name)
			}
		}
		if err := worktree.RemoveInDir(workspacePath, branch); err != nil {
			return toastMsg{text: "wt remove failed: " + err.Error(), level: toastError}
		}
		_ = wsreg.RemoveArchivedWorktree(workspacePath, worktreePath)
		if svc != nil {
			_ = svc.Invalidate(workspacePath)
		}
		return actionDoneMsg{}
	}
}

func (m Model) archiveWorktree(workspacePath, worktreePath string) tea.Cmd {
	svc := m.stats_svc
	return func() tea.Msg {
		if workspacePath == "" || worktreePath == "" {
			return toastMsg{text: "missing worktree path", level: toastWarn}
		}
		if !wsreg.AddArchivedWorktree(workspacePath, worktreePath) {
			return toastMsg{text: "archive failed", level: toastError}
		}
		if svc != nil {
			_ = svc.Invalidate(workspacePath)
		}
		return actionDoneMsg{}
	}
}

func (m Model) openActionPicker() (tea.Model, tea.Cmd) {
	if len(m.workspaces) == 0 {
		return m, nil
	}
	items := []actionMenuItem{}
	if m.focus == colWorkspaces {
		items = append(items, actionMenuItem{Label: "Remove workspace from list", Kind: actionKindRemoveWorkspace})
	} else if m.detCursor >= 0 && m.detCursor < len(m.branches) {
		br := m.branches[m.detCursor]
		if !br.IsMain && !isMainBranch(br.Name) {
			items = append(items,
				actionMenuItem{Label: "Archive worktree (hide from view)", Kind: actionKindArchiveWorktree},
				actionMenuItem{Label: "Delete worktree permanently", Kind: actionKindDeleteWorktree},
			)
		}
	}
	if len(items) == 0 {
		return m, m.pushToast("no actions available for this selection", toastInfo)
	}
	m.actionItems = items
	m.actionCursor = 0
	m.mode = modeActionPicker
	return m, nil
}

func (m Model) handleActionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.closeActionPicker()
		return m, nil
	case "j", "down":
		if m.actionCursor < len(m.actionItems)-1 {
			m.actionCursor++
		}
		return m, nil
	case "k", "up":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
		return m, nil
	case keyEnter:
		return m.confirmActionPickerSelection()
	}
	return m, nil
}

func (m *Model) closeActionPicker() {
	m.mode = modeNormal
	m.actionItems = nil
	m.actionCursor = 0
}

func (m Model) confirmActionPickerSelection() (tea.Model, tea.Cmd) {
	if m.actionCursor < 0 || m.actionCursor >= len(m.actionItems) || len(m.workspaces) == 0 {
		return m, nil
	}
	selected := m.actionItems[m.actionCursor]
	m.closeActionPicker()
	switch selected.Kind {
	case actionKindRemoveWorkspace:
		return m.dispatchRemoveWorkspaceAction()
	case actionKindArchiveWorktree:
		return m.dispatchArchiveWorktreeAction()
	case actionKindDeleteWorktree:
		return m.dispatchDeleteWorktreeAction()
	}
	return m, nil
}

func (m Model) dispatchRemoveWorkspaceAction() (tea.Model, tea.Cmd) {
	p := m.workspaces[m.wsCursor]
	m.confirmAction = confirmActionRemoveWorkspace
	m.confirmName = p.Name
	m.confirmPath = p.Path
	m.confirmBranch = ""
	m.confirmWPath = ""
	m.mode = modeConfirm
	return m, nil
}

func (m Model) dispatchArchiveWorktreeAction() (tea.Model, tea.Cmd) {
	br, ok := m.selectedBranch()
	if !ok {
		return m, nil
	}
	if br.Path == "" {
		return m, m.pushToast("missing worktree path", toastWarn)
	}
	p := m.workspaces[m.wsCursor]
	return m, m.archiveWorktree(p.Path, br.Path)
}

func (m Model) dispatchDeleteWorktreeAction() (tea.Model, tea.Cmd) {
	br, ok := m.selectedBranch()
	if !ok {
		return m, nil
	}
	if br.Path == "" {
		return m, m.pushToast("missing worktree path", toastWarn)
	}
	p := m.workspaces[m.wsCursor]
	m.confirmAction = confirmActionRemoveWorktree
	m.confirmName = p.Name
	m.confirmPath = p.Path
	m.confirmBranch = br.Name
	m.confirmWPath = br.Path
	m.mode = modeConfirm
	return m, nil
}

func (m Model) selectedBranch() (branchEntry, bool) {
	if m.detCursor < 0 || m.detCursor >= len(m.branches) {
		return branchEntry{}, false
	}
	return m.branches[m.detCursor], true
}

func (m Model) handleHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?":
		m.mode = modeNormal
		return m, nil
	default:
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
		if len(m.agentPicker.agents) == 0 || len(m.workspaces) == 0 {
			return m, nil
		}
		a := m.agentPicker.agents[m.agentPicker.cursor]
		mode := a.Modes[m.agentPicker.modeIndex[m.agentPicker.cursor]]
		m.mode = modeNormal
		return m, m.attachAgentToBranch(m.attachBranch, a, mode, m.agentPickerTarget)
	}
	return m, nil
}

// startAgentAttach routes the `a`/`A` key press to the appropriate flow.
// When focus is on the detail column with a branch under the cursor, the
// agent picker opens pre-targeting that branch. When focus is on the
// workspaces column (no branch context), it opens a small chooser instead.
func (m Model) startAgentAttach(target agentTarget) tea.Model {
	if len(m.workspaces) == 0 {
		return m
	}
	if m.focus == colDetail && m.detCursor >= 0 && m.detCursor < len(m.branches) {
		br := m.branches[m.detCursor]
		return m.openAgentPickerFor(br, target)
	}
	// Workspaces focus or agents row selected: open the "where do you want
	// the agent?" modal.
	m.mode = modeAgentAttachChoice
	m.attachChoiceCursor = 0
	m.agentPickerTarget = target
	return m
}

// openAgentPickerFor prepares the picker to attach an agent to `br`.
func (m Model) openAgentPickerFor(br branchEntry, target agentTarget) Model {
	m.mode = modeAgentPicker
	m.agentPicker.cursor = 0
	m.agentPickerIntent = agentIntentAttachBranch
	m.agentPickerTarget = target
	m.attachBranch = br
	return m
}

// handleAgentAttachChoice lets the user pick between attaching the agent to
// an existing branch or creating a new worktree (which then runs the agent).
func (m Model) handleAgentAttachChoice(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const choiceCount = 2
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case "j", "down":
		if m.attachChoiceCursor < choiceCount-1 {
			m.attachChoiceCursor++
		}
		return m, nil
	case "k", "up":
		if m.attachChoiceCursor > 0 {
			m.attachChoiceCursor--
		}
		return m, nil
	case keyEnter:
		if m.attachChoiceCursor == 0 {
			// Pick an existing branch.
			if len(m.branches) == 0 {
				m.mode = modeNormal
				return m, nil
			}
			m.mode = modeAttachBranchPicker
			m.attachBranchCursor = 0
			return m, nil
		}
		// "In new worktree…" → reuse the new-worktree flow.
		m.newBranchWs = m.workspaces[m.wsCursor]
		m.mode = modeNewBranch
		m.newBranch.SetValue("")
		m.newBranch.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

// handleAttachBranchPicker lets the user pick which existing branch inside
// the current workspace should host the new agent.
func (m Model) handleAttachBranchPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeAgentAttachChoice
		return m, nil
	case "j", "down":
		if m.attachBranchCursor < len(m.branches)-1 {
			m.attachBranchCursor++
		}
		return m, nil
	case "k", "up":
		if m.attachBranchCursor > 0 {
			m.attachBranchCursor--
		}
		return m, nil
	case keyEnter:
		if m.attachBranchCursor < 0 || m.attachBranchCursor >= len(m.branches) {
			return m, nil
		}
		br := m.branches[m.attachBranchCursor]
		return m.openAgentPickerFor(br, m.agentPickerTarget), nil
	}
	return m, nil
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
		if b.Path == "" {
			return toastMsg{text: "worktree missing path", level: toastWarn}
		}
		sessName, _, err := ensureSessionForPath(project, b.Name, b.Path)
		if err != nil {
			return toastMsg{text: err.Error(), level: toastError}
		}
		_ = tmux.SwitchClient(sessName)
		return switchDoneMsg{}
	}
}

// createWorktreeAndOpen runs `wt switch --create <branch>` in projPath, then
// ensures exactly one tmux session lives at the resulting worktree path.
// If a session already points at that path (e.g. the worktree or branch
// pre-existed) we reuse it instead of spawning a `-2` duplicate. When the
// caller also requested an agent we add a dedicated window for it — the
// existing window 0 is left alone so we never clobber a running shell.
// Worktree placement is fully delegated to worktrunk.
func (m Model) createWorktreeAndOpen(project, projPath, branch string, agent *agents.Agent, mode agents.AgentMode) tea.Cmd {
	svc := m.stats_svc
	return func() tea.Msg {
		if err := ensureWorktreeBranch(projPath, branch); err != nil {
			return toastMsg{text: err.Error(), level: toastError}
		}
		wts, err := worktree.ListInDir(projPath)
		if err != nil {
			return toastMsg{text: "wt list failed: " + err.Error(), level: toastError}
		}
		if svc != nil {
			_ = svc.Invalidate(projPath)
		}
		wt, ok := findWorktreeByBranch(wts, branch)
		if !ok {
			return actionDoneMsg{}
		}
		return attachSessionAndAgent(project, branch, wt, agent, mode)
	}
}

// ensureWorktreeBranch creates the branch if it doesn't exist yet.
// We probe first because `wt switch --create` against an existing branch can
// generate sibling directories with numeric suffixes.
func ensureWorktreeBranch(projPath, branch string) error {
	existing, err := worktree.ListInDir(projPath)
	if err != nil {
		return fmt.Errorf("wt list failed: %w", err)
	}
	alreadyExists := false
	for _, wt := range existing {
		if wt.Branch == branch {
			alreadyExists = true
			break
		}
	}
	args := []string{"switch", "--no-cd"}
	if !alreadyExists {
		args = append(args, "--create")
	}
	args = append(args, branch)
	cmd := exec.Command("wt", args...)
	cmd.Dir = projPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wt switch failed: %w", err)
	}
	return nil
}

func findWorktreeByBranch(wts []worktree.Worktree, branch string) (worktree.Worktree, bool) {
	for _, wt := range wts {
		if wt.Branch == branch {
			return wt, true
		}
	}
	return worktree.Worktree{}, false
}

func attachSessionAndAgent(project, branch string, wt worktree.Worktree, agent *agents.Agent, mode agents.AgentMode) tea.Msg {
	sessName, freshSession, err := ensureSessionForPath(project, branch, wt.Path)
	if err != nil {
		return toastMsg{text: err.Error(), level: toastError}
	}
	if agent != nil {
		if err := spawnAgentForSession(sessName, wt.Path, *agent, mode, freshSession); err != nil {
			return toastMsg{text: err.Error(), level: toastError}
		}
	}
	_ = tmux.SwitchClient(sessName)
	return switchDoneMsg{}
}

func spawnAgentForSession(sessName, worktreePath string, agent agents.Agent, mode agents.AgentMode, freshSession bool) error {
	command := agent.FullCommand(mode)
	if freshSession {
		// new-session already created window 0; reuse it so the layout stays minimal.
		winTarget := sessName + ":0"
		_ = tmux.RenameWindow(winTarget, agent.ID)
		_ = tmux.SendKeys(winTarget, command)
		return nil
	}
	winName := uniqueWindowName(sessName, agent.ID)
	if err := tmux.NewWindowInSession(sessName, winName, worktreePath, command); err != nil {
		return fmt.Errorf("tmux new-window failed: %w", err)
	}
	return nil
}

// ensureSessionForPath returns the tmux session that already points at
// worktreePath, or creates one named "<project>-<branch>" when none exists.
// The boolean indicates whether a new session was created (true) or an
// existing one was reused (false).
func ensureSessionForPath(project, branch, worktreePath string) (string, bool, error) {
	if sessions, err := tmux.ListSessions(); err == nil {
		for _, s := range sessions {
			if s.Path == worktreePath {
				return s.Name, false, nil
			}
		}
	}
	sessName := uniqueSessName(project + "-" + branch)
	if err := tmux.NewSessionInDir(sessName, worktreePath); err != nil {
		return "", false, fmt.Errorf("tmux new-session failed: %w", err)
	}
	return sessName, true, nil
}

// attachAgentToBranch runs `agent` at `br`. When the branch has a live tmux
// session it adds a new window (or a split, depending on target) so the
// existing shell keeps running. When it doesn't, it creates the session
// first and lands the agent in window 0.
func (m Model) attachAgentToBranch(br branchEntry, agent agents.Agent, mode agents.AgentMode, target agentTarget) tea.Cmd {
	if len(m.workspaces) == 0 {
		return nil
	}
	proj := m.workspaces[m.wsCursor]
	command := agent.FullCommand(mode)
	dir := br.Path
	if dir == "" {
		dir = proj.Path
	}
	branchName := br.Name
	if branchName == "" {
		branchName = "shell"
	}
	sessName := br.SessionName

	return func() tea.Msg {
		if sessName == "" {
			resolved, fresh, err := ensureSessionForPath(proj.Name, branchName, dir)
			if err != nil {
				return toastMsg{text: err.Error(), level: toastError}
			}
			sessName = resolved
			if fresh {
				winTarget := sessName + ":0"
				_ = tmux.RenameWindow(winTarget, agent.ID)
				_ = tmux.SendKeys(winTarget, command)
				_ = tmux.SwitchClient(sessName)
				return switchDoneMsg{}
			}
			// Fell through to the reused-session path below.
		}

		switch target {
		case agentTargetSplit:
			if _, err := tmux.SplitWindowInDir(sessName+":", dir, command); err != nil {
				return toastMsg{text: "tmux split-window failed: " + err.Error(), level: toastError}
			}
		default:
			winName := uniqueWindowName(sessName, agent.ID)
			if err := tmux.NewWindowInSession(sessName, winName, dir, command); err != nil {
				return toastMsg{text: "tmux new-window failed: " + err.Error(), level: toastError}
			}
		}
		_ = tmux.SwitchClient(sessName)
		return switchDoneMsg{}
	}
}

// uniqueWindowName returns a window name that does not collide with an
// existing window in the given session. When probing fails (for example in
// tests) the base name is returned unchanged.
func uniqueWindowName(session, base string) string {
	windows, err := tmux.ListWindows(session)
	if err != nil {
		return base
	}
	taken := make(map[string]bool, len(windows))
	for _, w := range windows {
		taken[w.Name] = true
	}
	if !taken[base] {
		return base
	}
	for i := 2; i <= 99; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !taken[candidate] {
			return candidate
		}
	}
	return base
}
