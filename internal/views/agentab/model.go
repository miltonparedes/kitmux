package agentab

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/app/messages"
)

type Model struct {
	promptInput textinput.Model
	planMode    bool
	width       int
	height      int
}

func New() Model {
	in := textinput.New()
	in.Prompt = "Prompt: "
	in.CharLimit = 2000
	in.Focus()
	return Model{
		promptInput: in,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m Model) IsEditing() bool { return true }

func (m *Model) Reset() {
	m.promptInput.SetValue("")
	m.promptInput.Focus()
	m.planMode = false
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return messages.BackFromAgentABMsg{}
			}
		case "tab":
			m.planMode = !m.planMode
			return m, nil
		case "enter":
			prompt := strings.TrimSpace(m.promptInput.Value())
			if prompt == "" {
				return m, nil
			}
			return m, func() tea.Msg {
				return messages.LaunchAgentABMsg{
					Prompt:   prompt,
					PlanMode: m.planMode,
				}
			}
		}
	}

	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}
