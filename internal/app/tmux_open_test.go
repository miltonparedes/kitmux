package app

import (
	"io"
	"reflect"
	"testing"
)

func TestOpenTmuxTargetUsesSwitchClientInsideTmux(t *testing.T) {
	calls := stubTmuxOpen(t, true)

	cmd := openTmuxTargetCommand{target: "droid-app", session: "droid-app"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !reflect.DeepEqual(calls.values, []string{"switch:droid-app"}) {
		t.Fatalf("calls = %#v", calls.values)
	}
}

func TestOpenTmuxTargetAttachesSessionOutsideTmux(t *testing.T) {
	calls := stubTmuxOpen(t, false)

	cmd := openTmuxTargetCommand{target: "droid-app", session: "droid-app"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"attach:droid-app"}
	if !reflect.DeepEqual(calls.values, want) {
		t.Fatalf("calls = %#v, want %#v", calls.values, want)
	}
}

func TestOpenTmuxPaneSelectsTargetBeforeAttachOutsideTmux(t *testing.T) {
	calls := stubTmuxOpen(t, false)

	cmd := openTmuxTargetCommand{target: "work:1.2", session: sessionFromTarget("work:1.2"), pane: true}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"select-window:work:1.2", "select-pane:work:1.2", "attach:work"}
	if !reflect.DeepEqual(calls.values, want) {
		t.Fatalf("calls = %#v, want %#v", calls.values, want)
	}
}

func TestOpenTmuxPaneSelectsTargetBeforeSwitchInsideTmux(t *testing.T) {
	calls := stubTmuxOpen(t, true)

	cmd := openTmuxTargetCommand{target: "work:1.2", session: sessionFromTarget("work:1.2"), pane: true}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"select-window:work:1.2", "select-pane:work:1.2", "switch:work"}
	if !reflect.DeepEqual(calls.values, want) {
		t.Fatalf("calls = %#v, want %#v", calls.values, want)
	}
}

func TestSessionFromTarget(t *testing.T) {
	tests := map[string]string{
		"droid-app": "droid-app",
		"work:1.2":  "work",
		"work.1":    "work",
	}
	for target, want := range tests {
		if got := sessionFromTarget(target); got != want {
			t.Fatalf("sessionFromTarget(%q) = %q, want %q", target, got, want)
		}
	}
}

type tmuxOpenCalls struct {
	values []string
}

func stubTmuxOpen(t *testing.T, inTmux bool) *tmuxOpenCalls {
	t.Helper()
	original := tmuxOpenOps
	t.Cleanup(func() {
		tmuxOpenOps = original
	})

	calls := &tmuxOpenCalls{}
	tmuxOpenOps = openTmuxOps{
		SwitchClient: func(target string) error {
			calls.values = append(calls.values, "switch:"+target)
			return nil
		},
		SelectWindow: func(target string) error {
			calls.values = append(calls.values, "select-window:"+target)
			return nil
		},
		SelectPane: func(target string) error {
			calls.values = append(calls.values, "select-pane:"+target)
			return nil
		},
		Attach: func(session string, _ io.Reader, _, _ io.Writer) error {
			calls.values = append(calls.values, "attach:"+session)
			return nil
		},
		InTmux: func() bool {
			return inTmux
		},
	}
	return calls
}
