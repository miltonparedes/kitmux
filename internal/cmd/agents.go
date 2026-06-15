package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/agentenv"
	"github.com/miltonparedes/kitmux/internal/agenthooks"
	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/agentthread"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

func addAgentCommands(parent *cobra.Command) {
	for _, agent := range agents.DefaultAgents() {
		parent.AddCommand(agentCmd(agent))
	}
}

func agentCmd(agent agents.Agent) *cobra.Command {
	var (
		headless bool
		modeID   string
		dir      string
		name     string
	)

	cmd := &cobra.Command{
		Use:   agent.ID,
		Short: "Run " + agent.Name,
		RunE: func(_ *cobra.Command, _ []string) error {
			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("current directory: %w", err)
				}
				dir = cwd
			}
			mode, ok := agents.FindMode(agent, modeID)
			if !ok {
				return fmt.Errorf("unknown mode %q for %s", modeID, agent.ID)
			}
			if err := installLaunchHooks(agent.ID); err != nil {
				return err
			}
			if headless {
				return agentthread.EnsureAndAttach(agentthread.Spec{
					AgentID: agent.ID,
					ModeID:  mode.ID,
					Dir:     dir,
					Name:    name,
				}, agentthread.DefaultOps())
			}
			return execShell(agent.ID, agent.FullCommand(mode), dir)
		},
	}
	cmd.Flags().BoolVar(&headless, "headless", false, "run or attach this agent as a persistent tmux thread")
	cmd.Flags().StringVar(&modeID, "mode", "default", "agent mode")
	cmd.Flags().StringVar(&dir, "dir", "", "working directory (defaults to current directory)")
	cmd.Flags().StringVar(&name, "name", "", "headless session name (defaults to agent-project)")
	return cmd
}

func installLaunchHooks(agentID string) error {
	if _, err := agenthooks.Install(agentID, ""); err != nil && !errors.Is(err, agenthooks.ErrUnsupportedAgent) {
		return err
	}
	return nil
}

func execShell(agentID, command, dir string) error {
	sh, err := exec.LookPath("sh")
	if err != nil {
		return err
	}
	env := os.Environ()
	if ctx, err := tmux.CurrentThreadContext(); err == nil {
		env = agentenv.WithTrackingEnv(env, agentID, ctx.SessionName, ctx.PaneID, ctx.Thread)
	} else {
		env = agentenv.WithTrackingEnv(env, agentID, "", "", false)
	}
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("chdir: %w", err)
		}
	}
	return syscall.Exec(sh, []string{"sh", "-lc", command}, env)
}
