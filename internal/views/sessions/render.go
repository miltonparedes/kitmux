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

	for i := start; i < end; i++ {
		node := m.visible[i]
		selected := i == m.cursor
		b.WriteString(renderNode(node, selected, m.width))
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
		return theme.AttachedBadge.Render(" kill? y/n")
	}
	if m.renaming {
		return " " + m.renameInput.View()
	}
	if m.searching {
		return " " + m.searchInput.View()
	}
	return theme.HelpStyle.Render(" ⏎ switch  ␣ fold  J/K group  / search  n open  q quit")
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
		proj := m.picker.filtered[i]
		selected := i == m.picker.cursor

		score := theme.TreeMeta.Render(fmt.Sprintf("%3.0f", proj.Score))
		if selected {
			fmt.Fprintf(&b, " %s %s %s",
				theme.PaletteItemSelected.Render("▸"),
				score,
				theme.TreeNodeSelected.Render(proj.Short))
		} else {
			fmt.Fprintf(&b, "   %s %s",
				score,
				theme.TreeNodeNormal.Render(proj.Short))
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

	meta := fmt.Sprintf("%dw", node.Windows)
	if node.Attached {
		meta += " " + theme.AttachedBadge.Render("●")
	}
	metaStr := theme.TreeMeta.Render(meta)

	var left string
	if node.Depth > 0 {
		connector := theme.TreeConnector.Render("┊ ")
		if selected {
			left = fmt.Sprintf(" %s%s  %s", connector, theme.TreeNodeSelected.Render(node.Name), metaStr)
		} else {
			left = fmt.Sprintf(" %s%s  %s", connector, theme.TreeNodeNormal.Render(node.Name), metaStr)
		}
	} else {
		if selected {
			left = fmt.Sprintf(" %s  %s", theme.TreeNodeSelected.Render(node.Name), metaStr)
		} else {
			left = fmt.Sprintf(" %s  %s", theme.TreeNodeNormal.Render(node.Name), metaStr)
		}
	}

	// Right-justify diff stats
	var right string
	if node.Added > 0 || node.Deleted > 0 {
		var parts []string
		if node.Added > 0 {
			parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", node.Added)))
		}
		if node.Deleted > 0 {
			parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", node.Deleted)))
		}
		right = strings.Join(parts, " ")
	}

	if right != "" && width > 0 {
		leftW := lipgloss.Width(left)
		rightW := lipgloss.Width(right)
		gap := width - 1 - leftW - rightW
		if gap < 2 {
			return left // no room, skip stats
		}
		return left + strings.Repeat(" ", gap) + right
	}
	return left
}
