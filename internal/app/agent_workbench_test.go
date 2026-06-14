package app

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agentlaunch"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/cache"
	"github.com/miltonparedes/kitmux/internal/store"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestLaunchAgentAutoWorkbenchWhenWide(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "auto")
	t.Setenv("KITMUX_AGENT_WORKBENCH_MIN_WIDTH", "160")
	t.Setenv("KITMUX_WORKBENCH_COMMAND", "")

	calls := stubAgentLaunch(t, 200, nil)

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"})

	if calls.sent != "codex" {
		t.Fatalf("expected codex to be sent, got %q", calls.sent)
	}
	if calls.sidepanelCommand != "kitmux workbench" {
		t.Fatalf("expected workbench split, got %q", calls.sidepanelCommand)
	}
	if calls.sidepanelRatio != 30 {
		t.Fatalf("expected 30%% workbench split, got %d", calls.sidepanelRatio)
	}
}

func TestLaunchAgentAutoWorkbenchSkipsWhenNarrow(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "auto")
	t.Setenv("KITMUX_AGENT_WORKBENCH_MIN_WIDTH", "160")

	calls := stubAgentLaunch(t, 120, nil)

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"})

	if calls.sent != "codex" {
		t.Fatalf("expected codex to be sent, got %q", calls.sent)
	}
	if calls.sidepanelCommand != "" {
		t.Fatalf("expected no workbench split, got %q", calls.sidepanelCommand)
	}
}

func TestLaunchAgentAlwaysWorkbenchIgnoresWidthError(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "always")

	calls := stubAgentLaunch(t, 0, errors.New("no tmux"))

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"})

	if calls.sidepanelCommand != "kitmux workbench" {
		t.Fatalf("expected workbench split, got %q", calls.sidepanelCommand)
	}
}

func TestLaunchAgentOffNeverStartsWorkbench(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "off")

	calls := stubAgentLaunch(t, 240, nil)

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"})

	if calls.sidepanelCommand != "" {
		t.Fatalf("expected no workbench split, got %q", calls.sidepanelCommand)
	}
}

func TestLaunchAgentExplicitSplitDoesNotStartWorkbench(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "always")

	calls := stubAgentLaunch(t, 240, nil)

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "split"})

	if calls.split != "codex" {
		t.Fatalf("expected explicit split command codex, got %q", calls.split)
	}
	if calls.sidepanelCommand != "" {
		t.Fatalf("expected no workbench split, got %q", calls.sidepanelCommand)
	}
}

func TestWorkbenchEditorResultKeepsPaneAlive(t *testing.T) {
	m := New(ModeSidepanel)

	_, cmd, handled := m.handleOpenLocalEditor(messages.OpenLocalEditorMsg{})
	if !handled {
		t.Fatal("expected editor result to be handled")
	}
	if cmd != nil {
		t.Fatal("expected workbench editor result to keep pane alive")
	}
}

func TestWorkbenchEscReturnsFromAuxView(t *testing.T) {
	m := New(ModeSidepanel)
	m.view = viewAgents

	updated, _, handled := m.handleEscByView()
	if !handled {
		t.Fatal("expected esc to be handled")
	}
	if updated.view != viewSidepanel {
		t.Fatalf("expected viewSidepanel, got %v", updated.view)
	}
}

func TestWorkbenchEscCancelsLaunchPickerWithoutQuitting(t *testing.T) {
	m := New(ModeSidepanel)
	m.sidepanelView = m.sidepanelView.StartAgentLaunchForTest()

	updated, cmd, handled := m.handleEscKey(tea.KeyMsg{Type: tea.KeyEsc}, true)
	if !handled {
		t.Fatal("expected esc to be handled")
	}
	if cmd != nil {
		t.Fatal("expected no quit command")
	}
	if updated.sidepanelView.IsEditing() {
		t.Fatal("expected workbench picker to be cancelled")
	}
	if updated.view != viewSidepanel {
		t.Fatalf("expected viewSidepanel, got %v", updated.view)
	}
}

