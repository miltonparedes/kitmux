package palette

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/recency"
)

type Model struct {
	commands []Command
	filtered []Command
	input    textinput.Model
	cursor   int
	scroll   int
	height   int
	width    int
}

func New() Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Type a command..."
	ti.CharLimit = 64
	ti.Focus()

	cmds := DefaultCommands()
	return Model{
		commands: cmds,
		filtered: cmds,
		input:    ti,
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) Reset() {
	m.input.SetValue("")
	m.input.Focus()
	store := recency.Load()
	m.commands = recency.SortByRecency(DefaultCommands(), store.Commands, func(c Command) string {
		return c.ID
	})
	m.filtered = m.commands
	m.cursor = 0
	m.scroll = 0
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		if updated, cmd, handled := m.handleKey(msg); handled {
			return updated, cmd
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refilter()
	return m, cmd
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonLeft:
		return m.handleMouseLeft(msg)
	case tea.MouseButtonWheelUp:
		m.moveCursor(-1)
	case tea.MouseButtonWheelDown:
		m.moveCursor(1)
	}
	return m, nil
}

func (m Model) handleMouseLeft(msg tea.MouseMsg) (Model, tea.Cmd) {
	if msg.Action != tea.MouseActionRelease {
		return m, nil
	}
	row := msg.Y
	if row < 2 || (row-2)%2 != 0 {
		return m, nil
	}
	idx := m.scroll + (row-2)/2
	if idx < 0 || idx >= len(m.filtered) {
		return m, nil
	}
	return m, executeCmdByID(m.filtered[idx].ID)
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		return m.handleEnter()
	case "up", "ctrl+k":
		m.moveCursor(-1)
		return m, nil, true
	case "down", "ctrl+j":
		m.moveCursor(1)
		return m, nil, true
	case "alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
		return m.handleAltDigit(msg)
	}
	return m, nil, false
}

func (m Model) handleEnter() (Model, tea.Cmd, bool) {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return m, nil, false
	}
	cmd := m.filtered[m.cursor]
	return m, executeCmdByID(cmd.ID), true
}

func (m Model) handleAltDigit(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	idx := int(msg.Runes[0]-'0') - 1
	if idx < 0 || idx >= len(m.filtered) {
		return m, nil, true
	}
	cmd := m.filtered[idx]
	return m, executeCmdByID(cmd.ID), true
}

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureVisible()
}

func (m *Model) refilter() {
	query := m.input.Value()
	if query == "" {
		m.filtered = m.commands
	} else {
		titles := make([]string, len(m.commands))
		for i, c := range m.commands {
			titles[i] = c.Title
		}
		matches := fuzzy.Find(query, titles)
		m.filtered = make([]Command, len(matches))
		for i, match := range matches {
			m.filtered[i] = m.commands[match.Index]
		}
	}
	m.cursor = 0
	m.scroll = 0
}

func executeCmdByID(id string) tea.Cmd {
	return func() tea.Msg {
		return messages.ExecuteCommandMsg{ID: id}
	}
}

func (m Model) maxVisible() int {
	avail := m.height - 3
	if avail < 1 {
		avail = 1
	}
	return (avail + 1) / 2
}

func (m *Model) ensureVisible() {
	visible := m.maxVisible()
	if visible < 1 {
		visible = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+visible {
		m.scroll = m.cursor - visible + 1
	}
}
