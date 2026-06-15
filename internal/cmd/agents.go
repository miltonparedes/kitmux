package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/agentthread"
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
			if headless {
				return agentthread.EnsureAndAttach(agentthread.Spec{
					AgentID: agent.ID,
					ModeID:  mode.ID,
					Dir:     dir,
					Name:    name,
				}, agentthread.DefaultOps())
			}
			return execShell(agent.FullCommand(mode), dir)
		},
	}
	cmd.Flags().BoolVar(&headless, "headless", false, "run or attach this agent as a persistent tmux thread")
	cmd.Flags().StringVar(&modeID, "mode", "default", "agent mode")
	cmd.Flags().StringVar(&dir, "dir", "", "working directory (defaults to current directory)")
	cmd.Flags().StringVar(&name, "name", "", "headless session name (defaults to agent-project)")
	return cmd
}

func execShell(command, dir string) error {
	sh, err := exec.LookPath("sh")
	if err != nil {
		return err
	}
	env := os.Environ()
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("chdir: %w", err)
		}
	}
	return syscall.Exec(sh, []string{"sh", "-lc", command}, env)
}
