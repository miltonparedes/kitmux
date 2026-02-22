package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/app"
	"github.com/miltonparedes/kitmux/internal/views/palette"
)

func addRunCommand(parent *cobra.Command) {
	parent.AddCommand(&cobra.Command{
		Use:   "run <command-id>",
		Short: "Run a palette command by ID",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var ids []string
			for _, c := range palette.DefaultCommands() {
				ids = append(ids, c.ID+"\t"+c.Description)
			}
			return ids, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			if !palette.IsValidCommand(id) {
				return fmt.Errorf("unknown command %q", id)
			}
			return runTUI(app.ModeRun, app.WithRunCommand(id))
		},
	})
}
