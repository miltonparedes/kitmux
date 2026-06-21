package agenthooks

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/agenttrack"
)

func TestRunStateEventUpdatesPaneForAnyTmuxPane(t *testing.T) {
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	err := RunStateEvent(StateEvent{State: stateWorking}, nil, StateOps{
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

func TestRunAgentEventPersistsSessionIDFromHookPayload(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "claude")
	t.Setenv("KITMUX_TMUX_SESSION", "claude-app")
	t.Setenv("KITMUX_TMUX_PANE", "%3")
	t.Setenv("KITMUX_TMUX_THREAD", "1")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	payload := `{"hook_event_name":"SessionStart","session_id":"33333333-3333-4333-8333-333333333333"}`

	err := RunAgentEvent(AgentEvent{Agent: "claude", StdinJSON: true}, strings.NewReader(payload), nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "Claude · app", nil },
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		EmitBell:              func(_ io.Writer) error { return nil },
		StartSpinner:          func(SpinnerTarget) error { return nil },
		RefreshSessionClients: func(string) {},
		Now:                   func() time.Time { return time.UnixMilli(999) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentSessionIDOption] != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("pane session id = %q", paneOptions[agentSessionIDOption])
	}
	if sessionOptions[agentSessionIDOption] != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("session id = %q", sessionOptions[agentSessionIDOption])
	}
}

func TestRunAgentEventPersistsDroidOpaqueSessionIDFromHookPayload(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "%3")
	t.Setenv("KITMUX_TMUX_THREAD", "1")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	payload := `{"hook_event_name":"SessionStart","session_id":"abc123","transcript_path":"/Users/me/.factory/projects/app/33333333-3333-4333-8333-333333333333.jsonl"}`

	err := RunAgentEvent(AgentEvent{Agent: "droid", StdinJSON: true}, strings.NewReader(payload), nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "Droid · app", nil },
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		EmitBell:              func(_ io.Writer) error { return nil },
		StartSpinner:          func(SpinnerTarget) error { return nil },
		RefreshSessionClients: func(string) {},
		Now:                   func() time.Time { return time.UnixMilli(999) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentSessionIDOption] != "abc123" {
		t.Fatalf("pane session id = %q", paneOptions[agentSessionIDOption])
	}
	if sessionOptions[agentSessionIDOption] != "abc123" {
		t.Fatalf("session id = %q", sessionOptions[agentSessionIDOption])
	}
}

func TestRunAgentEventIgnoresDroidChildSessionEvents(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "%3")
	t.Setenv("KITMUX_TMUX_THREAD", "1")

	root := t.TempDir()
	childID := "22222222-2222-4222-8222-222222222222"
	parentID := "11111111-1111-4111-8111-111111111111"
	childPath := filepath.Join(root, ".factory", "sessions", "-repo-app", childID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(childPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		childPath,
		[]byte(`{"type":"session_start","id":"`+childID+`","callingSessionId":"`+parentID+`"}`+"\n"),
		0o600,
	); err != nil {
		t.Fatalf("write child session: %v", err)
	}

	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	spinnerStarted := false
	payload := `{"hook_event_name":"Stop","session_id":"` + childID + `","transcript_path":"` + childPath + `"}`
	err := RunAgentEvent(AgentEvent{Agent: "droid", StdinJSON: true}, strings.NewReader(payload), nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "⠹ hooks", nil },
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		StartSpinner: func(SpinnerTarget) error {
			spinnerStarted = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if len(paneOptions) != 0 || len(sessionOptions) != 0 || spinnerStarted {
		t.Fatalf("child event wrote pane=%#v session=%#v spinner=%t", paneOptions, sessionOptions, spinnerStarted)
	}
}

func TestRunAgentEventIgnoresDroidMismatchedNestedSessionID(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "%3")
	t.Setenv("KITMUX_TMUX_THREAD", "1")

	parentID := "11111111-1111-4111-8111-111111111111"
	nestedID := "22222222-2222-4222-8222-222222222222"
	root := t.TempDir()
	childPath := filepath.Join(root, ".factory", "sessions", "-repo-app", nestedID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(childPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		childPath,
		[]byte(`{"type":"session_start","id":"`+nestedID+`","callingSessionId":"`+parentID+`"}`+"\n"),
		0o600,
	); err != nil {
		t.Fatalf("write child session: %v", err)
	}
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	payload := `{"hook_event_name":"SessionStart","session_id":"` + nestedID + `","source":"startup","transcript_path":"` + childPath + `"}`
	err := RunAgentEvent(AgentEvent{Agent: "droid", StdinJSON: true}, strings.NewReader(payload), nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "hooks", nil },
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		ShowSessionOption: func(_, option string) (string, error) {
			if option == agentSessionIDOption {
				return parentID, nil
			}
			return "", nil
		},
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if len(paneOptions) != 0 || len(sessionOptions) != 0 {
		t.Fatalf("nested event wrote pane=%#v session=%#v", paneOptions, sessionOptions)
	}
}

func TestRunAgentEventAcceptsDroidMainSessionRestartOnStartup(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "%3")
	t.Setenv("KITMUX_TMUX_THREAD", "1")

	parentID := "11111111-1111-4111-8111-111111111111"
	newMainID := "33333333-3333-4333-8333-333333333333"
	sessionOptions := make(map[string]string)
	payload := `{"hook_event_name":"SessionStart","session_id":"` + newMainID + `","source":"startup"}`
	err := RunAgentEvent(AgentEvent{Agent: "droid", StdinJSON: true}, strings.NewReader(payload), nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "hooks", nil },
		SetPaneOption:    func(_, _, _ string) error { return nil },
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		ShowSessionOption: func(_, option string) (string, error) {
			if option == agentSessionIDOption {
				return parentID, nil
			}
			return "", nil
		},
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if sessionOptions[agentSessionIDOption] != newMainID {
		t.Fatalf("session id = %q, want %q", sessionOptions[agentSessionIDOption], newMainID)
	}
}

