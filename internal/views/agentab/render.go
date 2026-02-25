package agentab

import (
	"strings"

	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	var b strings.Builder

	title := " A/B Launch: Codex + Claude"
	b.WriteString(theme.TreeNodeSelected.Render(title))
	b.WriteString("\n\n")
	b.WriteString(" ")
	b.WriteString(m.promptInput.View())
	b.WriteString("\n\n")

	plan := "OFF"
	if m.planMode {
		plan = "ON"
	}
	b.WriteString(" ")
	b.WriteString(theme.HelpStyle.Render("Plan mode: " + plan + " (TAB toggle)"))
	b.WriteString("\n")

	used := 6
	for used < m.height-2 {
		b.WriteString("\n")
		used++
	}

	b.WriteString(theme.HelpStyle.Render(" ⏎ launch  ⇥ plan  esc back  q quit"))
	return b.String()
}
