package workbench

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
	workspacesreg "github.com/miltonparedes/kitmux/internal/workspaces"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

var execCommand = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

type actionKind int

const (
	actionSwitchView actionKind = iota
	actionExecuteCommand
	actionRunPopup
	actionLaunchAgent
)

type action struct {
	title       string
	description string
	kind        actionKind
	value       string
}

type mode int

const (
	modeNormal mode = iota
	modeDirPicker
	modeAgentPicker
)

type dirEntry struct {
	Name string
	Path string
}

const actionRowHeight = 2

// Model is the compact agent sidecar panel.
type Model struct {
	actions []action
	project projectStats
	panes   []tmux.Pane

	mode     mode
	cursor   int
	width    int
	height   int
	dirInput textinput.Model

	dirs         []dirEntry
	filteredDirs []dirEntry
	dirCursor    int
	dirScroll    int
	selectedDir  dirEntry

	agentList      []agents.Agent
	agentCursor    int
	agentModeIndex []int
}

type panesLoadedMsg struct {
	panes []tmux.Pane
}

type projectStatsLoadedMsg struct {
	stats projectStats
}

type dirsLoadedMsg struct {
	dirs []dirEntry
}

func New() Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "select directory..."
	ti.CharLimit = 128
	agentList := agents.DefaultAgents()
	return Model{
		actions: []action{
			{title: "Launch Agent", description: "Choose directory and agent", kind: actionLaunchAgent},
			{title: "Editor", description: "Open current session locally", kind: actionExecuteCommand, value: "open_local_editor"},
			{title: "Lazygit", description: "Open lazygit popup", kind: actionRunPopup, value: "lazygit"},
			{title: "Lumen Diff", description: "Open lumen diff popup", kind: actionRunPopup, value: "lumen diff"},
			{title: "Worktrees", description: "Open worktree manager", kind: actionSwitchView, value: "worktrees"},
			{title: "Sessions", description: "Open session tree", kind: actionSwitchView, value: "sessions"},
		},
		dirInput:       ti,
		agentList:      agentList,
		agentModeIndex: make([]int, len(agentList)),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.Reload(), m.LoadProject())
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m Model) IsEditing() bool { return m.mode != modeNormal }

func (m Model) Reload() tea.Cmd {
	return func() tea.Msg {
		panes, err := tmux.ListPanes()
		if err != nil {
			return panesLoadedMsg{}
		}
		return panesLoadedMsg{panes: panes}
	}
}

func (m Model) LoadProject() tea.Cmd {
	return func() tea.Msg {
		return projectStatsLoadedMsg{stats: loadProjectStats()}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case panesLoadedMsg:
		m.panes = filterActivityPanes(msg.panes)
		return m, nil
	case projectStatsLoadedMsg:
		m.project = msg.stats
		return m, nil
	case dirsLoadedMsg:
		m.dirs = msg.dirs
		m.refilterDirs()
		return m, nil
	case messages.WorkbenchCommandDoneMsg:
		return m, tea.Batch(m.Reload(), m.LoadProject())
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if m.mode != modeNormal {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionRelease {
			return m, nil
		}
		if idx, ok := m.rowToCursor(msg.Y); ok {
			m.cursor = idx
			if a, ok := m.selectedAction(); ok && a.kind == actionLaunchAgent {
				updated := m.startAgentLaunch()
				return updated, tea.Batch(textinput.Blink, updated.loadDirs())
			}
			return m, m.selectedCmd()
		}
	case tea.MouseButtonWheelUp:
		m.cursor--
		m.clampCursor()
	case tea.MouseButtonWheelDown:
		m.cursor++
		m.clampCursor()
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.mode {
	case modeDirPicker:
		return m.handleDirPickerKey(msg)
	case modeAgentPicker:
		return m.handleAgentPickerKey(msg)
	}
	switch msg.String() {
	case "j", "down":
		m.cursor++
		m.clampCursor()
		return m, nil
	case "k", "up":
		m.cursor--
		m.clampCursor()
		return m, nil
	case "g", "home":
		m.cursor = 0
		return m, nil
	case "G", "end":
		m.cursor = m.selectableCount() - 1
		m.clampCursor()
		return m, nil
	case "enter":
		if a, ok := m.selectedAction(); ok && a.kind == actionLaunchAgent {
			updated := m.startAgentLaunch()
			return updated, tea.Batch(textinput.Blink, updated.loadDirs())
		}
		return m, m.selectedCmd()
	case "r":
		return m, tea.Batch(m.Reload(), m.LoadProject())
	case "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleDirPickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.dirInput.Blur()
		return m, nil
	case "enter":
		if len(m.filteredDirs) == 0 {
			return m, nil
		}
		m.selectedDir = m.filteredDirs[m.dirCursor]
		m.mode = modeAgentPicker
		m.agentCursor = 0
		return m, nil
	case "down", "ctrl+j":
		if m.dirCursor < len(m.filteredDirs)-1 {
			m.dirCursor++
		}
		m.ensureDirVisible()
		return m, nil
	case "up", "ctrl+k":
		if m.dirCursor > 0 {
			m.dirCursor--
		}
		m.ensureDirVisible()
		return m, nil
	}
	var cmd tea.Cmd
	m.dirInput, cmd = m.dirInput.Update(msg)
	m.refilterDirs()
	return m, cmd
}

func (m Model) handleAgentPickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeDirPicker
		m.dirInput.Focus()
		return m, nil
	case "down", "ctrl+j":
		if m.agentCursor < len(m.agentList)-1 {
			m.agentCursor++
		}
		return m, nil
	case "up", "ctrl+k":
		if m.agentCursor > 0 {
			m.agentCursor--
		}
		return m, nil
	case "tab":
		if len(m.agentList) > 0 {
			a := m.agentList[m.agentCursor]
			m.agentModeIndex[m.agentCursor] = (m.agentModeIndex[m.agentCursor] + 1) % len(a.Modes)
		}
		return m, nil
	case "enter":
		if len(m.agentList) == 0 {
			return m, nil
		}
		if m.selectedDir.Path == "" {
			return m, nil
		}
		a := m.agentList[m.agentCursor]
		mode := a.Modes[m.agentModeIndex[m.agentCursor]]
		m.mode = modeNormal
		return m, func() tea.Msg {
			return messages.LaunchWorkbenchAgentMsg{AgentID: a.ID, ModeID: mode.ID, Dir: m.selectedDir.Path}
		}
	}
	return m, nil
}

