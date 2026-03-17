package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/miltonparedes/kitmux/internal/theme"
	"github.com/miltonparedes/kitmux/internal/views/sessions"
)

func (m Model) View() string {
	switch m.mode {
	case modeProjectSearch:
		return m.viewProjectSearch()
	case modeWorktreePicker, modeNewBranch:
		return m.viewWorktreePicker()
	default:
		return m.viewNormal()
	}
}

func (m Model) viewNormal() string {
	var b strings.Builder

	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	itemSep := " " + theme.TreeMeta.Render(strings.Repeat("─", sepW))

	// Header
	if m.mode == modeFiltering {
		b.WriteString(" " + m.filter.View())
	} else {
		b.WriteString(theme.HelpStyle.Render(" Projects"))
	}
	b.WriteString("\n")
	b.WriteString(mainSep)
	b.WriteString("\n")

	if len(m.visible) == 0 {
		if m.mode == modeFiltering {
			b.WriteString(theme.HelpStyle.Render(" No matches"))
		} else {
			b.WriteString(theme.HelpStyle.Render(" No projects"))
		}
		b.WriteString("\n")
		return m.padAndFooter(&b, sepW)
	}

	b.WriteString("\n")

	avail := m.height - 5
	if avail < 1 {
		avail = 1
	}

	start := m.scroll
	end := start
	linesNeeded := 0
	for i := start; i < len(m.visible); i++ {
		extra := 0
		if i > start && m.visible[i].Depth == 0 {
			extra = 1
		}
		if linesNeeded+extra+1 > avail {
			break
		}
		linesNeeded += extra + 1
		end = i + 1
	}

	linesUsed := 0
	for i := start; i < end; i++ {
		if i > start && m.visible[i].Depth == 0 {
			b.WriteString(itemSep)
			b.WriteString("\n")
			linesUsed++
		}

		node := m.visible[i]
		selected := i == m.cursor
		b.WriteString(m.renderNode(node, selected))
		b.WriteString("\n")
		linesUsed++
	}

	for linesUsed < avail {
		b.WriteString("\n")
		linesUsed++
	}

	return m.padFooterOnly(&b, sepW)
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
		return m.padAndFooter(&b, sepW)
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

func (m Model) viewWorktreePicker() string {
	var b strings.Builder

	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))

	// Header
	if m.mode == modeNewBranch {
		b.WriteString(" " + m.newBranch.View())
	} else {
		b.WriteString(" " + theme.TreeGroupHeader.Render(m.wtPicker.project) + theme.HelpStyle.Render(" worktrees"))
	}
	b.WriteString("\n")
	b.WriteString(mainSep)
	b.WriteString("\n")

	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}

	if len(m.wtPicker.entries) == 0 {
		b.WriteString(theme.HelpStyle.Render(" No worktrees"))
		b.WriteString("\n")
		return m.padAndFooter(&b, sepW)
	}

	linesUsed := 0
	for i, e := range m.wtPicker.entries {
		if linesUsed >= avail {
			break
		}
		sel := i == m.wtPicker.cursor

		var label string
		if e.IsMain {
			label = e.Branch + " (main)"
		} else {
			label = e.Branch
		}

		var left string
		switch {
		case sel:
			left = " " + theme.TreeNodeSelected.Render(label)
		case e.HasSess:
			left = " " + theme.TreeNodeNormal.Render(label)
		default:
			left = " " + theme.HelpStyle.Render(label)
		}

		// Badges
		var badges []string
		if e.Attached {
			badges = append(badges, theme.AttachedBadge.Render("*"))
		} else if e.HasSess {
			badges = append(badges, theme.TreeMeta.Render("running"))
		}

		if len(badges) > 0 {
			left += "  " + strings.Join(badges, " ")
		}

		// Diff stats right-aligned
		right := wtDiffStats(e)
		if right != "" && m.width > 0 {
			leftW := lipgloss.Width(left)
			rightW := lipgloss.Width(right)
			gap := m.width - 1 - leftW - rightW
			if gap >= 2 {
				left += strings.Repeat(" ", gap) + right
			}
		}

		b.WriteString(left)
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
	if m.mode == modeNewBranch {
		b.WriteString(theme.HelpStyle.Render(" enter create  esc back"))
	} else {
		b.WriteString(theme.HelpStyle.Render(" enter open  c new branch  esc back"))
	}
	return b.String()
}

func wtDiffStats(e wtEntry) string {
	var parts []string
	if e.DiffAdded > 0 {
		parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", e.DiffAdded)))
	}
	if e.DiffDel > 0 {
		parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", e.DiffDel)))
	}
	return strings.Join(parts, " ")
}

// Shared helpers

func (m Model) padAndFooter(b *strings.Builder, sepW int) string {
	lines := strings.Count(b.String(), "\n")
	for lines < m.height-2 {
		b.WriteString("\n")
		lines++
	}
	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(m.statusLine())
	return b.String()
}

func (m Model) padFooterOnly(b *strings.Builder, sepW int) string {
	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(m.statusLine())
	return b.String()
}

func (m Model) statusLine() string {
	if m.mode == modeConfirm {
		name := ""
		if node := m.selected(); node != nil {
			name = node.Name
		}
		return theme.AttachedBadge.Render(fmt.Sprintf(" delete '%s'? y/n (hide only; no repo changes)", name))
	}
	if m.mode == modeFiltering {
		return theme.HelpStyle.Render(" arrows navigate  enter accept  esc clear")
	}
	return theme.HelpStyle.Render(" enter open  / filter  n add  d delete  r refresh")
}

func (m Model) renderNode(node *sessions.TreeNode, selected bool) string {
	inactive := node.Depth == 0 && len(node.Children) == 0

	if node.Depth == 0 {
		indicator := "▾"
		if !node.Expanded || inactive {
			indicator = "▸"
		}
		name := fmt.Sprintf("%s %s", indicator, node.Name)
		if inactive {
			if selected {
				return " " + theme.TreeNodeSelected.Render(name)
			}
			return " " + theme.HelpStyle.Render(name)
		}
		if selected {
			return " " + theme.TreeNodeSelected.Render(name)
		}
		return " " + theme.TreeGroupHeader.Render(name)
	}

	// Session child
	meta := fmt.Sprintf("%dw", node.Windows)
	if node.Attached {
		meta += " " + theme.AttachedBadge.Render("*")
	}
	metaStr := theme.TreeMeta.Render(meta)

	connector := theme.TreeConnector.Render("| ")
	var left string
	if selected {
		left = fmt.Sprintf(" %s%s  %s", connector, theme.TreeNodeSelected.Render(node.Name), metaStr)
	} else {
		left = fmt.Sprintf(" %s%s  %s", connector, theme.TreeNodeNormal.Render(node.Name), metaStr)
	}

	right := m.diffStatsFor(node)
	if right != "" && m.width > 0 {
		leftW := lipgloss.Width(left)
		rightW := lipgloss.Width(right)
		gap := m.width - 1 - leftW - rightW
		if gap >= 2 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return left
}

func (m Model) diffStatsFor(node *sessions.TreeNode) string {
	if node.Kind != sessions.KindSession || node.SessionName == "" {
		return ""
	}
	st, ok := m.stats[node.SessionName]
	if !ok {
		return ""
	}
	var parts []string
	if st.Added > 0 {
		parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", st.Added)))
	}
	if st.Deleted > 0 {
		parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", st.Deleted)))
	}
	return strings.Join(parts, " ")
}
