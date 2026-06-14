package sidepanel

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	switch m.mode {
	case modeDirPicker:
		return m.renderDirPicker()
	case modeAgentPicker:
		return m.renderAgentPicker()
	}

	var b strings.Builder
	b.WriteString(" " + theme.TreeNodeSelected.Render("Sidepanel"))
	b.WriteString("   " + theme.HelpStyle.Render("agents · git · actions"))
	b.WriteString("\n\n")

	b.WriteString(renderChanges(m.project) + "\n")
	b.WriteString("\n")
	b.WriteString(section("Agents") + "\n")
	linesUsed := 5
	for _, line := range m.visibleActivityLines() {
		b.WriteString(line + "\n")
		linesUsed++
	}

	b.WriteString("\n")
	linesUsed++

	b.WriteString(section("Actions") + "\n")
	linesUsed++
	for i, action := range m.actions {
		selected := i == m.cursor
		b.WriteString(renderAction(i, action, selected))
		b.WriteString("\n")
		linesUsed += actionRowHeight
	}

	for linesUsed < m.height-2 {
		b.WriteString("\n")
		linesUsed++
	}

	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	b.WriteString(" " + theme.TreeConnector.Render(strings.Repeat("─", sepW)) + "\n")
	b.WriteString(theme.HelpStyle.Render(" 1-9 action  ⏎ run  r refresh  esc close"))
	return b.String()
}

func (m Model) renderDirPicker() string {
	var b strings.Builder
	b.WriteString(" " + theme.TreeNodeSelected.Render("Launch agent"))
	b.WriteString("\n\n")
	b.WriteString(section("Directory") + "\n")
	b.WriteString(" " + m.dirInput.View() + "\n\n")
	limit := m.maxDirRows()
	end := m.dirScroll + limit
	if end > len(m.filteredDirs) {
		end = len(m.filteredDirs)
	}
	for i := m.dirScroll; i < end; i++ {
		dir := m.filteredDirs[i]
		if i == m.dirCursor {
			b.WriteString(" " + theme.TreeNodeSelected.Render("▸ "+dir.Name) + "\n")
		} else {
			b.WriteString(" " + theme.TreeNodeNormal.Render("  "+dir.Name) + "\n")
		}
		b.WriteString("   " + theme.HelpStyle.Render(dir.Path) + "\n")
	}
	if hidden := len(m.filteredDirs) - end; hidden > 0 {
		b.WriteString(" " + theme.HelpStyle.Render(fmt.Sprintf("+%d more", hidden)) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" type to search  ↑/↓ move  ⏎ choose  esc cancel"))
	return b.String()
}

func (m Model) renderAgentPicker() string {
	var b strings.Builder
	b.WriteString(" " + theme.TreeNodeSelected.Render("Choose agent"))
	b.WriteString("\n")
	b.WriteString(" " + theme.HelpStyle.Render(m.selectedDir.Path) + "\n\n")
	limit := m.height - 6
	if limit < 1 {
		limit = 1
	}
	for i, agent := range m.agentList {
		if i >= limit {
			break
		}
		modeName := agent.Modes[m.agentModeIndex[i]].Name
		if i == m.agentCursor {
			b.WriteString(" " + theme.TreeNodeSelected.Render("▸ "+agent.Name) + "\n")
		} else {
			b.WriteString(" " + theme.TreeNodeNormal.Render("  "+agent.Name) + "\n")
		}
		b.WriteString("   " + theme.HelpStyle.Render(modeName+" mode") + "\n")
	}
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" tab mode  ⏎ choose  esc back"))
	return b.String()
}

func section(title string) string {
	return " " + theme.HelpStyle.Render(title)
}

func renderChanges(stats projectStats) string {
	if stats.Err != "" {
		return " " + theme.TreeMeta.Render("Git") + " " + theme.HelpStyle.Render(stats.Err)
	}
	if stats.Added == 0 && stats.Deleted == 0 && stats.Staged == 0 && stats.Unstaged == 0 && stats.Untracked == 0 {
		return " " + theme.TreeMeta.Render("Git") + " " + theme.TreeNodeNormal.Render("clean")
	}
	var parts []string
	if stats.Added > 0 {
		parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", stats.Added)))
	}
	if stats.Deleted > 0 {
		parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", stats.Deleted)))
	}
	if stats.Staged > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", stats.Staged))
	}
	if stats.Unstaged > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", stats.Unstaged))
	}
	if stats.Untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", stats.Untracked))
	}
	return " " + theme.TreeMeta.Render("Git") + " " + theme.TreeNodeNormal.Render(strings.Join(parts, ", "))
}

func renderAction(index int, a action, selected bool) string {
	prefix := " "
	if index < 9 {
		prefix = theme.TreeMeta.Render(fmt.Sprintf("%d", index+1))
	}
	if selected {
		return fmt.Sprintf("%s%s\n   %s",
			prefix,
			theme.TreeNodeSelected.Render("▸ "+a.title),
			theme.HelpStyle.Render(a.description),
		)
	}
	return fmt.Sprintf("%s%s\n   %s",
		prefix,
		theme.TreeNodeNormal.Render("  "+a.title),
		theme.HelpStyle.Render(a.description),
	)
}

func renderActivity(activities []agentActivity) []string {
	if len(activities) == 0 {
		return []string{" " + theme.HelpStyle.Render("No active agents detected")}
	}
	lines := make([]string, 0, len(activities)*2)
	for _, activity := range activities {
		pane := activity.Pane
		target := fmt.Sprintf("%s:%d.%d", pane.SessionName, pane.WindowIndex, pane.PaneIndex)
		status := string(activity.Status)
		if activity.NeedsInput {
			status = "needs input"
		}
		lines = append(lines, fmt.Sprintf(" %s %s  %s",
			theme.TreeMeta.Render(status),
			theme.TreeNodeNormal.Render(pane.Command),
			theme.HelpStyle.Render(target),
		))
		detail := activity.Description
		if pane.Path != "" {
			detail = shortPath(pane.Path)
		}
		if detail != "" {
			lines = append(lines, "   "+theme.HelpStyle.Render(detail))
		}
	}
	return lines
}

func (m Model) visibleActivityLines() []string {
	lines := renderActivity(m.activities)
	maxRows := m.height - 10 - len(m.actions)*actionRowHeight
	if maxRows < 1 {
		maxRows = 1
	}
	if len(lines) <= maxRows {
		return lines
	}
	visible := append([]string{}, lines[:maxRows]...)
	visible = append(visible, " "+theme.HelpStyle.Render(fmt.Sprintf("+%d more", len(lines)-maxRows)))
	return visible
}

func shortPath(path string) string {
	dir := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	if parent == "." || parent == string(filepath.Separator) || parent == "" {
		return dir
	}
	return filepath.Join(parent, dir)
}
