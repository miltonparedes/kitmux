package agentsview

import (
	"fmt"
	"strings"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	if len(m.agents) == 0 {
		return theme.HelpStyle.Render(" No agents configured")
	}

	var b strings.Builder

	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	itemSep := " " + theme.TreeMeta.Render(strings.Repeat("─", sepW))

	// Each item = 2 lines (item + sep), last = 1. Footer = 2 lines (sep + help).
	// Available = height - 2
	avail := m.height - 2
	if avail < 1 {
		avail = 1
	}
	maxVisible := (avail + 1) / 2

	start := m.scroll
	end := start + maxVisible
	if end > len(m.agents) {
		end = len(m.agents)
	}

	for i := start; i < end; i++ {
		a := m.agents[i]
		selected := i == m.cursor
		b.WriteString(renderAgent(a, m.modeIndex[i], selected))
		b.WriteString("\n")

		if i < end-1 {
			b.WriteString(itemSep)
			b.WriteString("\n")
		}
	}

	// Pad
	rendered := end - start
	linesUsed := rendered*2 - 1
	if rendered == 0 {
		linesUsed = 0
	}
	for linesUsed < avail {
		b.WriteString("\n")
		linesUsed++
	}

	// Footer
	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(m.StatusLine())

	return b.String()
}

// StatusLine returns the footer content.
func (m Model) StatusLine() string {
	return theme.HelpStyle.Render(" ⏎ launch  s split  w window  ⇥ mode  q quit")
}

func renderAgent(a agents.Agent, modeIdx int, selected bool) string {
	modeName := a.Modes[modeIdx].Name

	if selected {
		name := theme.TreeNodeSelected.Render("▸ " + a.Name)
		mode := theme.AgentModeSelected.Render(modeName)
		return fmt.Sprintf(" %s  %s", name, mode)
	}

	name := theme.TreeNodeNormal.Render("  " + a.Name)
	mode := theme.AgentMode.Render(modeName)
	return fmt.Sprintf(" %s  %s", name, mode)
}
