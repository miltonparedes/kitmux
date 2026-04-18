package workspaces

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	switch m.mode {
	case modeProjectSearch:
		return m.viewProjectSearch()
	case modeAgentPicker:
		return m.viewAgentPicker()
	default:
		return m.viewColumns()
	}
}

func (m Model) viewColumns() string {
	leftW := m.leftWidth()
	rightW := m.rightWidth()
	avail := m.contentHeight()

	leftContent := m.renderLeftColumn(leftW, avail)
	rightContent := m.renderRightColumn(rightW, avail)

	leftTitle := "Projects"
	rightTitle := ""
	if p := m.selectedProject(); p != nil {
		rightTitle = p.Name
	}

	var leftPanel, rightPanel lipgloss.Style
	if m.focus == colProjects {
		leftPanel = theme.PanelActive.Width(leftW)
		rightPanel = theme.PanelInactive.Width(rightW)
	} else {
		leftPanel = theme.PanelInactive.Width(leftW)
		rightPanel = theme.PanelActive.Width(rightW)
	}

	leftBox := leftPanel.Render(leftContent)
	rightBox := rightPanel.Render(rightContent)

	// Inject titles into top border
	leftBox = injectTitle(leftBox, leftTitle, m.focus == colProjects)
	rightBox = injectTitle(rightBox, rightTitle, m.focus == colDetail)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	footer := m.footer()

	return columns + "\n" + footer
}

func injectTitle(box, title string, active bool) string {
	if title == "" {
		return box
	}
	var styled string
	if active {
		styled = theme.PanelTitle.Render(title)
	} else {
		styled = theme.PanelTitleInactive.Render(title)
	}
	lines := strings.SplitN(box, "\n", 2)
	if len(lines) < 2 {
		return box
	}
	topLine := lines[0]
	titleW := lipgloss.Width(styled)
	topW := lipgloss.Width(topLine)
	if titleW+4 > topW {
		return box
	}
	runes := []rune(topLine)
	insertPos := 3
	if insertPos+titleW > len(runes) {
		return box
	}
	newTop := string(runes[:insertPos]) + styled + string(runes[insertPos+titleW:])
	return newTop + "\n" + lines[1]
}

