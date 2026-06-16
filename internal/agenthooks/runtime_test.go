package agenthooks

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/agenttrack"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestRunStateEventUpdatesPaneForAnyTmuxPane(t *testing.T) {
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	err := RunStateEvent(StateEvent{State: stateWorking}, nil, StateOps{
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
	if paneOptions[agentStateOption] != stateWorking || paneOptions[agentUpdatedOption] != "1234" {
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
	err := RunAgentEvent(AgentEvent{Agent: "droid", Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
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
	if paneOptions[agentStateOption] != stateWorking || sessionOptions[agentStateOption] != stateWorking {
		t.Fatalf("paneOptions=%#v sessionOptions=%#v", paneOptions, sessionOptions)
	}
	if paneOptions[agentEventOption] != "pre-tool-use" || paneOptions[agentTitlePrefixOption] != SpinnerFrames[0] {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if spinner.PaneID != "%2" || spinner.SessionName != "droid-app" || spinner.Token != "5678" {
		t.Fatalf("spinner = %#v", spinner)
	}
}

func TestRunAgentEventKeepsTrackedThreadsIsolated(t *testing.T) {
	paneOptions := map[string]map[string]string{}
	sessionOptions := map[string]map[string]string{}
	var refreshed []string
	var spinners []SpinnerTarget
	ops := StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			t.Fatal("expected hook context to come only from explicit tracking env")
			return tmux.ThreadContext{}, nil
		},
		CurrentPaneTitle: func() (string, error) { return "feat/thread", nil },
		SetPaneOption: func(target, option, value string) error {
			if paneOptions[target] == nil {
				paneOptions[target] = map[string]string{}
			}
			paneOptions[target][option] = value
			return nil
		},
		SetSessionOption: func(target, option, value string) error {
			if sessionOptions[target] == nil {
				sessionOptions[target] = map[string]string{}
			}
			sessionOptions[target][option] = value
			return nil
		},
		EmitBell: func(_ io.Writer) error { return nil },
		StartSpinner: func(target SpinnerTarget) error {
			spinners = append(spinners, target)
			return nil
		},
		RefreshSessionClients: func(sessionName string) {
			refreshed = append(refreshed, sessionName)
		},
		Now: func() time.Time { return time.UnixMilli(777) },
	}

	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-one")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	t.Setenv("KITMUX_TMUX_THREAD", "1")
	if err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: stateWorking}, nil, nil, ops); err != nil {
		t.Fatalf("first RunAgentEvent() error = %v", err)
	}

	t.Setenv("KITMUX_TMUX_SESSION", "droid-two")
	t.Setenv("KITMUX_TMUX_PANE", "%2")
	if err := RunAgentEvent(AgentEvent{Event: "notification", State: stateInput}, nil, nil, ops); err != nil {
		t.Fatalf("second RunAgentEvent() error = %v", err)
	}

	if paneOptions["%1"][agentStateOption] != stateWorking || sessionOptions["droid-one"][agentStateOption] != stateWorking {
		t.Fatalf("thread one options leaked or missing: panes=%#v sessions=%#v", paneOptions, sessionOptions)
	}
	if paneOptions["%2"][agentStateOption] != stateInput || sessionOptions["droid-two"][agentStateOption] != stateInput {
		t.Fatalf("thread two options leaked or missing: panes=%#v sessions=%#v", paneOptions, sessionOptions)
	}
	if _, ok := paneOptions[""]; ok {
		t.Fatalf("unexpected current-pane write: %#v", paneOptions)
	}
	if len(spinners) != 1 || spinners[0].PaneID != "%1" || spinners[0].SessionName != "droid-one" {
		t.Fatalf("spinners = %#v", spinners)
	}
	if strings.Join(refreshed, "|") != "droid-one|droid-two" {
		t.Fatalf("refreshed = %#v", refreshed)
	}
}

func TestRunStateEventEmitsBellAndIgnoresTmuxErrors(t *testing.T) {
	var out bytes.Buffer
	err := RunStateEvent(StateEvent{State: stateIdle, Bell: true}, &out, StateOps{
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
	err := RunAgentEvent(AgentEvent{Agent: "codex", Event: "permission-request", State: stateWorking, StdinJSON: true}, strings.NewReader(input), nil, StateOps{
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
	if paneOptions[agentStateOption] != statePermission || paneOptions[agentDetailOption] != "Bash" || paneOptions[agentTitlePrefixOption] != "!" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
}

func TestRunAgentEventPreToolAskUserIsInputNoSpinner(t *testing.T) {
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	spinnerStarted := false
	input := `{"hook_event_name":"PreToolUse","tool_name":"AskUser"}`
	err := RunAgentEvent(AgentEvent{Agent: "droid", Event: "pre-tool-use", State: stateWorking, StdinJSON: true}, strings.NewReader(input), nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{SessionName: "work", PaneID: "%1"}, nil
		},
		CurrentPaneTitle: func() (string, error) { return "branch-title", nil },
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetCurrentPaneOption:    func(_, _ string) error { return nil },
		SetCurrentSessionOption: func(_, _ string) error { return nil },
		EmitBell:                func(_ io.Writer) error { return nil },
		StartSpinner:            func(SpinnerTarget) error { spinnerStarted = true; return nil },
		Now:                     func() time.Time { return time.UnixMilli(7) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != stateInput || paneOptions[agentTitlePrefixOption] != "?" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if spinnerStarted {
		t.Fatalf("spinner must not start for an attention state")
	}
}

func TestDeriveStatePostToolAskUserIsWorking(t *testing.T) {
	state := deriveState(stateWorking, "post-tool-use", hookInput{ToolName: "AskUser"})
	if state != stateWorking {
		t.Fatalf("post-tool AskUser state = %q, want working", state)
	}
}

func TestRunAgentEventAddsConsistentSpinnerAndTrimsNativeLoaderForCodex(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "codex")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	var spinner SpinnerTarget
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
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
	if paneOptions[agentStateOption] != stateWorking {
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
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
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
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
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
	if sessionOptions[agentStateOption] != stateWorking || sessionOptions[agentTitlePrefixOption] != SpinnerFrames[0] {
		t.Fatalf("sessionOptions = %#v", sessionOptions)
	}
	if spinner.SessionName != "droid-kitmux" || spinner.Token != "2026" {
		t.Fatalf("spinner = %#v", spinner)
	}
}

func TestTitlePrefixRevertsToAgentSymbolWhenIdle(t *testing.T) {
	cases := []struct {
		state string
		title string
		want  string
	}{
		{stateWorking, "Android app", SpinnerFrames[0]},
		{stateInput, "Android app", "?"},
		{statePermission, "Android app", "!"},
		{stateError, "Android app", "×"},
		{stateIdle, "⛬ Android app", "⛬"},
		{"idle", "⠹ Android app", "⛬"},
		{"idle", "⛬", ""},
	}
	for _, tc := range cases {
		if got := titlePrefix(tc.state, "droid", tc.title); got != tc.want {
			t.Fatalf("titlePrefix(%q, droid, %q) = %q, want %q", tc.state, tc.title, got, tc.want)
		}
	}
}

func TestRunAgentEventTargetsPaneFromEnvWithoutSessionSync(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "codex")
	t.Setenv("KITMUX_TMUX_SESSION", "work")
	t.Setenv("KITMUX_TMUX_PANE", "%42")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	err := RunAgentEvent(AgentEvent{Event: "permission-request", State: statePermission}, nil, nil, StateOps{
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
	if paneOptions[agentStateOption] != statePermission || paneOptions[agentTitlePrefixOption] != "!" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if len(sessionOptions) != 0 {
		t.Fatalf("sessionOptions = %#v", sessionOptions)
	}
}

func TestRunAgentEventWithoutTrackingEnvDoesNotTouchTmuxTargets(t *testing.T) {
	var paneWrites int
	var sessionWrites int
	spinnerStarted := false
	err := RunAgentEvent(AgentEvent{Agent: "droid", Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{SessionName: "unrelated", PaneID: "%9", Thread: true}, nil
		},
		CurrentPaneTitle: func() (string, error) {
			t.Fatal("expected no pane title lookup without explicit tracking env")
			return "", nil
		},
		SetPaneOption: func(_, _, _ string) error {
			paneWrites++
			return nil
		},
		SetCurrentPaneOption: func(_, _ string) error {
			paneWrites++
			return nil
		},
		SetSessionOption: func(_, _, _ string) error {
			sessionWrites++
			return nil
		},
		SetCurrentSessionOption: func(_, _ string) error {
			sessionWrites++
			return nil
		},
		EmitBell:     func(_ io.Writer) error { return nil },
		StartSpinner: func(SpinnerTarget) error { spinnerStarted = true; return nil },
		Now:          func() time.Time { return time.UnixMilli(11) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneWrites != 0 || sessionWrites != 0 || spinnerStarted {
		t.Fatalf("paneWrites=%d sessionWrites=%d spinnerStarted=%v", paneWrites, sessionWrites, spinnerStarted)
	}
}

func TestRunAgentEventResolvesTargetFromRegisteredAncestor(t *testing.T) {
	original := resolveAncestorContext
	t.Cleanup(func() {
		resolveAncestorContext = original
	})
	resolveAncestorContext = func(int) (agenttrack.Context, bool) {
		return agenttrack.Context{
			AgentID:     "droid",
			SessionName: "droid-thread",
			PaneID:      "%7",
			Thread:      true,
		}, true
	}

	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	var spinner SpinnerTarget
	err := RunAgentEvent(AgentEvent{Agent: "droid", Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			t.Fatal("expected registered process context, not ambient tmux lookup")
			return tmux.ThreadContext{}, nil
		},
		CurrentPaneTitle: func() (string, error) { return "registered thread", nil },
		SetPaneOption: func(target, option, value string) error {
			if target != "%7" {
				t.Fatalf("pane target = %q", target)
			}
			paneOptions[option] = value
			return nil
		},
		SetSessionOption: func(target, option, value string) error {
			if target != "droid-thread" {
				t.Fatalf("session target = %q", target)
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
		Now:                   func() time.Time { return time.UnixMilli(22) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != stateWorking || sessionOptions[agentStateOption] != stateWorking {
		t.Fatalf("paneOptions=%#v sessionOptions=%#v", paneOptions, sessionOptions)
	}
	if spinner.PaneID != "%7" || spinner.SessionName != "droid-thread" {
		t.Fatalf("spinner = %#v", spinner)
	}
}
