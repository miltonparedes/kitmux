package windows

import (
	"fmt"
	"strings"

	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(" " + theme.TreeNodeSelected.Render(m.sessionName) + "\n")

	viewHeight := m.height - 2 // header + footer
	if viewHeight < 1 {
		viewHeight = 1
	}

	if len(m.windows) == 0 {
		b.WriteString(theme.HelpStyle.Render(" no windows"))
		for i := 1; i < viewHeight; i++ {
			b.WriteString("\n")
		}
	} else {
		start := m.scroll
		end := start + viewHeight
		if end > len(m.windows) {
			end = len(m.windows)
		}

		for i := start; i < end; i++ {
			w := m.windows[i]
			selected := i == m.cursor
			name := fmt.Sprintf("%d:%s", w.Index, w.Name)

			active := ""
			if w.Active {
				active = " " + theme.AttachedBadge.Render("●")
			}

			if selected {
				fmt.Fprintf(&b, " %s%s", theme.TreeNodeSelected.Render(name), active)
			} else {
				fmt.Fprintf(&b, " %s%s", theme.TreeNodeNormal.Render(name), active)
			}
			if i < end-1 {
				b.WriteString("\n")
			}
		}

		rendered := end - start
		for rendered < viewHeight {
			b.WriteString("\n")
			rendered++
		}
	}

	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ switch  h back  q quit"))

	return b.String()
}