func (m Model) renderLeftColumn(width, avail int) string {
	if m.mode == modeFiltering && m.focus == colProjects {
		return m.renderFilteredLeft(width, avail)
	}

	var b strings.Builder

	if len(m.projects) == 0 {
		b.WriteString(theme.HelpStyle.Render("No workspaces"))
		b.WriteString("\n")
		b.WriteString(theme.HelpStyle.Render("n to add"))
		for i := 2; i < avail; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	end := m.projScroll + avail
	if end > len(m.projects) {
		end = len(m.projects)
	}

	linesUsed := 0
	for i := m.projScroll; i < end; i++ {
		p := m.projects[i]
		selected := i == m.projCursor && m.focus == colProjects
		b.WriteString(m.renderProjectLine(p, selected, width))
		b.WriteString("\n")
		linesUsed++
	}

	for linesUsed < avail {
		b.WriteString("\n")
		linesUsed++
	}

	return b.String()
}

func (m Model) renderFilteredLeft(width, avail int) string {
	var b strings.Builder
	b.WriteString(m.filter.View())
	b.WriteString("\n")
	avail--

	idxs := filteredProjectIndices(m.projects, m.filter.Value())
	linesUsed := 0
	for _, idx := range idxs {
		if linesUsed >= avail {
			break
		}
		p := m.projects[idx]
		selected := idx == m.projCursor
		b.WriteString(m.renderProjectLine(p, selected, width))
		b.WriteString("\n")
		linesUsed++
	}
	if len(idxs) == 0 {
		b.WriteString(theme.HelpStyle.Render("  no matches"))
		b.WriteString("\n")
		linesUsed++
	}
	for linesUsed < avail {
		b.WriteString("\n")
		linesUsed++
	}
	return b.String()
}

func (m Model) renderProjectLine(p projectEntry, selected bool, width int) string {
	name := p.Name
	if selected {
		name = theme.TreeNodeSelected.Render("▸ " + name)
	} else {
		name = "  " + theme.TreeNodeNormal.Render(name)
	}
	if p.Active {
		name += " " + theme.AttachedBadge.Render("●")
	}
	summary := projectDiffSummary(p)
	if summary == "" || width <= 0 {
		return name
	}
	leftW := lipgloss.Width(name)
	rightW := lipgloss.Width(summary)
	gap := width - leftW - rightW - 1
	if gap < 1 {
		return name
	}
	return name + strings.Repeat(" ", gap) + summary
}

func projectDiffSummary(p projectEntry) string {
	var parts []string
	if p.Added > 0 {
		parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", p.Added)))
	}
	if p.Deleted > 0 {
		parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", p.Deleted)))
	}
	return strings.Join(parts, " ")
}

func (m Model) renderRightColumn(width, avail int) string {
	if m.mode == modeNewBranch {
		return m.renderNewBranchOverlay(avail)
	}

	var b strings.Builder

	if len(m.projects) == 0 || m.detailItems == 0 {
		b.WriteString(theme.HelpStyle.Render("Select a project"))
		for i := 1; i < avail; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	b.WriteString(theme.TreeGroupHeader.Render("Branches"))
	b.WriteString("\n")
	avail--
	linesUsed := 0

	branchStart := m.detScroll
	if branchStart > len(m.branches) {
		branchStart = len(m.branches)
	}
	for i := branchStart; i < len(m.branches) && linesUsed < avail-2; i++ {
		br := m.branches[i]
		selected := i == m.detCursor && m.focus == colDetail
		b.WriteString(m.renderBranch(br, selected, width))
		b.WriteString("\n")
		linesUsed++
	}

	if len(m.branches) == 0 {
		b.WriteString(theme.HelpStyle.Render("  no branches"))
		b.WriteString("\n")
		linesUsed++
	}

	if linesUsed < avail-1 {
		b.WriteString("\n")
		linesUsed++
		b.WriteString(theme.TreeGroupHeader.Render("Agents"))
		b.WriteString("\n")
		linesUsed++
	}

	for i, ae := range m.agentEntries {
		if linesUsed >= avail {
			break
		}
		detIdx := len(m.branches) + i
		selected := detIdx == m.detCursor && m.focus == colDetail
		b.WriteString(m.renderAgentEntry(ae, selected))
		b.WriteString("\n")
		linesUsed++
	}

	for linesUsed < avail {
		b.WriteString("\n")
		linesUsed++
	}

	return b.String()
}

func (m Model) renderNewBranchOverlay(avail int) string {
	var b strings.Builder
	b.WriteString(theme.TreeGroupHeader.Render("New branch"))
	b.WriteString("\n")
	b.WriteString(m.newBranch.View())
	for i := 2; i < avail; i++ {
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderBranch(br branchEntry, selected bool, width int) string {
	connector := theme.TreeConnector.Render("┊ ")

	var left string
	if br.IsSession {
		meta := fmt.Sprintf("%dw", br.Windows)
		if br.Attached {
			meta += " " + theme.AttachedBadge.Render("*")
		}
		if br.Ahead > 0 {
			meta += " " + theme.TreeMeta.Render(fmt.Sprintf("↑%d", br.Ahead))
		}
		if br.Behind > 0 {
			meta += " " + theme.TreeMeta.Render(fmt.Sprintf("↓%d", br.Behind))
		}
		metaStr := theme.TreeMeta.Render(meta)
		if selected {
			left = fmt.Sprintf("%s%s  %s", connector, theme.TreeNodeSelected.Render(br.Name), metaStr)
		} else {
			left = fmt.Sprintf("%s%s  %s", connector, theme.TreeNodeNormal.Render(br.Name), metaStr)
		}
	} else {
		if selected {
			left = fmt.Sprintf("%s%s", connector, theme.TreeNodeSelected.Render(br.Name))
		} else {
			left = fmt.Sprintf("%s%s", connector, theme.HelpStyle.Render(br.Name))
		}
	}

	right := branchDiffStats(br)
	if right != "" && width > 0 {
		leftW := lipgloss.Width(left)
		rightW := lipgloss.Width(right)
		gap := width - leftW - rightW - 1
		if gap >= 2 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return left
}

func branchDiffStats(br branchEntry) string {
	var parts []string
	if br.DiffAdded > 0 {
		parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", br.DiffAdded)))
	}
	if br.DiffDel > 0 {
		parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", br.DiffDel)))
	}
	return strings.Join(parts, " ")
}

func (m Model) renderAgentEntry(ae agentEntry, selected bool) string {
	connector := theme.TreeConnector.Render("┊ ")
	if ae.IsLauncher {
		label := "+ launch agent..."
		if selected {
			return connector + theme.TreeNodeSelected.Render(label)
		}
		return connector + theme.HelpStyle.Render(label)
	}
	label := ae.Name
	meta := theme.TreeMeta.Render(fmt.Sprintf("(%s:%d.%d)", ae.SessionName, ae.WindowIndex, ae.PaneIndex))
	if selected {
		return fmt.Sprintf("%s%s  %s", connector, theme.TreeNodeSelected.Render(label), meta)
	}
	return fmt.Sprintf("%s%s  %s", connector, theme.TreeNodeNormal.Render(label), meta)
}

func (m Model) footer() string {
	if m.toast != "" {
		return m.renderToast()
	}
	switch m.mode {
	case modeConfirm:
		return theme.AttachedBadge.Render(fmt.Sprintf(" remove '%s'? y/n", m.confirmName))
	case modeFiltering:
		return theme.HelpStyle.Render(" type to filter  enter select  esc cancel")
	case modeNewBranch:
		return theme.HelpStyle.Render(" enter create  esc back")
	}
	if m.focus == colDetail {
		return theme.HelpStyle.Render(" h back  j/k nav  enter open  c new branch  a agent  / filter")
	}
	return theme.HelpStyle.Render(" l/enter open  j/k nav  n add  f find  d remove  a agent  / filter  r refresh  q quit")
}

func (m Model) renderToast() string {
	var style lipgloss.Style
	switch m.toastLvl {
	case toastError:
		style = theme.DiffRemoved
	case toastWarn:
		style = theme.DirtyBadge
	default:
		style = theme.HelpStyle
	}
	return style.Render(" " + m.toast)
}

func (m Model) viewProjectSearch() string {
	var b strings.Builder

	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))

	b.WriteString(" " + m.zoxide.input.View())
	b.WriteString("\n")
	b.WriteString(mainSep)
	b.WriteString("\n")

	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}

	if len(m.zoxide.filtered) == 0 {
		if len(m.zoxide.all) == 0 {
			b.WriteString(theme.HelpStyle.Render(" Loading..."))
		} else {
			b.WriteString(theme.HelpStyle.Render(" No matches"))
		}
		b.WriteString("\n")
	}

	end := m.zoxide.scroll + avail
	if end > len(m.zoxide.filtered) {
		end = len(m.zoxide.filtered)
	}

	linesUsed := 0
	for i := m.zoxide.scroll; i < end; i++ {
		e := m.zoxide.filtered[i]
		sel := i == m.zoxide.cursor
		if sel {
			b.WriteString(" " + theme.TreeNodeSelected.Render(e.Short))
		} else {
			b.WriteString(" " + theme.TreeNodeNormal.Render(e.Short))
		}
		b.WriteString("\n")
		linesUsed++
	}

	for linesUsed < avail {
		b.WriteString("\n")
		linesUsed++
	}

	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" enter select  esc back"))
	return b.String()
}

