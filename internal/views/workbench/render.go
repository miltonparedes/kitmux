package workbench

import (
	"fmt"
	"strings"

	"github.com/miltonparedes/kitmux/internal/theme"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

func (m Model) View() string {
	switch m.mode {
	case modeDirPicker:
		return m.renderDirPicker()
	case modeAgentPicker:
		return m.renderAgentPicker()
	}

	var b strings.Builder
	b.WriteString(" " + theme.TreeNodeSelected.Render("Summary"))
	b.WriteString("   " + theme.HelpStyle.Render("Review"))
	b.WriteString("   " + theme.HelpStyle.Render("+"))
	b.WriteString("\n\n")

	b.WriteString(section("Progress") + "\n")
	b.WriteString(renderProgress(m.panes) + "\n\n")

	b.WriteString(section("Branch details") + "\n")
	b.WriteString(renderBranch(m.project) + "\n")
	b.WriteString(renderChanges(m.project) + "\n")
	b.WriteString(renderProjectStats(m.project) + "\n\n")

	b.WriteString(section("Artifacts") + "\n")
	for _, line := range renderArtifacts(m.project) {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")

	b.WriteString(section("Sources") + "\n")
	b.WriteString(renderSources(m.project) + "\n\n")

	b.WriteString(section("Tools") + "\n")
	linesUsed := m.firstActionRow()
	for i, action := range m.actions {
		selected := i == m.cursor
		b.WriteString(renderAction(action, selected))
		b.WriteString("\n")
		linesUsed += actionRowHeight
	}

	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" Activity") + "\n")
	linesUsed += 2

	if len(m.panes) == 0 {
		b.WriteString(theme.HelpStyle.Render(" no active agents detected"))
		b.WriteString("\n")
		linesUsed++
	} else {
		maxActivity := m.height - linesUsed - 2
		if maxActivity < 1 {
			maxActivity = 1
		}
		for i, pane := range m.panes {
			if i >= maxActivity {
				break
			}
			b.WriteString(renderPane(pane))
			b.WriteString("\n")
			linesUsed++
		}
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
	b.WriteString(theme.HelpStyle.Render(" ⏎ run  r refresh  esc close"))
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

func renderProgress(panes []tmux.Pane) string {
	if len(panes) == 0 {
		return " " + theme.HelpStyle.Render("No active agent progress")
	}
	label := "agent"
	if len(panes) != 1 {
		label = "agents"
	}
	return fmt.Sprintf(" %s", theme.TreeNodeNormal.Render(fmt.Sprintf("%d active %s", len(panes), label)))
}

func renderBranch(stats projectStats) string {
	if stats.Err != "" {
		return " " + theme.HelpStyle.Render(stats.Err)
	}
	return fmt.Sprintf(" %s  %s",
		theme.TreeMeta.Render("branch"),
		theme.TreeNodeNormal.Render(stats.Branch),
	)
}

func renderChanges(stats projectStats) string {
	if stats.Err != "" {
		return " " + theme.HelpStyle.Render("Changes unavailable")
	}
	if stats.Staged == 0 && stats.Unstaged == 0 && stats.Untracked == 0 {
		return " " + theme.TreeMeta.Render("changes") + " " + theme.TreeNodeNormal.Render("clean")
	}
	var parts []string
	if stats.Staged > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", stats.Staged))
	}
	if stats.Unstaged > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", stats.Unstaged))
	}
	if stats.Untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", stats.Untracked))
	}
	return " " + theme.TreeMeta.Render("changes") + " " + theme.TreeNodeNormal.Render(strings.Join(parts, ", "))
}

func renderProjectStats(stats projectStats) string {
	if stats.Err != "" {
		return " " + theme.HelpStyle.Render("Stats unavailable")
	}
	diff := ""
	if stats.Added > 0 || stats.Deleted > 0 {
		diff = fmt.Sprintf("  %s", theme.HelpStyle.Render(fmt.Sprintf("+%d -%d", stats.Added, stats.Deleted)))
	}
	return fmt.Sprintf(" %s  %s files  %s lines%s",
		theme.TreeMeta.Render(stats.Name),
		formatInt(stats.Files),
		formatInt(stats.Lines),
		diff,
	)
}

func renderArtifacts(stats projectStats) []string {
	if stats.Err != "" {
		return []string{" " + theme.HelpStyle.Render("No artifacts")}
	}
	if len(stats.ChangedFiles) == 0 {
		return []string{" " + theme.HelpStyle.Render("No changed files")}
	}
	limit := 2
	lines := make([]string, 0, limit)
	for i, file := range stats.ChangedFiles {
		if i >= limit {
			break
		}
		lines = append(lines, " "+theme.TreeNodeNormal.Render(file))
	}
	if extra := len(stats.ChangedFiles) - limit; extra > 0 {
		lines = append(lines, " "+theme.HelpStyle.Render(fmt.Sprintf("+%d more", extra)))
	}
	return lines
}

func renderSources(stats projectStats) string {
	if stats.Err != "" {
		return " " + theme.HelpStyle.Render("Git unavailable") + "  " + theme.TreeNodeNormal.Render("tmux")
	}
	return " " + theme.TreeNodeNormal.Render("Git") + "  " + theme.TreeNodeNormal.Render("tmux")
}

func renderAction(a action, selected bool) string {
	if selected {
		return fmt.Sprintf(" %s\n   %s",
			theme.TreeNodeSelected.Render("▸ "+a.title),
			theme.HelpStyle.Render(a.description),
		)
	}
	return fmt.Sprintf(" %s\n   %s",
		theme.TreeNodeNormal.Render("  "+a.title),
		theme.HelpStyle.Render(a.description),
	)
}

func renderPane(pane tmux.Pane) string {
	return fmt.Sprintf(" %s %s",
		theme.TreeMeta.Render(fmt.Sprintf("%s:%d.%d", pane.SessionName, pane.WindowIndex, pane.PaneIndex)),
		theme.TreeNodeNormal.Render(pane.Command),
	)
}

func formatInt(value int) string {
	raw := fmt.Sprintf("%d", value)
	if len(raw) <= 3 {
		return raw
	}
	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return strings.Join(parts, ",")
}
