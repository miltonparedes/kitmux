package workspaces

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/miltonparedes/kitmux/internal/theme"
)

// View dispatches to the right sub-view based on the active mode. The
// regular dashboard intentionally avoids encapsulating borders to stay
// visually consistent with the sessions list and the command palette: a
// dim header line, generously spaced rows separated by hairlines, and a
// footer separator above the help/status text.
func (m Model) View() string {
	switch m.mode {
	case modeWorkspaceSearch:
		return m.viewProjectSearch()
	case modeAgentPicker, modeNewBranchAgent:
		return m.viewAgentPicker()
	case modeAgentAttachChoice:
		return m.viewAgentAttachChoice()
	case modeAttachBranchPicker:
		return m.viewAttachBranchPicker()
	default:
		return m.viewColumns()
	}
}

func (m Model) viewColumns() string {
	innerW := m.innerWidth()
	leftW := m.leftWidth()
	rightW := m.rightWidth()
	avail := m.contentHeight()

	leftCol := m.renderLeftColumn(leftW, avail)
	rightCol := m.renderRightColumn(rightW, avail)

	gutter := renderGutter(avail)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, gutter, rightCol)

	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))

	return body + "\n" + footerSep + "\n" + m.footer()
}

// renderGutter draws a vertical thin connector between the two columns.
func renderGutter(height int) string {
	if height < 1 {
		height = 1
	}
	line := theme.TreeConnector.Render("│")
	rows := make([]string, height)
	for i := range rows {
		rows[i] = " " + line + " "
	}
	return strings.Join(rows, "\n")
}

// columnHeader returns the dim italic header used at the top of a column,
// matching the "TreeGroupHeader" style used elsewhere.
func columnHeader(title string, active bool) string {
	if title == "" {
		title = " "
	}
	if active {
		return theme.TreeNodeSelected.Render(title)
	}
	return theme.TreeGroupHeader.Render(title)
}

// padBlock pads a string-builder backed slice up to `target` total lines.
func padTo(b *strings.Builder, used, target int) {
	for used < target {
		b.WriteString("\n")
		used++
	}
}

// rowSep returns a hairline separator the width of a column. We use the
// same dim "┄" feel as the sessions list (TreeMeta over '─').
func rowSep(width int) string {
	if width < 2 {
		width = 2
	}
	return theme.TreeMeta.Render(strings.Repeat("─", width-1))
}

func (m Model) renderLeftColumn(width, avail int) string {
	if m.mode == modeFiltering && m.focus == colWorkspaces {
		return m.renderFilteredLeft(width, avail)
	}

	var b strings.Builder
	used := 0

	b.WriteString(columnHeader("Workspaces", m.focus == colWorkspaces))
	b.WriteString("\n")
	used++
	b.WriteString(rowSep(width))
	b.WriteString("\n")
	used++

	if len(m.workspaces) == 0 {
		b.WriteString(" " + theme.HelpStyle.Render("No workspaces"))
		b.WriteString("\n")
		used++
		b.WriteString(" " + theme.HelpStyle.Render("press n to add"))
		b.WriteString("\n")
		used++
		padTo(&b, used, avail)
		return b.String()
	}

	// Each row uses 2 lines (item + sep), the last row only 1.
	maxVisible := (avail - used + 1) / 2
	if maxVisible < 1 {
		maxVisible = 1
	}

	end := m.wsScroll + maxVisible
	if end > len(m.workspaces) {
		end = len(m.workspaces)
	}

	for i := m.wsScroll; i < end; i++ {
		p := m.workspaces[i]
		selected := i == m.wsCursor && m.focus == colWorkspaces
		b.WriteString(m.renderWorkspaceLine(p, selected, width))
		b.WriteString("\n")
		used++
		if i < end-1 && used < avail-1 {
			b.WriteString(" " + rowSep(width-1))
			b.WriteString("\n")
			used++
		}
	}

	padTo(&b, used, avail)
	return b.String()
}

func (m Model) renderFilteredLeft(width, avail int) string {
	var b strings.Builder
	used := 0

	b.WriteString(" " + m.filter.View())
	b.WriteString("\n")
	used++
	b.WriteString(rowSep(width))
	b.WriteString("\n")
	used++

	idxs := filteredWorkspaceIndices(m.workspaces, m.filter.Value())
	if len(idxs) == 0 {
		b.WriteString(" " + theme.HelpStyle.Render("no matches"))
		b.WriteString("\n")
		used++
		padTo(&b, used, avail)
		return b.String()
	}

	maxVisible := (avail - used + 1) / 2
	if maxVisible < 1 {
		maxVisible = 1
	}

	for k, idx := range idxs {
		if k >= maxVisible {
			break
		}
		p := m.workspaces[idx]
		selected := idx == m.wsCursor
		b.WriteString(m.renderWorkspaceLine(p, selected, width))
		b.WriteString("\n")
		used++
		if k < len(idxs)-1 && k < maxVisible-1 && used < avail-1 {
			b.WriteString(" " + rowSep(width-1))
			b.WriteString("\n")
			used++
		}
	}

	padTo(&b, used, avail)
	return b.String()
}

