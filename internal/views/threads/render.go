package threads

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	if m.picking {
		return m.pickerView()
	}

	var b strings.Builder
	b.WriteString(" " + theme.TreeNodeSelected.Render("Agent Threads") + "\n")

	viewHeight := m.height - 2
	if viewHeight < 1 {
		viewHeight = 1
	}

	if len(m.rows) == 0 {
		b.WriteString(theme.HelpStyle.Render(" no running agents"))
		for i := 1; i < viewHeight; i++ {
			b.WriteString("\n")
		}
	} else {
		m.writeRows(&b, viewHeight)
	}

	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ open  n new  d/K kill headless  r refresh  q quit"))
	return b.String()
}

func (m Model) pickerView() string {
	var b strings.Builder
	b.WriteString(" " + theme.TreeNodeSelected.Render("New Headless Agent") + "\n")
	viewHeight := m.height - 2
	if viewHeight < 1 {
		viewHeight = 1
	}

	end := viewHeight
	if end > len(m.agents) {
		end = len(m.agents)
	}
	for i := 0; i < end; i++ {
		agent := m.agents[i]
		if i == m.agentIndex {
			b.WriteString(theme.TreeNodeSelected.Render("› " + agent.DisplayName()))
		} else {
			b.WriteString("  " + theme.TreeNodeNormal.Render(agent.DisplayName()))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	for rendered := end; rendered < viewHeight; rendered++ {
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ create  esc back"))
	return b.String()
}

func (m Model) writeRows(b *strings.Builder, viewHeight int) {
	start := m.scroll
	end := start + viewHeight
	if end > len(m.rows) {
		end = len(m.rows)
	}
	for i := start; i < end; i++ {
		m.writeRow(b, i)
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	for rendered := end - start; rendered < viewHeight; rendered++ {
		b.WriteString("\n")
	}
}

func (m Model) writeRow(b *strings.Builder, i int) {
	row := m.rows[i]
	selected := i == m.cursor
	label := row.AgentName
	if row.SessionName != "" {
		label += " " + theme.TreeMeta.Render(row.SessionName)
	}

	badge := theme.AgentMode.Render("ephemeral")
	if row.Kind == RowHeadless {
		badge = theme.AgentModeSelected.Render("headless")
	}
	if row.Attached {
		badge += " " + theme.AttachedBadge.Render("●")
	}

	path := shortPath(row.Path)
	if path != "" {
		label += " " + theme.TreeMeta.Render(path)
	}

	if row.Kind == RowEphemeral {
		label += " " + theme.TreeMeta.Render(fmt.Sprintf("%d.%d", row.WindowIndex, row.PaneIndex))
	}

	prefix := " "
	if i < 9 {
		prefix = theme.TreeMeta.Render(fmt.Sprintf("%d", i+1))
	}
	b.WriteString(prefix + " ")
	if selected {
		b.WriteString(theme.TreeNodeSelected.Render(label))
	} else {
		b.WriteString(theme.TreeNodeNormal.Render(label))
	}
	b.WriteString(" " + badge)
}

func shortPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(filepath.Clean(path))
}
