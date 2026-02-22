package palette

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/app/messages"
)

type Model struct {
	commands []Command
	filtered []Command
	input    textinput.Model
	cursor   int
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
	m.filtered = m.commands
	m.cursor = 0
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.cursor >= 0 && m.cursor < len(m.filtered) {
				cmd := m.filtered[m.cursor]
				return m, func() tea.Msg {
					return messages.ExecuteCommandMsg{ID: cmd.ID}
				}
			}

		case "up", "ctrl+k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = 0
			}
			return m, nil

		case "down", "ctrl+j":
			m.cursor++
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Re-filter on every keystroke
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

	return m, cmd
}
