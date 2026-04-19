package windows

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/config"
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
		if msg.session == m.sessionName {
			m.windows = msg.windows
			m.clampCursor()
		}
		return m, nil
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
	if row < 1 {
		return m, nil
	}
	idx := m.scroll + (row - 1)
	if idx < 0 || idx >= len(m.windows) {
		return m, nil
	}
	return m, switchWindowCmd(m.windows[idx])
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if updated, handled := m.handleNav(msg); handled {
		return updated, nil
	}
	if updated, cmd, handled := m.handleAction(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.handleDigit(msg); handled {
		return updated, cmd
	}
	return m, nil
}

func (m Model) handleNav(msg tea.KeyMsg) (Model, bool) {
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
		m.cursor = len(m.windows) - 1
		m.ensureVisible()
		return m, true
	}
	return m, false
}

func (m Model) handleAction(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		if w := m.selected(); w != nil {
			return m, switchWindowCmd(*w), true
		}
		return m, nil, true
	case "h", "left", "esc":
		return m, func() tea.Msg { return messages.BackToSessionsMsg{} }, true
	}
	return m, nil, false
}

func (m Model) handleDigit(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
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
	if idx >= 0 && idx < len(m.windows) {
		return m, switchWindowCmd(m.windows[idx]), true
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

func switchWindowCmd(w tmux.Window) tea.Cmd {
	target := fmt.Sprintf("%s:%d", w.SessionName, w.Index)
	return func() tea.Msg {
		return messages.SwitchWindowMsg{Target: target}
	}
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
