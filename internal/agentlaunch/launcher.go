package agentlaunch

import (
	"errors"
	"fmt"

	"github.com/miltonparedes/kitmux/internal/agentenv"
	"github.com/miltonparedes/kitmux/internal/agenthooks"
	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/config"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

type Target string

const (
	TargetPane        Target = "pane"
	TargetSplit       Target = "split"
	TargetWindow      Target = "window"
	CurrentPaneTarget        = "!"
)

type Ops struct {
	SendKeys                func(string, string) error
	SplitWindow             func(string) error
	NewWindowWithCommand    func(string, string) error
	NewWindowInDir          func(string, string, string) (string, error)
	NewWindowInSession      func(string, string, string, string) (string, error)
	SplitWindowInDir        func(string, string, string) (string, error)
	SplitWindowInDirPercent func(string, string, string, int) (string, error)
	CurrentClientWidth      func() (int, error)
	RenameWindow            func(string, string) error
	InstallHooks            func(string) error
}

type SessionRequest struct {
	SessionName   string
	WindowName    string
	Dir           string
	Agent         agents.Agent
	Mode          agents.AgentMode
	Target        Target
	FreshSession  bool
	OpenSidepanel bool
}

func DefaultOps() Ops {
	return Ops{
		SendKeys:                tmux.SendKeys,
		SplitWindow:             tmux.SplitWindow,
		NewWindowWithCommand:    tmux.NewWindowWithCommand,
		NewWindowInDir:          tmux.NewWindowInDir,
		NewWindowInSession:      tmux.NewWindowInSessionPaneID,
		SplitWindowInDir:        tmux.SplitWindowInDir,
		SplitWindowInDirPercent: tmux.SplitWindowInDirPercent,
		CurrentClientWidth:      tmux.CurrentClientWidth,
		RenameWindow:            tmux.RenameWindow,
		InstallHooks:            InstallHooks,
	}
}

func InstallHooks(agentID string) error {
	if _, err := agenthooks.Install(agentID, ""); err != nil && !errors.Is(err, agenthooks.ErrUnsupportedAgent) {
		return err
	}
	return nil
}

func LaunchCurrent(agent agents.Agent, mode agents.AgentMode, target Target, ops Ops) error {
	ops = ops.withDefaults()
	if err := ops.InstallHooks(agent.ID); err != nil {
		return err
	}
	command := agentenv.WrapTmuxCommand(agent.ID, "", agent.FullCommand(mode), false)
	switch target {
	case TargetSplit:
		return ops.SplitWindow(command)
	case TargetWindow:
		return ops.NewWindowWithCommand(agent.Name, command)
	default:
		if err := ops.SendKeys(CurrentPaneTarget, command); err != nil {
			return err
		}
		return OpenSidepanelSplit("", CurrentPaneTarget, ops)
	}
}

func LaunchSidepanelWindow(agent agents.Agent, mode agents.AgentMode, dir string, ops Ops) error {
	ops = ops.withDefaults()
	if err := ops.InstallHooks(agent.ID); err != nil {
		return err
	}
	paneID, err := ops.NewWindowInDir(agent.ID, dir, agentenv.WrapTmuxCommand(agent.ID, "", agent.FullCommand(mode), false))
	if err != nil {
		return err
	}
	return OpenSidepanelSplit(dir, paneID, ops)
}

func LaunchInSession(req SessionRequest, ops Ops) error {
	ops = ops.withDefaults()
	if err := ops.InstallHooks(req.Agent.ID); err != nil {
		return err
	}
	command := agentenv.WrapTmuxCommand(req.Agent.ID, req.SessionName, req.Agent.FullCommand(req.Mode), false)

	if req.FreshSession {
		target := req.SessionName + ":0"
		_ = ops.RenameWindow(target, req.Agent.ID)
		if err := ops.SendKeys(target, command); err != nil {
			return err
		}
		return openSidepanelIfRequested(req, req.Dir, target, ops)
	}

	switch req.Target {
	case TargetSplit:
		paneID, err := ops.SplitWindowInDir(req.SessionName+":", req.Dir, command)
		if err != nil {
			return fmt.Errorf("tmux split-window failed: %w", err)
		}
		return openSidepanelIfRequested(req, req.Dir, paneID, ops)
	default:
		paneID, err := ops.NewWindowInSession(req.SessionName, req.WindowName, req.Dir, command)
		if err != nil {
			return fmt.Errorf("tmux new-window failed: %w", err)
		}
		return openSidepanelIfRequested(req, req.Dir, paneID, ops)
	}
}

func OpenSidepanelSplit(dir, targetPane string, ops Ops) error {
	ops = ops.withDefaults()
	if !ShouldOpenSidepanel(ops) {
		return nil
	}
	_, err := ops.SplitWindowInDirPercent(
		targetPane,
		dir,
		config.SidepanelCommand(),
		config.AgentSidepanelRatio(),
	)
	return err
}

func ShouldOpenSidepanel(ops Ops) bool {
	ops = ops.withDefaults()
	switch config.AgentSidepanel() {
	case "always":
		return true
	case "off":
		return false
	default:
		width, err := ops.CurrentClientWidth()
		if err != nil {
			return false
		}
		return width >= config.AgentSidepanelMinWidth()
	}
}

func openSidepanelIfRequested(req SessionRequest, dir, targetPane string, ops Ops) error {
	if !req.OpenSidepanel {
		return nil
	}
	return OpenSidepanelSplit(dir, targetPane, ops)
}

func (ops Ops) withDefaults() Ops {
	defaults := DefaultOps()
	if ops.SendKeys == nil {
		ops.SendKeys = defaults.SendKeys
	}
	if ops.SplitWindow == nil {
		ops.SplitWindow = defaults.SplitWindow
	}
	if ops.NewWindowWithCommand == nil {
		ops.NewWindowWithCommand = defaults.NewWindowWithCommand
	}
	if ops.NewWindowInDir == nil {
		ops.NewWindowInDir = defaults.NewWindowInDir
	}
	if ops.NewWindowInSession == nil {
		ops.NewWindowInSession = defaults.NewWindowInSession
	}
	if ops.SplitWindowInDir == nil {
		ops.SplitWindowInDir = defaults.SplitWindowInDir
	}
	if ops.SplitWindowInDirPercent == nil {
		ops.SplitWindowInDirPercent = defaults.SplitWindowInDirPercent
	}
	if ops.CurrentClientWidth == nil {
		ops.CurrentClientWidth = defaults.CurrentClientWidth
	}
	if ops.RenameWindow == nil {
		ops.RenameWindow = defaults.RenameWindow
	}
	if ops.InstallHooks == nil {
		ops.InstallHooks = defaults.InstallHooks
	}
	return ops
}
