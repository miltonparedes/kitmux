package cmd

import (
	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/app"
	"github.com/miltonparedes/kitmux/internal/config"
	"github.com/miltonparedes/kitmux/internal/views/palette"
)

// Execute builds the CLI and runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kitmux",
		Short:        "tmux session manager and command palette",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&config.SuperKey, "super", "none",
		"modifier for 1-9 jump (alt|none)")

	addViewCommands(cmd)
	addRunCommand(cmd)
	addCommandsCommand(cmd)

	// Register each palette command ID as a hidden subcommand so that
	// "kitmux switch_session" works as shorthand for "kitmux run switch_session".
	for _, c := range palette.DefaultCommands() {
		cmd.AddCommand(hiddenRunCmd(c.ID, c.Description))
	}

	return cmd
}

func hiddenRunCmd(id, short string) *cobra.Command {
	return &cobra.Command{
		Use:    id,
		Short:  short,
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTUI(app.ModeRun, app.WithRunCommand(id))
		},
	}
}
