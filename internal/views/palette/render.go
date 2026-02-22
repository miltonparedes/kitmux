package palette

import (
	"fmt"
	"strings"

	"github.com/miltonparedes/kitmux/internal/theme"
)

func (m Model) View() string {
	var b strings.Builder

	// Input
	b.WriteString(" " + m.input.View())
	b.WriteString("\n")

	// Separator under input
	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	mainSep := " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
	itemSep := "  " + theme.TreeMeta.Render(strings.Repeat("─", sepW-1))

	b.WriteString(mainSep)
	b.WriteString("\n")

	maxVisible := m.maxVisible()

	start := m.scroll
	end := start + maxVisible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := start; i < end; i++ {
		cmd := m.filtered[i]
		cat := theme.PaletteCategory.Render(cmd.Category)

		if i < 9 {
			b.WriteString(theme.TreeMeta.Render(fmt.Sprintf("%d", i+1)))
		} else {
			b.WriteString(" ")
		}

		if i == m.cursor {
			fmt.Fprintf(&b, " %s %s  %s",
				theme.PaletteItemSelected.Render("▸"),
				theme.PaletteItemSelected.Render(cmd.Title),
				cat)
		} else {
			fmt.Fprintf(&b, "   %s  %s",
				theme.PaletteItem.Render(cmd.Title),
				cat)
		}
		b.WriteString("\n")

		if i < end-1 {
			b.WriteString(itemSep)
			b.WriteString("\n")
		}
	}

	return b.String()
}
