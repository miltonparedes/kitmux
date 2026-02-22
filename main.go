package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/app"
	"github.com/miltonparedes/kitmux/internal/config"
	"github.com/miltonparedes/kitmux/internal/views/palette"
)

func main() {
	mode := app.Mode(-1)
	var opts []app.Option
	args := os.Args[1:]

	// Parse global options first, collect remaining positional args
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
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
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch positional[0] {
	case "sessions", "s":
		mode = app.ModeSessions
	case "palette", "p":
		mode = app.ModePalette
	case "worktrees", "wt":
		mode = app.ModeWorktrees
	case "agents", "a":
		mode = app.ModeAgents
	case "projects", "o":
		mode = app.ModeProjects
	case "windows", "w":
		mode = app.ModeWindows
	case "run":
		if len(positional) < 2 {
			fmt.Fprintln(os.Stderr, "kitmux: run requires a command ID")
			fmt.Fprintln(os.Stderr, "")
			printAvailableCommands()
			os.Exit(1)
		}
		cmdID := positional[1]
		if !palette.IsValidCommand(cmdID) {
			fmt.Fprintf(os.Stderr, "kitmux: unknown command %q\n", cmdID)
			fmt.Fprintln(os.Stderr, "")
			printAvailableCommands()
			os.Exit(1)
		}
		mode = app.ModeRun
		opts = append(opts, app.WithRunCommand(cmdID))
	case "commands":
		printAvailableCommands()
		return
	default:
		// Check if it's a valid command ID (shorthand for "run <id>")
		if palette.IsValidCommand(positional[0]) {
			mode = app.ModeRun
			opts = append(opts, app.WithRunCommand(positional[0]))
		} else {
			fmt.Fprintf(os.Stderr, "kitmux: unknown command %q\n", positional[0])
			fmt.Fprintln(os.Stderr, "")
			printUsage()
			os.Exit(1)
		}
	}

	p := tea.NewProgram(app.New(mode, opts...), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kitmux: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: kitmux <command> [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Views:")
	fmt.Fprintln(os.Stderr, "  sessions (s)    Session tree")
	fmt.Fprintln(os.Stderr, "  palette  (p)    Command palette")
	fmt.Fprintln(os.Stderr, "  worktrees (wt)  Worktree manager")
	fmt.Fprintln(os.Stderr, "  agents   (a)    Agent launcher")
	fmt.Fprintln(os.Stderr, "  projects (o)    Open a project")
	fmt.Fprintln(os.Stderr, "  windows  (w)    Window list for current session")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Execute:")
	fmt.Fprintln(os.Stderr, "  run <id>        Run a palette command by ID")
	fmt.Fprintln(os.Stderr, "  commands        List all available command IDs")
	fmt.Fprintln(os.Stderr, "  <id>            Shorthand for 'run <id>'")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  --super KEY     Modifier for 1-9 jump (alt|none, default: none)")
}

func printAvailableCommands() {
	fmt.Fprintln(os.Stderr, "Available commands:")
	fmt.Fprintln(os.Stderr, "")
	var lastCat string
	for _, cmd := range palette.DefaultCommands() {
		if cmd.Category != lastCat {
			if lastCat != "" {
				fmt.Fprintln(os.Stderr, "")
			}
			fmt.Fprintf(os.Stderr, "  %s:\n", cmd.Category)
			lastCat = cmd.Category
		}
		fmt.Fprintf(os.Stderr, "    %-24s %s\n", cmd.ID, cmd.Description)
	}
}
