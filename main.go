package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/app"
)

func main() {
	mode := app.Mode(-1)
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--sidebar", "-s":
			mode = app.ModeSidebar
		case "--palette", "-p":
			mode = app.ModePalette
		case "--worktrees", "-w":
			mode = app.ModeWorktrees
		case "--agents", "-a":
			mode = app.ModeAgents
		}
	}

	if mode < 0 {
		fmt.Fprintln(os.Stderr, "Usage: kitmux <flag>")
		fmt.Fprintln(os.Stderr, "  -s, --sidebar     Session sidebar")
		fmt.Fprintln(os.Stderr, "  -p, --palette     Command palette")
		fmt.Fprintln(os.Stderr, "  -w, --worktrees   Worktree manager")
		fmt.Fprintln(os.Stderr, "  -a, --agents      Agent launcher")
		os.Exit(1)
	}

	p := tea.NewProgram(app.New(mode), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kitmux: %v\n", err)
		os.Exit(1)
	}
}
