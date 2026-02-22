package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/app"
	"github.com/miltonparedes/kitmux/internal/config"
)

func main() {
	mode := app.Mode(-1)
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--sidebar", "-s":
			mode = app.ModeSidebar
		case "--palette", "-p":
			mode = app.ModePalette
		case "--worktrees", "-w":
			mode = app.ModeWorktrees
		case "--agents", "-a":
			mode = app.ModeAgents
		case "--super":
			if i+1 < len(args) {
				i++
				switch args[i] {
				case "alt", "none":
					config.SuperKey = args[i]
				default:
					fmt.Fprintf(os.Stderr, "kitmux: unknown --super value %q (use alt or none)\n", args[i])
					os.Exit(1)
				}
			}
		}
	}

	if mode < 0 {
		fmt.Fprintln(os.Stderr, "Usage: kitmux <flag>")
		fmt.Fprintln(os.Stderr, "  -s, --sidebar     Session sidebar")
		fmt.Fprintln(os.Stderr, "  -p, --palette     Command palette")
		fmt.Fprintln(os.Stderr, "  -w, --worktrees   Worktree manager")
		fmt.Fprintln(os.Stderr, "  -a, --agents      Agent launcher")
		fmt.Fprintln(os.Stderr, "      --super KEY   Modifier for 1-9 jump (alt|none, default: alt)")
		os.Exit(1)
	}

	p := tea.NewProgram(app.New(mode), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kitmux: %v\n", err)
		os.Exit(1)
	}
}
