package worktrees

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/config"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// Model is the worktrees view.
type Model struct {
	worktrees []worktree.Worktree
	cursor    int
	scroll    int
	height    int
	width     int

	confirming       bool // remove confirmation
	describing       bool // create-from-description
	confirmingBranch bool // confirm generated branch name
	creating         bool // direct branch name creation
	describeInput    textinput.Model
	branchInput      textinput.Model
	newInput         textinput.Model
}

func New() Model {
	di := textinput.New()
	di.Prompt = "Describe: "
	di.CharLimit = 128

	bi := textinput.New()
	bi.Prompt = "Branch: "
	bi.CharLimit = 64

	ni := textinput.New()
	ni.Prompt = "Branch: "
	ni.CharLimit = 64

	return Model{
		describeInput: di,
		branchInput:   bi,
		newInput:      ni,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadWorktrees
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// IsEditing returns true when the user is in an input mode.
func (m Model) IsEditing() bool {
	return m.confirming || m.describing || m.confirmingBranch || m.creating
}

type worktreesLoadedMsg struct {
	worktrees []worktree.Worktree
}

func (m Model) loadWorktrees() tea.Msg {
	wts, err := worktree.List()
	if err != nil {
		return worktreesLoadedMsg{}
	}
	return worktreesLoadedMsg{worktrees: wts}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		m.clampCursor()
		return m, nil
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.routeKey(msg)
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if m.IsEditing() {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		return m.handleMouseLeft(msg)
	case tea.MouseButtonWheelUp:
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
	case tea.MouseButtonWheelDown:
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
	}
	return m, nil
}

func (m Model) handleMouseLeft(msg tea.MouseMsg) (Model, tea.Cmd) {
	if msg.Action != tea.MouseActionRelease {
		return m, nil
	}
	row := msg.Y
	if row%2 != 0 {
		return m, nil
	}
	idx := m.scroll + row/2
	if idx < 0 || idx >= len(m.worktrees) {
		return m, nil
	}
	return m, switchWorktreeCmd(m.worktrees[idx].Branch)
}

func (m Model) routeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case m.confirming:
		return m.handleConfirm(msg)
	case m.describing:
		return m.handleDescribe(msg)
	case m.confirmingBranch:
		return m.handleConfirmBranch(msg)
	case m.creating:
		return m.handleCreate(msg)
	default:
		return m.handleNormal(msg)
	}
}

func switchWorktreeCmd(branch string) tea.Cmd {
	return func() tea.Msg { return messages.SwitchWorktreeMsg{Branch: branch} }
}

func (m Model) handleNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	if updated, cmd, handled := m.handleWorktreeNav(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.handleWorktreeAction(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.handleWorktreeDigit(msg); handled {
		return updated, cmd
	}
	if msg.String() == "esc" {
		return m, func() tea.Msg { return messages.SwitchViewMsg{View: "sessions"} }
	}
	return m, nil
}

func (m Model) handleWorktreeNav(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down":
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, nil, true
	case "k", "up":
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, nil, true
	case "g", "home":
		m.cursor = 0
		m.scroll = 0
		return m, nil, true
	case "G", "end":
		m.cursor = len(m.worktrees) - 1
		m.ensureVisible()
		return m, nil, true
	case "enter":
		if wt := m.selected(); wt != nil {
			return m, switchWorktreeCmd(wt.Branch), true
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleWorktreeAction(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "n":
		m.creating = true
		m.newInput.SetValue("")
		m.newInput.Focus()
		return m, textinput.Blink, true
	case "N":
		m.describing = true
		m.describeInput.SetValue("")
		m.describeInput.Focus()
		return m, textinput.Blink, true
	case "d":
		if wt := m.selected(); wt != nil && !wt.IsMain && !wt.IsCurrent {
			m.confirming = true
		}
		return m, nil, true
	case "m":
		return m, popupCmd("wt merge"), true
	case "c":
		return m, popupCmd("wt step commit"), true
	}
	return m, nil, false
}

func popupCmd(command string) tea.Cmd {
	return func() tea.Msg {
		return messages.RunPopupMsg{Command: command, Width: "80%", Height: "80%"}
	}
}

func (m Model) handleWorktreeDigit(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
	default:
		return m, nil, false
	}
	if !digitJumpActive(msg) {
		return m, nil, true
	}
	idx := int(msg.Runes[0]-'0') - 1
	if idx >= 0 && idx < len(m.worktrees) {
		return m, switchWorktreeCmd(m.worktrees[idx].Branch), true
	}
	return m, nil, true
}

func digitJumpActive(msg tea.KeyMsg) bool {
	if config.SuperKey == "none" && !msg.Alt {
		return true
	}
	if config.SuperKey == "alt" && msg.Alt {
		return true
	}
	return false
}

func (m Model) handleDescribe(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.describing = false
		desc := m.describeInput.Value()
		if desc != "" {
			branch := worktree.GenerateBranchName(desc)
			m.branchInput.SetValue(branch)
			m.confirmingBranch = true
			m.branchInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	case "esc":
		m.describing = false
		return m, nil
	}
	var cmd tea.Cmd
	m.describeInput, cmd = m.describeInput.Update(msg)
	return m, cmd
}

func (m Model) handleConfirmBranch(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.confirmingBranch = false
		branch := m.branchInput.Value()
		if branch != "" {
			return m, func() tea.Msg {
				return messages.CreateWorktreeMsg{Branch: branch}
			}
		}
		return m, nil
	case "esc":
		m.confirmingBranch = false
		return m, nil
	}
	var cmd tea.Cmd
	m.branchInput, cmd = m.branchInput.Update(msg)
	return m, cmd
}

func (m Model) handleCreate(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.creating = false
		branch := m.newInput.Value()
		if branch != "" {
			return m, func() tea.Msg {
				return messages.CreateWorktreeMsg{Branch: branch}
			}
		}
		return m, nil
	case "esc":
		m.creating = false
		return m, nil
	}
	var cmd tea.Cmd
	m.newInput, cmd = m.newInput.Update(msg)
	return m, cmd
}

func (m Model) handleConfirm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirming = false
		if wt := m.selected(); wt != nil {
			branch := wt.Branch
			return m, func() tea.Msg {
				return messages.RemoveWorktreeMsg{Branch: branch}
			}
		}
	case "n", "N", "esc":
		m.confirming = false
	}
	return m, nil
}

func (m Model) selected() *worktree.Worktree {
	if m.cursor >= 0 && m.cursor < len(m.worktrees) {
		return &m.worktrees[m.cursor]
	}
	return nil
}

func (m *Model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.worktrees) {
		m.cursor = len(m.worktrees) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) ensureVisible() {
	avail := m.height - 2
	if avail < 1 {
		avail = 1
	}
	maxVisible := (avail + 1) / 2
	if maxVisible < 1 {
		maxVisible = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+maxVisible {
		m.scroll = m.cursor - maxVisible + 1
	}
}

// Reload triggers a worktree reload.
func (m Model) Reload() tea.Cmd {
	return m.loadWorktrees
}