func (m Model) renderWorkspaceLine(p workspaceEntry, selected bool, width int) string {
	var name string
	if selected {
		name = " " + theme.TreeNodeSelected.Render("▸ "+p.Name)
	} else {
		name = "   " + theme.TreeNodeNormal.Render(p.Name)
	}
	if p.Active {
		name += " " + theme.AttachedBadge.Render("●")
	}
	summary := workspaceDiffSummary(p)
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

func workspaceDiffSummary(p workspaceEntry) string {
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
		return m.renderNewBranchOverlay(width, avail)
	}

	var b strings.Builder
	used := 0

	title := " "
	if p := m.selectedWorkspace(); p != nil {
		title = p.Name
	}
	b.WriteString(columnHeader(title, m.focus == colDetail))
	b.WriteString("\n")
	used++
	b.WriteString(rowSep(width))
	b.WriteString("\n")
	used++

	if len(m.workspaces) == 0 || m.detailItems == 0 {
		b.WriteString(" " + theme.HelpStyle.Render("Select a workspace"))
		b.WriteString("\n")
		used++
		padTo(&b, used, avail)
		return b.String()
	}

	// Branches section
	if len(m.branches) > 0 {
		b.WriteString(" " + theme.TreeGroupHeader.Render("Branches"))
		b.WriteString("\n")
		used++

		branchStart := m.detScroll
		if branchStart > len(m.branches) {
			branchStart = len(m.branches)
		}
		// Reserve at least 4 lines for the agents section if any.
		reserve := 0
		if len(m.agentEntries) > 0 {
			reserve = 3
		}
		branchBudget := avail - used - reserve
		if branchBudget < 1 {
			branchBudget = 1
		}
		shown := 0
		for i := branchStart; i < len(m.branches); i++ {
			if shown*2+1 > branchBudget {
				break
			}
			br := m.branches[i]
			selected := i == m.detCursor && m.focus == colDetail
			b.WriteString(m.renderBranch(br, selected, width))
			b.WriteString("\n")
			used++
			shown++
			if i < len(m.branches)-1 && shown*2 < branchBudget {
				b.WriteString(" " + rowSep(width-1))
				b.WriteString("\n")
				used++
			}
		}
	} else {
		b.WriteString(" " + theme.HelpStyle.Render("no branches"))
		b.WriteString("\n")
		used++
	}

	// Agents section
	if len(m.agentEntries) > 0 && used < avail-1 {
		b.WriteString("\n")
		used++
		b.WriteString(" " + theme.TreeGroupHeader.Render("Agents"))
		b.WriteString("\n")
		used++
		for i, ae := range m.agentEntries {
			if used >= avail {
				break
			}
			detIdx := len(m.branches) + i
			selected := detIdx == m.detCursor && m.focus == colDetail
			b.WriteString(m.renderAgentEntry(ae, selected))
			b.WriteString("\n")
			used++
		}
	}

	padTo(&b, used, avail)
	return b.String()
}

func (m Model) renderNewBranchOverlay(width, avail int) string {
	var b strings.Builder
	used := 0
	b.WriteString(columnHeader("New worktree", true))
	b.WriteString("\n")
	used++
	b.WriteString(rowSep(width))
	b.WriteString("\n")
	used++
	b.WriteString(" " + m.newBranch.View())
	b.WriteString("\n")
	used++
	padTo(&b, used, avail)
	return b.String()
}

