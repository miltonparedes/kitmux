package windows

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

type Model struct {
	sessionName string
	windows     []tmux.Window
	cursor      int
	scroll      int
	height      int
	width       int
}

func New() Model {
	return Model{}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SessionName returns the currently loaded session name.
func (m Model) SessionName() string {
	return m.sessionName
}

type windowsLoadedMsg struct {
	session string
	windows []tmux.Window
}

// LoadSession loads windows for the given session (called by app on cursor change).
func (m *Model) LoadSession(name string) tea.Cmd {
	if name == m.sessionName {
		return nil // already loaded
	}
	m.sessionName = name
	m.cursor = 0
	m.scroll = 0
	m.windows = nil
	return func() tea.Msg {
		wins, err := tmux.ListWindows(name)
		if err != nil {
			return windowsLoadedMsg{session: name}
		}
		return windowsLoadedMsg{session: name, windows: wins}
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case windowsLoadedMsg:
		// Only apply if it matches current session (avoid stale loads)
		if msg.session == m.sessionName {
			m.windows = msg.windows
			m.clampCursor()
		}

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
			m.cursor = len(m.windows) - 1
			m.ensureVisible()

		case "enter":
			if w := m.selected(); w != nil {
				target := fmt.Sprintf("%s:%d", w.SessionName, w.Index)
				return m, func() tea.Msg {
					return messages.SwitchWindowMsg{Target: target}
				}
			}

		case "h", "left", "esc":
			return m, func() tea.Msg {
				return messages.BackToSessionsMsg{}
			}

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if msg.Alt {
				idx := int(msg.String()[0]-'0') - 1
				if idx < len(m.windows) {
					m.cursor = idx
					m.ensureVisible()
				}
			}
		}
	}
	return m, nil
}

func (m Model) selected() *tmux.Window {
	if m.cursor >= 0 && m.cursor < len(m.windows) {
		return &m.windows[m.cursor]
	}
	return nil
}

func (m *Model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.windows) {
		m.cursor = len(m.windows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) ensureVisible() {
	viewHeight := m.height
	if viewHeight < 1 {
		viewHeight = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+viewHeight {
		m.scroll = m.cursor - viewHeight + 1
	}
}
