package cmd

import (
	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/agenthooks"
)

func addHookCommand(parent *cobra.Command) {
	hookCmd := &cobra.Command{
		Use:    "hook",
		Short:  "Internal hook entrypoints",
		Hidden: true,
	}

	var state string
	var bell bool
	agentStateCmd := &cobra.Command{
		Use:    "agent-state",
		Short:  "Update agent state for the current tmux pane",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return agenthooks.RunStateEvent(agenthooks.StateEvent{
				State: state,
				Bell:  bell,
			}, cmd.OutOrStdout(), agenthooks.DefaultStateOps())
		},
	}
	agentStateCmd.Flags().StringVar(&state, "state", "", "agent state: idle, working, input")
	agentStateCmd.Flags().BoolVar(&bell, "bell", false, "emit a terminal bell")
	hookCmd.AddCommand(agentStateCmd)
	parent.AddCommand(hookCmd)
}
