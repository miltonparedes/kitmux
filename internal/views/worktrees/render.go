package worktrees

import (
	"fmt"
	"strings"

	"github.com/miltonparedes/kitmux/internal/theme"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

func (m Model) View() string {
	if len(m.worktrees) == 0 {
		return theme.HelpStyle.Render(" No worktrees found")
	}

	var b strings.Builder

	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	itemSep := " " + theme.TreeMeta.Render(strings.Repeat("─", sepW))

	avail := m.height - 2
	if avail < 1 {
		avail = 1
	}
	maxVisible := (avail + 1) / 2

	start := m.scroll
	end := start + maxVisible
	if end > len(m.worktrees) {
		end = len(m.worktrees)
	}

	for i := start; i < end; i++ {
		w := &m.worktrees[i]
		selected := i == m.cursor
		if i < 9 {
			b.WriteString(theme.TreeMeta.Render(fmt.Sprintf("%d", i+1)))
		} else {
			b.WriteString(" ")
		}
		b.WriteString(renderWorktree(w, selected))
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
		branch := ""
		if wt := m.selected(); wt != nil {
			branch = wt.Branch
		}
		return theme.AttachedBadge.Render(fmt.Sprintf(" remove '%s'? y/n", branch))
	}
	if m.describing {
		return " " + m.describeInput.View()
	}
	if m.confirmingBranch {
		return " " + m.branchInput.View()
	}
	if m.creating {
		return " " + m.newInput.View()
	}
	return theme.HelpStyle.Render(" ⏎ switch  n new  N describe  d rm  q quit")
}

func renderWorktree(w *worktree.Worktree, selected bool) string {
	var parts []string

	// Cursor indicator
	if selected {
		parts = append(parts, "▸")
	} else {
		parts = append(parts, " ")
	}

	// Branch name
	if selected {
		parts = append(parts, theme.TreeNodeSelected.Render(w.Branch))
	} else {
		parts = append(parts, theme.TreeNodeNormal.Render(w.Branch))
	}

	// Status symbols
	if w.Symbols != "" {
		if strings.ContainsAny(w.Symbols, "+!?") {
			parts = append(parts, theme.DirtyBadge.Render(w.Symbols))
		} else {
			parts = append(parts, theme.CleanBadge.Render(w.Symbols))
		}
	}

	// Diff stats
	if w.WorkingTree.Diff.Added > 0 || w.WorkingTree.Diff.Deleted > 0 {
		if w.WorkingTree.Diff.Added > 0 {
			parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", w.WorkingTree.Diff.Added)))
		}
		if w.WorkingTree.Diff.Deleted > 0 {
			parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", w.WorkingTree.Diff.Deleted)))
		}
	}

	// Remote ahead/behind
	if w.Remote.Ahead > 0 {
		parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("⇡%d", w.Remote.Ahead)))
	}
	if w.Remote.Behind > 0 {
		parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("⇣%d", w.Remote.Behind)))
	}

	// Current indicator
	if w.IsCurrent {
		parts = append(parts, theme.AttachedBadge.Render("●"))
	}

	return " " + strings.Join(parts, " ")
}
