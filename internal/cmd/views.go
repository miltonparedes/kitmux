package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/agenthooks"
	"github.com/miltonparedes/kitmux/internal/agentthread"
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
	{"windows", []string{"w"}, "Window list for current session", app.ModeWindows},
	{"workspaces", []string{"o"}, "Workspace manager", app.ModeWorkspaces},
	{"sidepanel", nil, "Agent sidepanel", app.ModeSidepanel},
	{"threads", []string{"t"}, "Running agent threads", app.ModeThreads},
}

func addViewCommands(parent *cobra.Command) {
	for _, v := range views {
		parent.AddCommand(viewCmd(v))
	}
}

func viewCmd(v viewDef) *cobra.Command {
	var installHooks bool
	var installAgentHooks bool
	command := &cobra.Command{
		Use:     v.name,
		Aliases: v.aliases,
		Short:   v.short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if v.mode == app.ModeThreads && installAgentHooks {
				return installAgentHookSupport(cmd, nil)
			}
			if v.mode == app.ModeThreads && installHooks {
				return installAgentSupport(cmd, nil)
			}
			return runTUI(v.mode)
		},
	}
	if v.mode == app.ModeThreads {
		command.Flags().BoolVar(&installHooks, "install-support", false,
			"install or update official kitmux agent support on existing headless threads")
		command.Flags().BoolVar(&installHooks, "install-hooks", false,
			"install or update official kitmux agent hooks on existing headless threads")
		_ = command.Flags().MarkHidden("install-hooks")
		command.Flags().BoolVar(&installAgentHooks, "install-agent-hooks", false,
			"install or update notification hooks inside supported agent CLIs")
		command.AddCommand(&cobra.Command{
			Use:     "install-support",
			Aliases: []string{"install-hooks", "sync-hooks"},
			Short:   "Install or update official kitmux agent support",
			RunE:    installAgentSupport,
		})
		command.AddCommand(&cobra.Command{
			Use:     "install-agent-hooks",
			Aliases: []string{"install-agents", "sync-agent-hooks"},
			Short:   "Install or update notification hooks inside supported agent CLIs",
			RunE:    installAgentHookSupport,
		})
	}
	return command
}

func installAgentSupport(cmd *cobra.Command, _ []string) error {
	count, err := agentthread.InstallAllSupport(agentthread.DefaultOps())
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Installed kitmux agent support for %d thread(s).\n", count)
	return nil
}

func installAgentHookSupport(cmd *cobra.Command, _ []string) error {
	results, err := agenthooks.InstallAll("")
	if err != nil {
		return err
	}
	for _, result := range results {
		status := "ok"
		if result.Changed {
			status = "updated"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\n", result.AgentID, status, result.Path)
	}
	return nil
}

func runTUI(mode app.Mode, opts ...app.Option) error {
	p := tea.NewProgram(app.New(mode, opts...), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
