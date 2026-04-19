package windows

import (
	"fmt"
	"strings"

	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(" " + theme.TreeNodeSelected.Render(m.sessionName) + "\n")

	viewHeight := m.height - 2 // header + footer
	if viewHeight < 1 {
		viewHeight = 1
	}

	if len(m.windows) == 0 {
		writeEmptyWindowView(&b, viewHeight)
	} else {
		m.writeWindowRows(&b, viewHeight)
	}

	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ switch  h back  q quit"))
	return b.String()
}

func writeEmptyWindowView(b *strings.Builder, viewHeight int) {
	b.WriteString(theme.HelpStyle.Render(" no windows"))
	for i := 1; i < viewHeight; i++ {
		b.WriteString("\n")
	}
}

func (m Model) writeWindowRows(b *strings.Builder, viewHeight int) {
	start := m.scroll
	end := start + viewHeight
	if end > len(m.windows) {
		end = len(m.windows)
	}
	for i := start; i < end; i++ {
		m.writeWindowRow(b, i)
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	for rendered := end - start; rendered < viewHeight; rendered++ {
		b.WriteString("\n")
	}
}

func (m Model) writeWindowRow(b *strings.Builder, i int) {
	w := m.windows[i]
	selected := i == m.cursor
	name := fmt.Sprintf("%d:%s", w.Index, w.Name)

	active := ""
	if w.Active {
		active = " " + theme.AttachedBadge.Render("●")
	}

	if i < 9 {
		b.WriteString(theme.TreeMeta.Render(fmt.Sprintf("%d", i+1)))
	} else {
		b.WriteString(" ")
	}

	if selected {
		fmt.Fprintf(b, " %s%s", theme.TreeNodeSelected.Render(name), active)
	} else {
		fmt.Fprintf(b, " %s%s", theme.TreeNodeNormal.Render(name), active)
	}
}
