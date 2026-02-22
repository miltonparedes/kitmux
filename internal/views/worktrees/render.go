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
		wt := &m.worktrees[i]
		selected := i == m.cursor
		b.WriteString(renderWorktree(wt, selected))
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
		return theme.AttachedBadge.Render(" remove? y/n")
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

func renderWorktree(wt *worktree.Worktree, selected bool) string {
	var parts []string

	// Cursor indicator
	if selected {
		parts = append(parts, "▸")
	} else {
		parts = append(parts, " ")
	}

	// Branch name
	if selected {
		parts = append(parts, theme.TreeNodeSelected.Render(wt.Branch))
	} else {
		parts = append(parts, theme.TreeNodeNormal.Render(wt.Branch))
	}

	// Status symbols
	if wt.Symbols != "" {
		if strings.ContainsAny(wt.Symbols, "+!?") {
			parts = append(parts, theme.DirtyBadge.Render(wt.Symbols))
		} else {
			parts = append(parts, theme.CleanBadge.Render(wt.Symbols))
		}
	}

	// Diff stats
	if wt.WorkingTree.Diff.Added > 0 || wt.WorkingTree.Diff.Deleted > 0 {
		if wt.WorkingTree.Diff.Added > 0 {
			parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("+%d", wt.WorkingTree.Diff.Added)))
		}
		if wt.WorkingTree.Diff.Deleted > 0 {
			parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("-%d", wt.WorkingTree.Diff.Deleted)))
		}
	}

	// Remote ahead/behind
	if wt.Remote.Ahead > 0 {
		parts = append(parts, theme.DiffAdded.Render(fmt.Sprintf("⇡%d", wt.Remote.Ahead)))
	}
	if wt.Remote.Behind > 0 {
		parts = append(parts, theme.DiffRemoved.Render(fmt.Sprintf("⇣%d", wt.Remote.Behind)))
	}

	// Current indicator
	if wt.IsCurrent {
		parts = append(parts, theme.AttachedBadge.Render("●"))
	}

	return " " + strings.Join(parts, " ")
}
