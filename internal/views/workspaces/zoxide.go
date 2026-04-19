package workspaces

import "github.com/sahilm/fuzzy"

func (z *zoxidePicker) filter() {
	query := z.input.Value()
	if query == "" {
		z.filtered = z.all
	} else {
		shorts := make([]string, len(z.all))
		for i, e := range z.all {
			shorts[i] = e.Short
		}
		matches := fuzzy.Find(query, shorts)
		filtered := make([]zoxideEntry, len(matches))
		for i, m := range matches {
			filtered[i] = z.all[m.Index]
		}
		z.filtered = filtered
	}
	z.cursor = 0
	z.scroll = 0
}

func (z *zoxidePicker) selected() *zoxideEntry {
	if z.cursor >= 0 && z.cursor < len(z.filtered) {
		return &z.filtered[z.cursor]
	}
	return nil
}

func (z *zoxidePicker) clampCursor() {
	if z.cursor < 0 {
		z.cursor = 0
	}
	if z.cursor >= len(z.filtered) {
		z.cursor = len(z.filtered) - 1
	}
	if z.cursor < 0 {
		z.cursor = 0
	}
}

func (z *zoxidePicker) ensureVisible(maxVisible int) {
	if maxVisible < 1 {
		maxVisible = 1
	}
	if z.cursor < z.scroll {
		z.scroll = z.cursor
	}
	if z.cursor >= z.scroll+maxVisible {
		z.scroll = z.cursor - maxVisible + 1
	}
}
