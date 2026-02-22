package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/app"
)

type viewDef struct {
	name    string
	aliases []string
	short   string
	mode    app.Mode
}

var views = []viewDef{
	{"sessions", []string{"s"}, "Session tree", app.ModeSessions},
	{"palette", []string{"p"}, "Command palette", app.ModePalette},
	{"worktrees", []string{"wt"}, "Worktree manager", app.ModeWorktrees},
	{"agents", []string{"a"}, "Agent launcher", app.ModeAgents},
	{"projects", []string{"o"}, "Open a project", app.ModeProjects},
	{"windows", []string{"w"}, "Window list for current session", app.ModeWindows},
}

func addViewCommands(parent *cobra.Command) {
	for _, v := range views {
		parent.AddCommand(viewCmd(v))
	}
}

func viewCmd(v viewDef) *cobra.Command {
	return &cobra.Command{
		Use:     v.name,
		Aliases: v.aliases,
		Short:   v.short,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTUI(v.mode)
		},
	}
}

func runTUI(mode app.Mode, opts ...app.Option) error {
	p := tea.NewProgram(app.New(mode, opts...), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