func (m Model) selectedCmd() tea.Cmd {
	a, ok := m.selectedAction()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		switch a.kind {
		case actionSwitchView:
			return messages.SwitchViewMsg{View: a.value}
		case actionExecuteCommand:
			return messages.ExecuteCommandMsg{ID: a.value}
		case actionRunPopup:
			return messages.RunPopupMsg{Command: a.value, Width: "100%", Height: "100%", Stay: true}
		default:
			return nil
		}
	}
}

func (m Model) selectedAction() (action, bool) {
	if m.cursor < 0 {
		return action{}, false
	}
	if m.cursor < len(m.actions) {
		return m.actions[m.cursor], true
	}
	return action{}, false
}

func (m Model) rowToCursor(row int) (int, bool) {
	actionStart := m.firstActionRow()
	actionEnd := actionStart + len(m.actions)*actionRowHeight
	if row >= actionStart && row < actionEnd {
		return (row - actionStart) / actionRowHeight, true
	}
	return 0, false
}

func (m Model) firstActionRow() int {
	return 16 + m.artifactRows()
}

func (m Model) artifactRows() int {
	if m.project.Err != "" || len(m.project.ChangedFiles) == 0 {
		return 1
	}
	if len(m.project.ChangedFiles) > 2 {
		return 3
	}
	return len(m.project.ChangedFiles)
}

func (m Model) selectableCount() int {
	return len(m.actions)
}

func (m *Model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= m.selectableCount() {
		m.cursor = m.selectableCount() - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) startAgentLaunch() Model {
	m.mode = modeDirPicker
	m.dirs = buildDirEntries(m.project.Path)
	m.filteredDirs = m.dirs
	m.dirCursor = 0
	m.dirScroll = 0
	m.dirInput.SetValue("")
	m.dirInput.Focus()
	return m
}

func (m Model) StartAgentLaunchForTest() Model {
	return m.startAgentLaunch()
}

func (m Model) loadDirs() tea.Cmd {
	current := m.project.Path
	return func() tea.Msg {
		return dirsLoadedMsg{dirs: buildDirEntries(current)}
	}
}

func (m *Model) refilterDirs() {
	query := m.dirInput.Value()
	if query == "" {
		m.filteredDirs = m.dirs
	} else {
		items := make([]string, len(m.dirs))
		for i, d := range m.dirs {
			items[i] = d.Name + " " + d.Path
		}
		matches := fuzzy.Find(query, items)
		m.filteredDirs = make([]dirEntry, len(matches))
		for i, match := range matches {
			m.filteredDirs[i] = m.dirs[match.Index]
		}
	}
	m.dirCursor = 0
	m.dirScroll = 0
}

func (m Model) maxDirRows() int {
	limit := m.height - 9
	if limit > 5 {
		limit = 5
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}

func (m *Model) ensureDirVisible() {
	visible := m.maxDirRows()
	if m.dirCursor < m.dirScroll {
		m.dirScroll = m.dirCursor
	}
	if m.dirCursor >= m.dirScroll+visible {
		m.dirScroll = m.dirCursor - visible + 1
	}
	if m.dirScroll < 0 {
		m.dirScroll = 0
	}
}

func buildDirEntries(current string) []dirEntry {
	seen := make(map[string]bool)
	var dirs []dirEntry
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		dirs = append(dirs, dirEntry{Name: filepath.Base(path), Path: path})
	}
	add(current)
	addSessionDirs(add, current)
	addWorktreeDirs(add, current)
	for _, ws := range workspacesreg.LoadRegistry() {
		add(ws.Path)
	}
	addZoxideDirs(add)
	return dirs
}

func addSessionDirs(add func(string), current string) {
	sessions, err := tmux.ListSessions()
	if err != nil {
		return
	}
	for _, session := range sessions {
		if current == "" || strings.HasPrefix(session.Path, current) || strings.HasPrefix(current, session.Path) {
			add(session.Path)
		}
	}
}

func addWorktreeDirs(add func(string), current string) {
	if current == "" {
		return
	}
	worktrees, err := worktree.ListInDir(current)
	if err != nil {
		return
	}
	for _, wt := range worktrees {
		add(wt.Path)
	}
}

func addZoxideDirs(add func(string)) {
	out, err := execCommand("zoxide", "query", "-ls")
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.IndexByte(line, ' ')
		if idx < 0 {
			continue
		}
		add(strings.TrimSpace(line[idx+1:]))
	}
}

func filterActivityPanes(panes []tmux.Pane) []tmux.Pane {
	commands := map[string]bool{
		"claude":   true,
		"codex":    true,
		"gemini":   true,
		"aichat":   true,
		"opencode": true,
	}
	filtered := make([]tmux.Pane, 0, len(panes))
	for _, pane := range panes {
		if commands[pane.Command] {
			filtered = append(filtered, pane)
		}
	}
	return filtered
}
