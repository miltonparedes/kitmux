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

	// Each item = 2 lines (item + separator), last item = 1 line
	// Available lines = height - 3 (input + main sep + trailing)
	avail := m.height - 3
	if avail < 1 {
		avail = 1
	}
	maxItems := (avail + 1) / 2 // N items take 2N-1 lines
	if len(m.filtered) < maxItems {
		maxItems = len(m.filtered)
	}

	for i := 0; i < maxItems; i++ {
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

		if i < maxItems-1 {
			b.WriteString(itemSep)
			b.WriteString("\n")
		}
	}

	return b.String()
}