func (m Model) viewAgentPicker() string {
	var b strings.Builder

	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))

	b.WriteString(" " + theme.TreeGroupHeader.Render("Launch Agent"))
	b.WriteString("\n")
	b.WriteString(mainSep)
	b.WriteString("\n")

	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}

	linesUsed := 0
	for i, a := range m.agentPicker.agents {
		if linesUsed >= avail {
			break
		}
		selected := i == m.agentPicker.cursor
		modeName := a.Modes[m.agentPicker.modeIndex[i]].Name
		if selected {
			name := theme.TreeNodeSelected.Render("▸ " + a.Name)
			mode := theme.AgentModeSelected.Render(modeName)
			fmt.Fprintf(&b, " %s  %s", name, mode)
		} else {
			name := theme.TreeNodeNormal.Render("  " + a.Name)
			mode := theme.AgentMode.Render(modeName)
			fmt.Fprintf(&b, " %s  %s", name, mode)
		}
		b.WriteString("\n")
		linesUsed++
	}

	for linesUsed < avail {
		b.WriteString("\n")
		linesUsed++
	}

	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" enter launch  tab mode  esc back"))
	return b.String()
}

// Column widths

func (m Model) leftWidth() int {
	w := m.width*30/100 - 2
	if w < 10 {
		w = 10
	}
	return w
}

func (m Model) rightWidth() int {
	total := m.width - 2
	left := m.leftWidth() + 2
	w := total - left
	if w < 10 {
		w = 10
	}
	return w
}