func TestRunAgentEventAcceptsDroidSessionIDWhenThreadHasNoSessionID(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "%3")
	t.Setenv("KITMUX_TMUX_THREAD", "1")

	sessionOptions := make(map[string]string)
	sessionID := "11111111-1111-4111-8111-111111111111"
	payload := `{"hook_event_name":"SessionStart","session_id":"` + sessionID + `","source":"startup"}`
	err := RunAgentEvent(AgentEvent{Agent: "droid", StdinJSON: true}, strings.NewReader(payload), nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "hooks", nil },
		SetPaneOption:    func(_, _, _ string) error { return nil },
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		ShowSessionOption: func(_, option string) (string, error) {
			if option == agentSessionIDOption {
				return "", nil
			}
			return "", nil
		},
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if sessionOptions[agentSessionIDOption] != sessionID {
		t.Fatalf("session id = %q", sessionOptions[agentSessionIDOption])
	}
}

func TestRunAgentEventKeepsTrackedThreadsIsolated(t *testing.T) {
	paneOptions := map[string]map[string]string{}
	sessionOptions := map[string]map[string]string{}
	var refreshed []string
	var spinners []SpinnerTarget
	ops := StateOps{
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

func TestRunAgentEventIgnoresDifferentTrackedAgent(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	t.Setenv("KITMUX_TMUX_THREAD", "1")

	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	err := RunAgentEvent(AgentEvent{Agent: "cursor", Event: "pre-tool-use"}, nil, nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "hooks", nil },
		SetPaneOption: func(_, option, value string) error {
			paneOptions[option] = value
			return nil
		},
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if len(paneOptions) != 0 || len(sessionOptions) != 0 {
		t.Fatalf("cross-agent event wrote pane=%#v session=%#v", paneOptions, sessionOptions)
	}
}

func TestRunAgentEventDoesNotMergeMismatchedAncestorAgent(t *testing.T) {
	originalResolve := resolveAncestorContext
	t.Cleanup(func() { resolveAncestorContext = originalResolve })
	resolveAncestorContext = func(int) (agenttrack.Context, bool) {
		return agenttrack.Context{
			AgentID:     "droid",
			SessionName: "droid-thread",
			PaneID:      "%7",
			Thread:      true,
		}, true
	}
	t.Setenv("KITMUX_AGENT_ID", "cursor")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	t.Setenv("KITMUX_TMUX_SESSION", "")
	t.Setenv("KITMUX_TMUX_THREAD", "")

	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	err := RunAgentEvent(AgentEvent{Agent: "cursor", Event: "pre-tool-use"}, nil, nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "cursor", nil },
		SetPaneOption: func(target, option, value string) error {
			if target != "%1" {
				t.Fatalf("pane target = %q", target)
			}
			paneOptions[option] = value
			return nil
		},
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != stateWorking {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if len(sessionOptions) != 0 {
		t.Fatalf("mismatched ancestor session was merged: %#v", sessionOptions)
	}
}

func TestRunAgentEventMergesMissingPaneFromRegistry(t *testing.T) {
	originalResolve := resolveAncestorContext
	t.Cleanup(func() { resolveAncestorContext = originalResolve })
	resolveAncestorContext = func(int) (agenttrack.Context, bool) {
		return agenttrack.Context{
			AgentID:     "droid",
			SessionName: "droid-app",
			PaneID:      "%9",
			Thread:      true,
		}, true
	}

	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "")
	t.Setenv("KITMUX_TMUX_THREAD", "1")
	paneOptions := make(map[string]string)
	sessionOptions := make(map[string]string)
	var spinner SpinnerTarget
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "Droid app", nil },
		SetPaneOption: func(target, option, value string) error {
			if target != "%9" {
				t.Fatalf("pane target = %q", target)
			}
			paneOptions[option] = value
			return nil
		},
		SetSessionOption: func(target, option, value string) error {
			if target != "droid-app" {
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
		Now:                   func() time.Time { return time.UnixMilli(42) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != stateWorking || sessionOptions[agentStateOption] != stateWorking {
		t.Fatalf("pane=%#v session=%#v", paneOptions, sessionOptions)
	}
	if spinner.PaneID != "%9" || spinner.SessionName != "droid-app" || spinner.Token != "42" {
		t.Fatalf("spinner = %#v", spinner)
	}
}

func TestRunAgentEventDoesNotLeaveStaticSpinnerWithoutPane(t *testing.T) {
	originalResolve := resolveAncestorContext
	t.Cleanup(func() { resolveAncestorContext = originalResolve })
	resolveAncestorContext = func(int) (agenttrack.Context, bool) {
		return agenttrack.Context{}, false
	}

	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_SESSION", "droid-app")
	t.Setenv("KITMUX_TMUX_PANE", "")
	t.Setenv("KITMUX_TMUX_THREAD", "1")
	sessionOptions := make(map[string]string)
	spinnerStarted := false
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
		CurrentPaneTitle: func() (string, error) { return "Droid app", nil },
		SetSessionOption: func(_, option, value string) error {
			sessionOptions[option] = value
			return nil
		},
		EmitBell:              func(_ io.Writer) error { return nil },
		StartSpinner:          func(SpinnerTarget) error { spinnerStarted = true; return nil },
		RefreshSessionClients: func(string) {},
		Now:                   func() time.Time { return time.UnixMilli(43) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if sessionOptions[agentTitlePrefixOption] != "" {
		t.Fatalf("static spinner prefix = %q", sessionOptions[agentTitlePrefixOption])
	}
	if spinnerStarted {
		t.Fatal("spinner should not start without a pane")
	}
}

func TestRunAgentEventDerivesPermissionAndDetailFromHookJSON(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "codex")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	var bells int
	input := `{"hook_event_name":"PermissionRequest","tool_name":"Bash","tool_input":{"description":"Run tests"}}`
	err := RunAgentEvent(AgentEvent{Agent: "codex", Event: "permission-request", StdinJSON: true}, strings.NewReader(input), nil, StateOps{
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
		EmitBell:                func(_ io.Writer) error { bells++; return nil },
		StartSpinner:            func(SpinnerTarget) error { return nil },
		Now:                     func() time.Time { return time.UnixMilli(99) },
	})
	if err != nil {
		t.Fatalf("RunAgentEvent() error = %v", err)
	}
	if paneOptions[agentStateOption] != statePermission || paneOptions[agentDetailOption] != "Bash" || paneOptions[agentTitlePrefixOption] != "!" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if bells != 1 {
		t.Fatalf("bells = %d, want 1", bells)
	}
}

func TestRunAgentEventPreToolAskUserIsInputNoSpinner(t *testing.T) {
	clearTrackingEnv(t)
	t.Setenv("KITMUX_AGENT_ID", "droid")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	spinnerStarted := false
	input := `{"hook_event_name":"PreToolUse","tool_name":"AskUser"}`
	err := RunAgentEvent(AgentEvent{Agent: "droid", Event: "pre-tool-use", StdinJSON: true}, strings.NewReader(input), nil, StateOps{
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
	if paneOptions[agentStateOption] != stateInput || paneOptions[agentTitlePrefixOption] != "⮞" {
		t.Fatalf("paneOptions = %#v", paneOptions)
	}
	if spinnerStarted {
		t.Fatalf("spinner must not start for an attention state")
	}
}

func TestRunAgentEventMapsCursorPromptSubmitAndSubagentStop(t *testing.T) {
	tests := []struct {
		name      string
		eventName string
		wantState string
	}{
		{name: "before submit prompt", eventName: "beforeSubmitPrompt", wantState: stateWorking},
		{name: "subagent stop", eventName: "subagentStop", wantState: stateIdle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearTrackingEnv(t)
			t.Setenv("KITMUX_AGENT_ID", "cursor")
			t.Setenv("KITMUX_TMUX_PANE", "%1")
			paneOptions := make(map[string]string)
			err := RunAgentEvent(AgentEvent{Agent: "cursor", StdinJSON: true}, strings.NewReader(`{"hook_event_name":"`+tt.eventName+`","chat_id":"44444444-4444-4444-8444-444444444444"}`), nil, StateOps{
				CurrentPaneTitle: func() (string, error) { return "⌬ cursor task", nil },
				SetPaneOption: func(_, option, value string) error {
					paneOptions[option] = value
					return nil
				},
			})
			if err != nil {
				t.Fatalf("RunAgentEvent() error = %v", err)
			}
			if paneOptions[agentStateOption] != tt.wantState {
				t.Fatalf("state = %q, want %q", paneOptions[agentStateOption], tt.wantState)
			}
			if paneOptions[agentSessionIDOption] != "44444444-4444-4444-8444-444444444444" {
				t.Fatalf("session id = %q", paneOptions[agentSessionIDOption])
			}
		})
	}
}

func TestDeriveStatePostToolAskUserIsWorking(t *testing.T) {
	state := deriveState(stateWorking, "post-tool-use", hookInput{ToolName: "AskUser"})
	if state != stateWorking {
		t.Fatalf("post-tool AskUser state = %q, want working", state)
	}
}

func TestDeriveStateNamespacedAskUserIsInput(t *testing.T) {
	state := deriveState(stateWorking, "pre-tool-use", hookInput{ToolName: "mcp__factory__AskUser"})
	if state != stateInput {
		t.Fatalf("namespaced AskUser state = %q, want input", state)
	}
}

func TestNotificationStateCompletedIsIdle(t *testing.T) {
	state := notificationState(hookInput{Message: "Task completed successfully"})
	if state != stateIdle {
		t.Fatalf("completed notification state = %q, want idle", state)
	}
}

func TestNotificationStateNegativeCompletionPhrasesStayInput(t *testing.T) {
	for _, message := range []string{"Task not completed", "Work unfinished", "Task isn't finished"} {
		state := notificationState(hookInput{Message: message})
		if state != stateInput {
			t.Fatalf("message %q state = %q, want input", message, state)
		}
	}
}

func TestDeriveBellForAttentionEvents(t *testing.T) {
	bellEvents := []string{
		"notification",
		"permission-request",
		"permission.asked",
		"permission-denied",
		"elicitation",
		"stop",
		"stop-failure",
		"session.idle",
		"session.error",
	}
	for _, event := range bellEvents {
		if !deriveBell(false, event) {
			t.Fatalf("deriveBell(%q) = false, want true", event)
		}
	}
	for _, event := range []string{"session-start", "user-prompt-submit", "pre-tool-use", "post-tool-use"} {
		if deriveBell(false, event) {
			t.Fatalf("deriveBell(%q) = true, want false", event)
		}
	}
	if !deriveBell(true, "pre-tool-use") {
		t.Fatal("explicit bell should be preserved")
	}
}

func TestRunAgentEventAddsConsistentSpinnerAndTrimsNativeLoaderForCodex(t *testing.T) {
	t.Setenv("KITMUX_AGENT_ID", "codex")
	t.Setenv("KITMUX_TMUX_PANE", "%1")
	paneOptions := make(map[string]string)
	var spinner SpinnerTarget
	err := RunAgentEvent(AgentEvent{Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
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
		{stateInput, "Android app", "⮞"},
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
	clearTrackingEnv(t)
	original := resolveAncestorContext
	t.Cleanup(func() {
		resolveAncestorContext = original
	})
	resolveAncestorContext = func(int) (agenttrack.Context, bool) {
		return agenttrack.Context{}, false
	}

	var paneWrites int
	var sessionWrites int
	spinnerStarted := false
	err := RunAgentEvent(AgentEvent{Agent: "droid", Event: "pre-tool-use", State: stateWorking}, nil, nil, StateOps{
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
	clearTrackingEnv(t)
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

func clearTrackingEnv(t *testing.T) {
	t.Helper()
	t.Setenv("KITMUX_AGENT_ID", "")
	t.Setenv("KITMUX_TMUX_SESSION", "")
	t.Setenv("KITMUX_TMUX_PANE", "")
	t.Setenv("KITMUX_TMUX_THREAD", "")
}
