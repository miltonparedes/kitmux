package cmd

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/agenthooks"
	"github.com/miltonparedes/kitmux/internal/agenttrack"
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

	var agent string
	var eventName string
	var detail string
	var stdinJSON bool
	agentEventCmd := &cobra.Command{
		Use:    "agent-event",
		Short:  "Update agent state metadata for the current tmux pane",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return agenthooks.RunAgentEvent(agenthooks.AgentEvent{
				Agent:     agent,
				Event:     eventName,
				State:     state,
				Detail:    detail,
				Bell:      bell,
				StdinJSON: stdinJSON,
			}, cmd.InOrStdin(), cmd.OutOrStdout(), agenthooks.DefaultStateOps())
		},
	}
	agentEventCmd.Flags().StringVar(&agent, "agent", "", "agent id")
	agentEventCmd.Flags().StringVar(&eventName, "event", "", "agent hook event")
	agentEventCmd.Flags().StringVar(&state, "state", "", "agent state: idle, working, input, permission, error")
	agentEventCmd.Flags().StringVar(&detail, "detail", "", "short event detail")
	agentEventCmd.Flags().BoolVar(&bell, "bell", false, "emit a terminal bell")
	agentEventCmd.Flags().BoolVar(&stdinJSON, "stdin-json", false, "read hook JSON from stdin")
	hookCmd.AddCommand(agentEventCmd)

	var pane string
	var session string
	var token string
	agentSpinnerCmd := &cobra.Command{
		Use:    "agent-spinner",
		Short:  "Animate agent title prefix while the current event is working",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return agenthooks.RunSpinner(agenthooks.SpinnerTarget{
				PaneID:      pane,
				SessionName: session,
				AgentID:     agent,
				Token:       token,
			})
		},
	}
	agentSpinnerCmd.Flags().StringVar(&pane, "pane", "", "tmux pane id")
	agentSpinnerCmd.Flags().StringVar(&session, "session", "", "tmux session name")
	agentSpinnerCmd.Flags().StringVar(&agent, "agent", "", "agent id")
	agentSpinnerCmd.Flags().StringVar(&token, "token", "", "event freshness token")
	hookCmd.AddCommand(agentSpinnerCmd)

	hookCmd.AddCommand(agentRegisterCommand())

	parent.AddCommand(hookCmd)
}

func agentRegisterCommand() *cobra.Command {
	var agent string
	var pid string
	var session string
	var pane string
	var thread string
	cmd := &cobra.Command{
		Use:    "agent-register",
		Short:  "Register a running agent process for hook targeting",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			pidInt, err := strconv.Atoi(pid)
			if err != nil {
				return err
			}
			return agenttrack.Register(pidInt, agenttrack.Context{
				AgentID:     agent,
				SessionName: session,
				PaneID:      pane,
				Thread:      thread == "1" || thread == "true",
			})
		},
	}
	cmd.Flags().StringVar(&pid, "pid", "", "agent process id")
	cmd.Flags().StringVar(&agent, "agent", "", "agent id")
	cmd.Flags().StringVar(&session, "session", "", "tmux session name")
	cmd.Flags().StringVar(&pane, "pane", "", "tmux pane id")
	cmd.Flags().StringVar(&thread, "thread", "", "tmux thread marker")
	return cmd
}
