package agenthooks

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestRunStateEventUpdatesPaneForAnyTmuxPane(t *testing.T) {
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	err := RunStateEvent(StateEvent{State: "working"}, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{SessionName: "work", PaneID: "%1"}, nil
		},
		CurrentPaneTitle: func() (string, error) {
			return "feat/threads", nil
		},
		SetCurrentPaneOption: func(option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetCurrentSessionOption: func(option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		EmitBell:     func(_ io.Writer) error { return nil },
		StartSpinner: func(SpinnerTarget) error { return nil },
		Now:          func() time.Time { return time.UnixMilli(1234) },
	})
	if err != nil {
		t.Fatalf("RunStateEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != "working" || paneOptions[agentUpdatedOption] != "1234" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if sessionOptions[agentStateOption] != "" {
		t.Fatalf("sessionOptions = %#v", sessionOptions)
	}
}

func TestRunStateEventSyncsSessionForThread(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "%2")
	t.Setenv("KITMUX_TMUX_THREAD", "1")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	var spinner SpinnerTarget
	err := RunAgentEvent(AgentEvent{Agent: "droid", Event: "pre-tool-use", State: "working"}, nil, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{SessionName: "droid-app", PaneID: "%2", Thread: true, AgentID: "droid"}, nil
		},
		CurrentPaneTitle: func() (string, error) {
			return "feat/threads", nil
		},
		SetCurrentPaneOption: func(option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetCurrentSessionOption: func(option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		EmitBell: func(_ io.Writer) error { return nil },
		StartSpinner: func(target SpinnerTarget) error {
			spinner = target
			return nil
		},
		RefreshSessionClients: func(string) {},
		Now:                   func() time.Time { return time.UnixMilli(5678) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != "working" || sessionOptions[agentStateOption] != "working" {
		t.Fatalf("paneOptions=%#v sessionOptions=%#v", paneOptions, sessionOptions)
	}
	if paneOptions[agentEventOption] != "pre-tool-use" || paneOptions[agentTitlePrefixOption] != SpinnerFrames[0] {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if spinner.PaneID != "%2" || spinner.SessionName != "droid-app" || spinner.Token != "5678" {
		t.Fatalf("spinner = %#v", spinner)
	}
}

func TestRunStateEventEmitsBellAndIgnoresTmuxErrors(t *testing.T) {
	var out bytes.Buffer
	err := RunStateEvent(StateEvent{State: "idle", Bell: true}, &out, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{}, fmt.Errorf("not in tmux")
		},
		CurrentPaneTitle: func() (string, error) {
			return "", fmt.Errorf("not in tmux")
		},
		SetCurrentPaneOption: func(_, _ string) error {
			return fmt.Errorf("not in tmux")
		},
		EmitBell: func(w io.Writer) error {
			_, err := w.Write([]byte("\a"))
			return err
		},
		StartSpinner: func(SpinnerTarget) error { return nil },
		Now:          func() time.Time { return time.UnixMilli(1) },
	})
	if err != nil {
		t.Fatalf("RunStateEvent() error = %v", err)
	}
	if out.String() != "\a" {
		t.Fatalf("bell output = %q", out.String())
	}
}

func TestRunStateEventRejectsUnknownState(t *testing.T) {
	err := RunStateEvent(StateEvent{State: "paused"}, nil, StateOps{})
	if err == nil {
		t.Fatal("expected invalid state error")
	}
}

func TestRunAgentEventDerivesPermissionAndDetailFromHookJSON(t *testing.T) {
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	input := `{"hook_event_name":"PermissionRequest","tool_name":"Bash","tool_input":{"description":"Run tests"}}`
	err := RunAgentEvent(AgentEvent{Agent: "codex", Event: "permission-request", State: "working", StdinJSON: true}, strings.NewReader(input), nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{SessionName: "work", PaneID: "%1"}, nil
		},
		CurrentPaneTitle: func() (string, error) { return "branch-title", nil },
		SetCurrentPaneOption: func(option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetCurrentSessionOption: func(_, _ string) error { return nil },
		EmitBell:                func(_ io.Writer) error { return nil },
		StartSpinner:            func(SpinnerTarget) error { return nil },
		Now:                     func() time.Time { return time.UnixMilli(99) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != "permission" || paneOptions[agentDetailOption] != "Bash" || paneOptions[agentTitlePrefixOption] != "!" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
}

func TestRunAgentEventAddsConsistentSpinnerAndTrimsNativeLoaderForCodex(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "codex")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	var spinner SpinnerTarget
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: "working"}, nil, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{}, fmt.Errorf("not in tmux")
		},
		CurrentPaneTitle: func() (string, error) { return "⠹ › kitmux", nil },
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetCurrentPaneOption:    func(_, _ string) error { return nil },
		SetCurrentSessionOption: func(_, _ string) error { return nil },
		EmitBell:                func(_ io.Writer) error { return nil },
		StartSpinner: func(target SpinnerTarget) error {
			spinner = target
			return nil
		},
		Now: func() time.Time { return time.UnixMilli(4040) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != "working" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if paneOptions[agentTitlePrefixOption] != SpinnerFrames[0] || paneOptions[agentTitleDisplayOption] != "kitmux" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if spinner.PaneID != "%1" || spinner.Token != "4040" {
		t.Fatalf("spinner = %#v", spinner)
	}
}

func TestRunAgentEventReplacesDroidSymbolWithSpinnerInTitleDisplay(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: "working"}, nil, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{}, fmt.Errorf("not in tmux")
		},
		CurrentPaneTitle: func() (string, error) { return "⠂ ⛬ Android app", nil },
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetCurrentPaneOption:    func(_, _ string) error { return nil },
		SetCurrentSessionOption: func(_, _ string) error { return nil },
		EmitBell:                func(_ io.Writer) error { return nil },
		StartSpinner:            func(SpinnerTarget) error { return nil },
		Now:                     func() time.Time { return time.UnixMilli(5050) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentTitlePrefixOption] != SpinnerFrames[0] || paneOptions[agentTitleDisplayOption] != "Android app" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
}