func (m Model) renderBranch(br branchEntry, selected bool, width int) string {
	var left string
	if br.IsSession {
		meta := fmt.Sprintf("%dw", br.Windows)
		if br.Attached {
			meta += " " + theme.AttachedBadge.Render("●")
		}
		if br.Ahead > 0 {
			meta += " " + theme.TreeMeta.Render(fmt.Sprintf("↑%d", br.Ahead))
		}
		if br.Behind > 0 {
			meta += " " + theme.TreeMeta.Render(fmt.Sprintf("↓%d", br.Behind))
		}
		metaStr := theme.TreeMeta.Render(meta)
		if selected {
			left = fmt.Sprintf(" %s %s  %s", theme.PaletteItemSelected.Render("▸"),
				theme.TreeNodeSelected.Render(br.Name), metaStr)
		} else {
			left = fmt.Sprintf("   %s  %s", theme.TreeNodeNormal.Render(br.Name), metaStr)
		}
	} else {
		if selected {
			left = fmt.Sprintf(" %s %s", theme.PaletteItemSelected.Render("▸"),
				theme.TreeNodeSelected.Render(br.Name))
		} else {
			left = fmt.Sprintf("   %s", theme.HelpStyle.Render(br.Name))
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
	if ae.IsLauncher {
		label := "+ launch agent..."
		if selected {
			return " " + theme.PaletteItemSelected.Render("▸") + " " +
				theme.TreeNodeSelected.Render(label)
		}
		return "   " + theme.HelpStyle.Render(label)
	}
	label := ae.Name
	meta := theme.TreeMeta.Render(fmt.Sprintf("(%s:%d.%d)", ae.SessionName, ae.WindowIndex, ae.PaneIndex))
	if selected {
		return fmt.Sprintf(" %s %s  %s", theme.PaletteItemSelected.Render("▸"),
			theme.TreeNodeSelected.Render(label), meta)
	}
	return fmt.Sprintf("   %s  %s", theme.TreeNodeNormal.Render(label), meta)
}

func (m Model) footer() string {
	if m.toast != "" {
		return m.renderToast()
	}
	switch m.mode {
	case modeConfirm:
		switch m.confirmAction {
		case confirmActionRemoveWorktree:
			return theme.AttachedBadge.Render(fmt.Sprintf(" remove worktree '%s'? y/n", m.confirmBranch))
		default:
			return theme.AttachedBadge.Render(fmt.Sprintf(" remove '%s'? y/n", m.confirmName))
		}
	case modeFiltering:
		return theme.HelpStyle.Render(" type to filter  ⏎ select  esc cancel")
	case modeNewBranch:
		return theme.HelpStyle.Render(" ⏎ create  tab + agent  esc back")
	case modeNewBranchAgent:
		return theme.HelpStyle.Render(" ⏎ create + launch  tab mode  esc back")
	case modeAgentAttachChoice:
		return theme.HelpStyle.Render(" ⏎ choose  j/k nav  esc back")
	case modeAttachBranchPicker:
		return theme.HelpStyle.Render(" ⏎ select branch  j/k nav  esc back")
	}
	if m.focus == colDetail {
		return theme.HelpStyle.Render(" h back  j/k nav  ⏎ open  c new worktree  d remove  a agent  A split  / filter")
	}
	return theme.HelpStyle.Render(" l/⏎ open  j/k nav  n add  c new worktree  f find  d remove  a agent  / filter  r refresh  q quit")
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

	innerW := m.innerWidth()

	b.WriteString(" " + m.zoxide.input.View())
	b.WriteString("\n")
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))
	itemSep := " " + theme.TreeMeta.Render(strings.Repeat("─", innerW))

	b.WriteString(mainSep)
	b.WriteString("\n")

	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}

	used := 0
	if len(m.zoxide.filtered) == 0 {
		if len(m.zoxide.all) == 0 {
			b.WriteString(theme.HelpStyle.Render(" Loading..."))
		} else {
			b.WriteString(theme.HelpStyle.Render(" No matches"))
		}
		b.WriteString("\n")
		used++
	}

	maxVisible := (avail + 1) / 2
	if maxVisible < 1 {
		maxVisible = 1
	}
	end := m.zoxide.scroll + maxVisible
	if end > len(m.zoxide.filtered) {
		end = len(m.zoxide.filtered)
	}

	for i := m.zoxide.scroll; i < end; i++ {
		e := m.zoxide.filtered[i]
		sel := i == m.zoxide.cursor
		if sel {
			b.WriteString(" " + theme.PaletteItemSelected.Render("▸") + " " +
				theme.TreeNodeSelected.Render(e.Short))
		} else {
			b.WriteString("   " + theme.TreeNodeNormal.Render(e.Short))
		}
		b.WriteString("\n")
		used++
		if i < end-1 && used < avail-1 {
			b.WriteString(itemSep)
			b.WriteString("\n")
			used++
		}
	}

	padTo(&b, used, avail)

	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ select  esc back"))
	return b.String()
}

