package sessions

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	if m.picking {
		return m.viewPicker()
	}

	if len(m.visible) == 0 {
		return theme.HelpStyle.Render(" No sessions found")
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
	if end > len(m.visible) {
		end = len(m.visible)
	}

	// Count sessions before the visible window for correct numbering
	sessionNum := 0
	for i := 0; i < start; i++ {
		if m.visible[i].Kind == KindSession {
			sessionNum++
		}
	}
	for i := start; i < end; i++ {
		node := m.visible[i]
		selected := i == m.cursor
		if node.Kind == KindSession {
			sessionNum++
			if sessionNum <= 9 {
				b.WriteString(theme.TreeMeta.Render(fmt.Sprintf("%d", sessionNum)))
			} else {
				b.WriteString(" ")
			}
		} else {
			b.WriteString(" ")
		}
		b.WriteString(renderNode(node, selected, m.width-1))
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
	if m.confirming {
		name := ""
		if node := m.selected(); node != nil {
			name = node.SessionName
		}
		return theme.AttachedBadge.Render(fmt.Sprintf(" kill '%s'? y/n", name))
	}
	if m.renaming {
		return " " + m.renameInput.View()
	}
	if m.searching {
		return " " + m.searchInput.View()
	}
	return theme.HelpStyle.Render(" ⏎ switch  ␣ fold  J/K group  / search  n open  d kill  r rename  q quit")
}

func (m Model) viewPicker() string {
	var b strings.Builder

	b.WriteString(" " + m.picker.input.View())
	b.WriteString("\n")

	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	itemSep := " " + theme.TreeMeta.Render(strings.Repeat("─", sepW))

	b.WriteString(mainSep)
	b.WriteString("\n")

	if len(m.picker.filtered) == 0 && len(m.picker.all) > 0 {
		b.WriteString(theme.HelpStyle.Render(" No matches"))
		b.WriteString("\n")
	} else if len(m.picker.all) == 0 {
		b.WriteString(theme.HelpStyle.Render(" Loading..."))
		b.WriteString("\n")
	}

	maxVisible := m.pickerMaxVisible()
	start := m.picker.scroll
	end := start + maxVisible
	if end > len(m.picker.filtered) {
		end = len(m.picker.filtered)
	}

	for i := start; i < end; i++ {
		entry := m.picker.filtered[i]
		selected := i == m.picker.cursor

		score := theme.TreeMeta.Render(fmt.Sprintf("%3.0f", entry.Score))
		if selected {
			fmt.Fprintf(&b, " %s %s %s",
				theme.PaletteItemSelected.Render("▸"),
				score,
				theme.TreeNodeSelected.Render(entry.Short))
		} else {
			fmt.Fprintf(&b, "   %s %s",
				score,
				theme.TreeNodeNormal.Render(entry.Short))
		}
		b.WriteString("\n")

		if i < end-1 {
			b.WriteString(itemSep)
			b.WriteString("\n")
		}
	}

	// Pad remaining space
	rendered := end - start
	linesUsed := rendered*2 - 1
	if rendered <= 0 {
		linesUsed = 1 // "Loading..." or "No matches" takes 1 line
	}
	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}
	for linesUsed < avail {
		b.WriteString("\n")
		linesUsed++
	}

	footerSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	b.WriteString(footerSep)
	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ open  esc cancel"))

	return b.String()
}

func renderNode(node *TreeNode, selected bool, width int) string {
	if node.Kind == KindGroupHeader {
		return renderGroupHeader(node, selected)
	}
	left := renderSessionLeft(node, selected)
	right := renderDiffStats(node)
	return joinSessionLine(left, right, width)
}

func renderGroupHeader(node *TreeNode, selected bool) string {
	indicator := "▾"
	if !node.Expanded {
		indicator = "▸"
	}
	name := fmt.Sprintf("%s %s", indicator, node.Name)
	if selected {
		return " " + theme.TreeNodeSelected.Render(name)
	}
	return " " + theme.TreeGroupHeader.Render(name)
}

func renderSessionLeft(node *TreeNode, selected bool) string {
	metaStr := theme.TreeMeta.Render(sessionMeta(node))
	nameStr := styledSessionName(node.Name, selected)
	if node.Depth > 0 {
		connector := theme.TreeConnector.Render("┊ ")
		return fmt.Sprintf(" %s%s  %s", connector, nameStr, metaStr)
	}
	return fmt.Sprintf(" %s  %s", nameStr, metaStr)
}

func sessionMeta(node *TreeNode) string {
	meta := fmt.Sprintf("%dw", node.Windows)
	if node.Attached {
		meta += " " + theme.AttachedBadge.Render("●")
	}
	return meta
}

func styledSessionName(name string, selected bool) string {
	if selected {
		return theme.TreeNodeSelected.Render(name)
	}
	return theme.TreeNodeNormal.Render(name)
}

func renderDiffStats(node *TreeNode) string {
	if node.Added == 0 && node.Deleted == 0 {
		return ""
	}
	var parts []string
	if node.Added > 0 {
		parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", node.Added)))
	}
	if node.Deleted > 0 {
		parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", node.Deleted)))
	}
	return strings.Join(parts, " ")
}

func joinSessionLine(left, right string, width int) string {
	if right == "" || width <= 0 {
		return left
	}
	gap := width - 1 - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}