func TestRunAgentEventTargetsSessionFromEnvWithoutTmuxPane(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-kitmux")
	t.Setenv("KITMUX_TMUX_THREAD", "1")
	sessionOptions := make(map[string]string)
	var spinner SpinnerTarget
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: "working"}, nil, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{}, fmt.Errorf("not in tmux")
		},
		CurrentPaneTitle: func() (string, error) { return "", fmt.Errorf("not in tmux") },
		SetCurrentPaneOption: func(_, _ string) error {
			t.Fatal("expected no implicit pane writes without KITMUX_TMUX_PANE")
			return nil
		},
		SetCurrentSessionOption: func(_, _ string) error {
			t.Fatal("expected targeted session writes")
			return nil
		},
		SetSessionOption: func(target, option, value string) error {
			if target != "droid-kitmux" {
				t.Fatalf("target = %q", target)
			}
			sessionOptions[option] = value
			return nil
		},
		EmitBell: func(_ io.Writer) error { return nil },
		StartSpinner: func(target SpinnerTarget) error {
			spinner = target
			return nil
		},
		RefreshSessionClients: func(string) {},
		Now:                   func() time.Time { return time.UnixMilli(2026) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if sessionOptions[agentStateOption] != "working" || sessionOptions[agentTitlePrefixOption] != SpinnerFrames[0] {
		t.Fatalf("sessionOptions = %#v", sessionOptions)
	}
	if spinner.SessionName != "droid-kitmux" || spinner.Token != "2026" {
		t.Fatalf("spinner = %#v", spinner)
	}
}

func TestRunAgentEventTargetsPaneFromEnvWithoutSessionSync(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "codex")
	t.Setenv("KITMUX_TMUX_SESSION", "work")
	t.Setenv("KITMUX_TMUX_PANE", "%42")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	err := RunAgentEvent(AgentEvent{Event: "permission-request", State: "permission"}, nil, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{}, fmt.Errorf("not in tmux")
		},
		CurrentPaneTitle: func() (string, error) { return "branch", nil },
		SetCurrentPaneOption: func(_, _ string) error {
			t.Fatal("expected targeted pane writes")
			return nil
		},
		SetPaneOption: func(target, option, value string) error {
			if target != "%42" {
				t.Fatalf("target = %q", target)
			}
			paneOptions[option] = value
			return nil
		},
		SetCurrentSessionOption: func(option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		EmitBell:     func(_ io.Writer) error { return nil },
		StartSpinner: func(SpinnerTarget) error { return nil },
		Now:          func() time.Time { return time.UnixMilli(3030) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != "permission" || paneOptions[agentTitlePrefixOption] != "!" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if len(sessionOptions) != 0 {
		t.Fatalf("sessionOptions = %#v", sessionOptions)
	}
}