func (m Model) viewAgentPicker() string {
	var b strings.Builder
	innerW := m.innerWidth()

	title := "Launch Agent"
	switch m.mode {
	case modeNewBranchAgent:
		title = "New worktree: pick agent (" + m.newBranch.Value() + ")"
	case modeAgentPicker:
		if m.attachBranch.Name != "" {
			if m.agentPickerTarget == agentTargetSplit {
				title = "Split agent into " + m.attachBranch.Name
			} else {
				title = "Attach agent to " + m.attachBranch.Name
			}
		}
	}
	b.WriteString(" " + theme.TreeGroupHeader.Render(title))
	b.WriteString("\n")
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))
	itemSep := " " + theme.TreeMeta.Render(strings.Repeat("─", innerW))
	b.WriteString(mainSep)
	b.WriteString("\n")

	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}

	maxVisible := (avail + 1) / 2
	if maxVisible < 1 {
		maxVisible = 1
	}
	used := 0
	for i, a := range m.agentPicker.agents {
		if i >= maxVisible {
			break
		}
		selected := i == m.agentPicker.cursor
		modeName := a.Modes[m.agentPicker.modeIndex[i]].Name
		if selected {
			name := theme.TreeNodeSelected.Render(a.Name)
			mode := theme.AgentModeSelected.Render(modeName)
			fmt.Fprintf(&b, " %s %s  %s",
				theme.PaletteItemSelected.Render("▸"), name, mode)
		} else {
			name := theme.TreeNodeNormal.Render(a.Name)
			mode := theme.AgentMode.Render(modeName)
			fmt.Fprintf(&b, "   %s  %s", name, mode)
		}
		b.WriteString("\n")
		used++
		if i < len(m.agentPicker.agents)-1 && i < maxVisible-1 && used < avail-1 {
			b.WriteString(itemSep)
			b.WriteString("\n")
			used++
		}
	}

	padTo(&b, used, avail)

	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ launch  tab mode  esc back"))
	return b.String()
}

// viewAgentAttachChoice renders the "In existing branch / In new worktree"
// modal shown when `a` is pressed without a branch context.
func (m Model) viewAgentAttachChoice() string {
	var b strings.Builder
	innerW := m.innerWidth()

	b.WriteString(" " + theme.TreeGroupHeader.Render("Where should the agent run?"))
	b.WriteString("\n")
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))
	itemSep := " " + theme.TreeMeta.Render(strings.Repeat("─", innerW))
	b.WriteString(mainSep)
	b.WriteString("\n")

	items := []string{
		"In existing branch...",
		"In new worktree...",
	}
	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}
	used := 0
	for i, it := range items {
		selected := i == m.attachChoiceCursor
		if selected {
			b.WriteString(" " + theme.PaletteItemSelected.Render("▸") + " " +
				theme.TreeNodeSelected.Render(it))
		} else {
			b.WriteString("   " + theme.TreeNodeNormal.Render(it))
		}
		b.WriteString("\n")
		used++
		if i < len(items)-1 && used < avail-1 {
			b.WriteString(itemSep)
			b.WriteString("\n")
			used++
		}
	}
	padTo(&b, used, avail)

	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ choose  j/k nav  esc back"))
	return b.String()
}

// viewAttachBranchPicker renders a simple list of branches for the current
// workspace so the user can pick where to attach the agent.
func (m Model) viewAttachBranchPicker() string {
	var b strings.Builder
	innerW := m.innerWidth()

	b.WriteString(" " + theme.TreeGroupHeader.Render("Pick branch to attach agent"))
	b.WriteString("\n")
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))
	itemSep := " " + theme.TreeMeta.Render(strings.Repeat("─", innerW))
	b.WriteString(mainSep)
	b.WriteString("\n")

	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}
	maxVisible := (avail + 1) / 2
	if maxVisible < 1 {
		maxVisible = 1
	}

	used := 0
	for i, br := range m.branches {
		if i >= maxVisible {
			break
		}
		label := br.Name
		if br.IsSession {
			label += "  " + theme.TreeMeta.Render("(session)")
		}
		selected := i == m.attachBranchCursor
		if selected {
			b.WriteString(" " + theme.PaletteItemSelected.Render("▸") + " " +
				theme.TreeNodeSelected.Render(label))
		} else {
			b.WriteString("   " + theme.TreeNodeNormal.Render(label))
		}
		b.WriteString("\n")
		used++
		if i < len(m.branches)-1 && i < maxVisible-1 && used < avail-1 {
			b.WriteString(itemSep)
			b.WriteString("\n")
			used++
		}
	}
	padTo(&b, used, avail)

	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", innerW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ select branch  j/k nav  esc back"))
	return b.String()
}

// --- Layout ---

// innerWidth is the usable width inside the 1-char left margin we keep
// throughout (matching sessions/palette).
func (m Model) innerWidth() int {
	w := m.width - 2
	if w < 4 {
		w = 4
	}
	return w
}

// leftWidth is the workspaces column. We aim for ~32% of usable width with
// reasonable bounds.
func (m Model) leftWidth() int {
	inner := m.innerWidth()
	w := inner * 32 / 100
	if w < 18 {
		w = 18
	}
	if w > inner-20 {
		w = inner - 20
	}
	if w < 1 {
		w = 1
	}
	return w
}

// rightWidth covers the remaining space minus the 3-char gutter
// (" │ " between columns).
func (m Model) rightWidth() int {
	w := m.innerWidth() - m.leftWidth() - 3
	if w < 1 {
		w = 1
	}
	return w
}
