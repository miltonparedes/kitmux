package agenthooks

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestRunStateEventUpdatesPaneForAnyTmuxPane(t *testing.T) {
	var paneState string
	var sessionState string
	err := RunStateEvent(StateEvent{State: "working"}, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{SessionName: "work"}, nil
		},
		SetCurrentPaneOption: func(_, value string) error {
			paneState = value
			return nil
		},
		SetCurrentSessionOption: func(_, value string) error {
			sessionState = value
			return nil
		},
		EmitBell: func(_ io.Writer) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunStateEvent() error = %v", err)
	}
	if paneState != "working" {
		t.Fatalf("paneState = %q", paneState)
	}
	if sessionState != "" {
		t.Fatalf("sessionState = %q", sessionState)
	}
}

func TestRunStateEventSyncsSessionForThread(t *testing.T) {
	var paneState string
	var sessionState string
	err := RunStateEvent(StateEvent{State: "input"}, nil, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{SessionName: "droid-app", Thread: true, AgentID: "droid"}, nil
		},
		SetCurrentPaneOption: func(_, value string) error {
			paneState = value
			return nil
		},
		SetCurrentSessionOption: func(_, value string) error {
			sessionState = value
			return nil
		},
		EmitBell: func(_ io.Writer) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunStateEvent() error = %v", err)
	}
	if paneState != "input" || sessionState != "input" {
		t.Fatalf("paneState=%q sessionState=%q", paneState, sessionState)
	}
}

func TestRunStateEventEmitsBellAndIgnoresTmuxErrors(t *testing.T) {
	var out bytes.Buffer
	err := RunStateEvent(StateEvent{State: "idle", Bell: true}, &out, StateOps{
		CurrentThreadContext: func() (tmux.ThreadContext, error) {
			return tmux.ThreadContext{}, fmt.Errorf("not in tmux")
		},
		SetCurrentPaneOption: func(_, _ string) error {
			return fmt.Errorf("not in tmux")
		},
		EmitBell: func(w io.Writer) error {
			_, err := w.Write([]byte("\a"))
			return err
		},
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
