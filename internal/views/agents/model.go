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
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.cursor++
			m.clampCursor()
			m.ensureVisible()
		case "k", "up":
			m.cursor--
			m.clampCursor()
			m.ensureVisible()
		case "g", "home":
			m.cursor = 0
			m.scroll = 0
		case "G", "end":
			m.cursor = len(m.agents) - 1
			m.ensureVisible()

		case "tab":
			if len(m.agents) > 0 {
				a := m.agents[m.cursor]
				m.modeIndex[m.cursor] = (m.modeIndex[m.cursor] + 1) % len(a.Modes)
			}
		case "shift+tab":
			if len(m.agents) > 0 {
				a := m.agents[m.cursor]
				m.modeIndex[m.cursor] = (m.modeIndex[m.cursor] - 1 + len(a.Modes)) % len(a.Modes)
			}

		case "enter":
			return m, m.launchCmd("pane")
		case "s":
			return m, m.launchCmd("split")
		case "w":
			return m, m.launchCmd("window")

		case "1", "2", "3", "4", "5", "6", "7", "8", "9",
			"alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
			if config.SuperKey == "none" && !msg.Alt || config.SuperKey == "alt" && msg.Alt {
				idx := int(msg.Runes[0]-'0') - 1
				if idx < len(m.agents) {
					m.cursor = idx
					return m, m.launchCmd("pane")
				}
			}

		case "esc":
			return m, func() tea.Msg {
				return messages.SwitchViewMsg{View: "sessions"}
			}
		}
	}
	return m, nil
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
