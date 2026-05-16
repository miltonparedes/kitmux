package agentlaunch

import (
	"testing"

	"github.com/miltonparedes/kitmux/internal/agents"
)

func TestLaunchInSessionFreshSessionOpensWorkbench(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "always")
	t.Setenv("KITMUX_WORKBENCH_COMMAND", "kitmux workbench")

	calls := &launchCalls{}
	err := LaunchInSession(sessionReq(true, TargetWindow), stubOps(calls))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if calls.renamedTarget != "kitmux-main:0" {
		t.Fatalf("expected window 0 rename, got %q", calls.renamedTarget)
	}
	if calls.sentTarget != "kitmux-main:0" || calls.sentKeys != "droid" {
		t.Fatalf("expected droid sent to window 0, got target=%q keys=%q", calls.sentTarget, calls.sentKeys)
	}
	if calls.workbenchTarget != "kitmux-main:0" || calls.workbenchDir != "/repo" {
		t.Fatalf("expected workbench beside reused pane, got target=%q dir=%q", calls.workbenchTarget, calls.workbenchDir)
	}
}

func TestLaunchInSessionWindowOpensWorkbenchFromPaneID(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "always")

	calls := &launchCalls{}
	err := LaunchInSession(sessionReq(false, TargetWindow), stubOps(calls))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if calls.windowName != "droid" || calls.windowDir != "/repo" || calls.windowCommand != "droid" {
		t.Fatalf("unexpected window launch: name=%q dir=%q command=%q", calls.windowName, calls.windowDir, calls.windowCommand)
	}
	if calls.workbenchTarget != "%9" {
		t.Fatalf("expected workbench target %%9, got %q", calls.workbenchTarget)
	}
}

func TestLaunchInSessionSplitOpensWorkbenchFromPaneID(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "always")

	calls := &launchCalls{}
	err := LaunchInSession(sessionReq(false, TargetSplit), stubOps(calls))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if calls.splitTarget != "kitmux-main:" || calls.splitCommand != "droid" {
		t.Fatalf("unexpected split launch: target=%q command=%q", calls.splitTarget, calls.splitCommand)
	}
	if calls.workbenchTarget != "%7" {
		t.Fatalf("expected workbench target %%7, got %q", calls.workbenchTarget)
	}
}

func TestLaunchInSessionHonorsWorkbenchOff(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "off")

	calls := &launchCalls{}
	err := LaunchInSession(sessionReq(false, TargetWindow), stubOps(calls))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if calls.workbenchCommand != "" {
		t.Fatalf("expected no workbench split, got %q", calls.workbenchCommand)
	}
}

type launchCalls struct {
	sentTarget       string
	sentKeys         string
	renamedTarget    string
	windowName       string
	windowDir        string
	windowCommand    string
	splitTarget      string
	splitCommand     string
	workbenchTarget  string
	workbenchDir     string
	workbenchCommand string
}

func sessionReq(fresh bool, target Target) SessionRequest {
	return SessionRequest{
		SessionName:   "kitmux-main",
		WindowName:    "droid",
		Dir:           "/repo",
		Agent:         agents.Agent{ID: "droid", Name: "Droid", Command: "droid"},
		Mode:          agents.AgentMode{ID: "default", Name: "Default"},
		Target:        target,
		FreshSession:  fresh,
		OpenWorkbench: true,
	}
}

func stubOps(calls *launchCalls) Ops {
	return Ops{
		SendKeys: func(target, keys string) error {
			calls.sentTarget = target
			calls.sentKeys = keys
			return nil
		},
		NewWindowInSession: func(_, name, dir, command string) (string, error) {
			calls.windowName = name
			calls.windowDir = dir
			calls.windowCommand = command
			return "%9", nil
		},
		SplitWindowInDir: func(target, _, command string) (string, error) {
			calls.splitTarget = target
			calls.splitCommand = command
			return "%7", nil
		},
		SplitWindowInDirPercent: func(target, dir, command string, _ int) (string, error) {
			calls.workbenchTarget = target
			calls.workbenchDir = dir
			calls.workbenchCommand = command
			return "%8", nil
		},
		CurrentClientWidth: func() (int, error) { return 240, nil },
		RenameWindow: func(target, _ string) error {
			calls.renamedTarget = target
			return nil
		},
	}
}
