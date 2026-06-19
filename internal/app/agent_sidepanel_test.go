package app

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agentenv"
	"github.com/miltonparedes/kitmux/internal/agentlaunch"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/cache"
	"github.com/miltonparedes/kitmux/internal/store"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestLaunchAgentAutoSidepanelWhenWide(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "auto")
	t.Setenv("KITMUX_AGENT_SIDEPANEL_MIN_WIDTH", "160")
	t.Setenv("KITMUX_SIDEPANEL_COMMAND", "")

	calls := stubAgentLaunch(t, 200, nil)

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"})

	if calls.sent != trackedCommand("codex", "codex") {
		t.Fatalf("expected codex to be sent, got %q", calls.sent)
	}
	if calls.sidepanelCommand != "kitmux sidepanel" {
		t.Fatalf("expected sidepanel split, got %q", calls.sidepanelCommand)
	}
	if calls.sidepanelRatio != 30 {
		t.Fatalf("expected 30%% sidepanel split, got %d", calls.sidepanelRatio)
	}
}

func TestLaunchAgentAutoSidepanelSkipsWhenNarrow(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "auto")
	t.Setenv("KITMUX_AGENT_SIDEPANEL_MIN_WIDTH", "160")

	calls := stubAgentLaunch(t, 120, nil)

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"})

	if calls.sent != trackedCommand("codex", "codex") {
		t.Fatalf("expected codex to be sent, got %q", calls.sent)
	}
	if calls.sidepanelCommand != "" {
		t.Fatalf("expected no sidepanel split, got %q", calls.sidepanelCommand)
	}
}

func TestLaunchAgentAlwaysSidepanelIgnoresWidthError(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "always")

	calls := stubAgentLaunch(t, 0, errors.New("no tmux"))

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"})

	if calls.sidepanelCommand != "kitmux sidepanel" {
		t.Fatalf("expected sidepanel split, got %q", calls.sidepanelCommand)
	}
}

func TestLaunchAgentOffNeverStartsSidepanel(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "off")

	calls := stubAgentLaunch(t, 240, nil)

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"})

	if calls.sidepanelCommand != "" {
		t.Fatalf("expected no sidepanel split, got %q", calls.sidepanelCommand)
	}
}

func TestLaunchAgentExplicitSplitDoesNotStartSidepanel(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "always")

	calls := stubAgentLaunch(t, 240, nil)

	m := New(ModeAgents)
	m.launchAgent(messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "split"})

	if calls.split != trackedCommand("codex", "codex") {
		t.Fatalf("expected explicit split command codex, got %q", calls.split)
	}
	if calls.sidepanelCommand != "" {
		t.Fatalf("expected no sidepanel split, got %q", calls.sidepanelCommand)
	}
}

func TestSidepanelEditorResultKeepsPaneAlive(t *testing.T) {
	m := New(ModeSidepanel)

	_, cmd, handled := m.handleOpenLocalEditor(messages.OpenLocalEditorMsg{})
	if !handled {
		t.Fatal("expected editor result to be handled")
	}
	if cmd != nil {
		t.Fatal("expected sidepanel editor result to keep pane alive")
	}
}

func TestSidepanelEscReturnsFromAuxView(t *testing.T) {
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

func TestSidepanelEscCancelsLaunchPickerWithoutQuitting(t *testing.T) {
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
		t.Fatal("expected sidepanel picker to be cancelled")
	}
	if updated.view != viewSidepanel {
		t.Fatalf("expected viewSidepanel, got %v", updated.view)
	}
}

func TestSidepanelBackToSessionsReturnsToSidepanel(t *testing.T) {
	m := New(ModeSidepanel)
	m.view = viewWindows

	model, cmd, handled := m.dispatchNavigation(messages.BackToSessionsMsg{})
	updated := model.(Model)
	if !handled {
		t.Fatal("expected back navigation to be handled")
	}
	if updated.view != viewSidepanel {
		t.Fatalf("expected viewSidepanel, got %v", updated.view)
	}
	if cmd == nil {
		t.Fatal("expected sidepanel init command")
	}
}

func TestSidepanelSwitchToSessionsInitializesSessions(t *testing.T) {
	m := New(ModeSidepanel)
	m.view = viewSidepanel

	model, cmd, handled := m.handleSwitchView(messages.SwitchViewMsg{View: "sessions"})
	updated := model.(Model)
	if !handled {
		t.Fatal("expected switch view to be handled")
	}
	if updated.view != viewSessions {
		t.Fatalf("expected viewSessions, got %v", updated.view)
	}
	if cmd == nil {
		t.Fatal("expected sessions init command")
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

func trackedCommand(agentID, command string) string {
	return agentenv.WrapTmuxCommand(agentID, "", command, false)
}

func TestSidepanelLaunchAgentCreatesWindowAndSplitWhenWide(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "auto")
	t.Setenv("KITMUX_AGENT_SIDEPANEL_MIN_WIDTH", "160")

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
	if calls.windowCommand != trackedCommand("codex", "codex") {
		t.Fatalf("expected codex command, got %q", calls.windowCommand)
	}
	if calls.sidepanelCommand != "kitmux sidepanel" {
		t.Fatalf("expected sidepanel split, got %q", calls.sidepanelCommand)
	}
	if calls.sidepanelRatio != 30 {
		t.Fatalf("expected 30%% sidepanel split, got %d", calls.sidepanelRatio)
	}
}

func TestSidepanelLaunchAgentSkipsSplitWhenNarrow(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "auto")
	t.Setenv("KITMUX_AGENT_SIDEPANEL_MIN_WIDTH", "160")

	calls := stubAgentLaunch(t, 100, nil)
	m := New(ModeSidepanel)
	_ = m.launchSidepanelAgent(messages.LaunchSidepanelAgentMsg{AgentID: "opencode", ModeID: "default", Dir: "/tmp/repo"})()
	if calls.windowCommand != trackedCommand("opencode", "opencode") {
		t.Fatalf("expected opencode command, got %q", calls.windowCommand)
	}
	if calls.sidepanelCommand != "" {
		t.Fatalf("expected no sidepanel split, got %q", calls.sidepanelCommand)
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