type launchCalls struct {
	sent             string
	split            string
	windowDir        string
	windowCommand    string
	sidepanelCommand string
	sidepanelRatio   int
}

func stubAgentLaunch(t *testing.T, width int, widthErr error) *launchCalls {
	t.Helper()

	originalOps := agentLaunchOps
	t.Cleanup(func() {
		agentLaunchOps = originalOps
	})

	calls := &launchCalls{}
	agentLaunchOps = agentlaunch.Ops{}
	agentLaunchOps.SendKeys = func(_, keys string) error {
		calls.sent = keys
		return nil
	}
	agentLaunchOps.SplitWindow = func(command string) error {
		calls.split = command
		return nil
	}
	agentLaunchOps.NewWindowWithCommand = func(_, _ string) error {
		return nil
	}
	agentLaunchOps.NewWindowInDir = func(_, dir, command string) (string, error) {
		calls.windowDir = dir
		calls.windowCommand = command
		return "%9", nil
	}
	agentLaunchOps.CurrentClientWidth = func() (int, error) {
		return width, widthErr
	}
	agentLaunchOps.SplitWindowInDirPercent = func(_, _, command string, percent int) (string, error) {
		calls.sidepanelCommand = command
		calls.sidepanelRatio = percent
		return "%2", nil
	}
	return calls
}

func TestWorkbenchLaunchAgentCreatesWindowAndSplitWhenWide(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "auto")
	t.Setenv("KITMUX_AGENT_WORKBENCH_MIN_WIDTH", "160")

	calls := stubAgentLaunch(t, 220, nil)
	m := New(ModeSidepanel)
	cmd := m.launchSidepanelAgent(messages.LaunchSidepanelAgentMsg{AgentID: "codex", ModeID: "default", Dir: "/tmp/repo"})
	if cmd == nil {
		t.Fatal("expected command")
	}
	_ = cmd()
	if calls.windowDir != "/tmp/repo" {
		t.Fatalf("expected window dir /tmp/repo, got %q", calls.windowDir)
	}
	if calls.windowCommand != "codex" {
		t.Fatalf("expected codex command, got %q", calls.windowCommand)
	}
	if calls.sidepanelCommand != "kitmux workbench" {
		t.Fatalf("expected workbench split, got %q", calls.sidepanelCommand)
	}
	if calls.sidepanelRatio != 30 {
		t.Fatalf("expected 30%% workbench split, got %d", calls.sidepanelRatio)
	}
}

func TestWorkbenchLaunchAgentSkipsSplitWhenNarrow(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "auto")
	t.Setenv("KITMUX_AGENT_WORKBENCH_MIN_WIDTH", "160")

	calls := stubAgentLaunch(t, 100, nil)
	m := New(ModeSidepanel)
	_ = m.launchSidepanelAgent(messages.LaunchSidepanelAgentMsg{AgentID: "aichat", ModeID: "default", Dir: "/tmp/repo"})()
	if calls.windowCommand != "aichat" {
		t.Fatalf("expected aichat command, got %q", calls.windowCommand)
	}
	if calls.sidepanelCommand != "" {
		t.Fatalf("expected no workbench split, got %q", calls.sidepanelCommand)
	}
}

func TestSessionsEnterPreservesSwitchCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store.ResetForTests()
	t.Cleanup(store.ResetForTests)

	if err := cache.Save(&cache.Snapshot{
		Sessions:  []tmux.Session{{Name: "target", Windows: 1, Path: "/tmp/target"}},
		RepoRoots: map[string]string{"target": "/tmp/target"},
	}); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	m := New(ModeSessions)
	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("expected cached init command")
	}
	model, _ := m.routeToSessions(initCmd())
	m = model.(Model)

	_, cmd := m.routeToSessions(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter to emit switch-session command")
	}
	msg, ok := cmd().(messages.SwitchSessionMsg)
	if !ok {
		t.Fatalf("expected SwitchSessionMsg, got %T", cmd())
	}
	if msg.Name != "target" {
		t.Fatalf("expected target session, got %q", msg.Name)
	}
}
