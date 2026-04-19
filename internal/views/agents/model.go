package agentsview

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/config"
)

// Model is the agents list view.
type Model struct {
	agents    []agents.Agent
	cursor    int
	modeIndex []int // per-agent selected mode index
	height    int
	width     int
	scroll    int
}

func New() Model {
	agentList := agents.DefaultAgents()
	return Model{
		agents:    agentList,
		modeIndex: make([]int, len(agentList)),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// IsEditing returns false; the agents view has no input states.
func (m Model) IsEditing() bool { return false }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
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
	if idx < 0 || idx >= len(m.agents) {
		return m, nil
	}
	m.cursor = idx
	return m, m.launchCmd("pane")
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if updated, handled := m.handleAgentNav(msg); handled {
		return updated, nil
	}
	if updated, cmd, handled := m.handleAgentAction(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.handleAgentDigit(msg); handled {
		return updated, cmd
	}
	if msg.String() == "esc" {
		return m, func() tea.Msg { return messages.SwitchViewMsg{View: "sessions"} }
	}
	return m, nil
}

func (m Model) handleAgentNav(msg tea.KeyMsg) (Model, bool) {
	switch msg.String() {
	case "j", "down":
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, true
	case "k", "up":
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, true
	case "g", "home":
		m.cursor = 0
		m.scroll = 0
		return m, true
	case "G", "end":
		m.cursor = len(m.agents) - 1
		m.ensureVisible()
		return m, true
	case "tab":
		m.rotateMode(1)
		return m, true
	case "shift+tab":
		m.rotateMode(-1)
		return m, true
	}
	return m, false
}

func (m *Model) rotateMode(delta int) {
	if len(m.agents) == 0 {
		return
	}
	a := m.agents[m.cursor]
	n := len(a.Modes)
	m.modeIndex[m.cursor] = (m.modeIndex[m.cursor] + delta + n) % n
}

func (m Model) handleAgentAction(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		return m, m.launchCmd("pane"), true
	case "s":
		return m, m.launchCmd("split"), true
	case "w":
		return m, m.launchCmd("window"), true
	case "A":
		return m, func() tea.Msg { return messages.OpenAgentABMsg{Source: "agents"} }, true
	}
	return m, nil, false
}

func (m Model) handleAgentDigit(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
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
	if idx >= 0 && idx < len(m.agents) {
		m.cursor = idx
		return m, m.launchCmd("pane"), true
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

func (m Model) launchCmd(target string) tea.Cmd {
	if len(m.agents) == 0 {
		return nil
	}
	a := m.agents[m.cursor]
	mode := a.Modes[m.modeIndex[m.cursor]]
	return func() tea.Msg {
		return messages.LaunchAgentMsg{
			AgentID: a.ID,
			ModeID:  mode.ID,
			Target:  target,
		}
	}
}

func (m *Model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.agents) {
		m.cursor = len(m.agents) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) ensureVisible() {
	// Each item takes 2 lines (item + separator), last takes 1.
	// Available lines = height - 2 (footer sep + help)
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
