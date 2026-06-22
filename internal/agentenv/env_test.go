package agentenv

import (
	"strings"
	"testing"
)

func TestWrapTmuxCommandInjectsTrackingEnv(t *testing.T) {
	got := WrapTmuxCommand("droid", "droid-app", "droid", true)
	for _, want := range []string{
		"KITMUX_AGENT_ID='droid'",
		"KITMUX_TMUX_SESSION='droid-app'",
		`KITMUX_TMUX_PANE="${TMUX_PANE:-$(tmux display-message -p '#{pane_id}' 2>/dev/null)}"`,
		"KITMUX_TMUX_THREAD=1",
		"exec droid",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("WrapTmuxCommand() missing %q in %q", want, got)
		}
	}
}

func TestWrapRegisteredTmuxCommandRegistersAgentPID(t *testing.T) {
	got := WrapRegisteredTmuxCommand("droid", "droid-app", "droid", true, "/tmp/kitmux")
	for _, want := range []string{
		"export KITMUX_AGENT_ID KITMUX_TMUX_SESSION KITMUX_TMUX_PANE KITMUX_TMUX_THREAD",
		"'/tmp/kitmux' hook agent-register --pid \"$$\" --agent \"$KITMUX_AGENT_ID\"",
		"--session \"$KITMUX_TMUX_SESSION\" --pane \"$KITMUX_TMUX_PANE\" --thread \"$KITMUX_TMUX_THREAD\"",
		"exec droid",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("WrapRegisteredTmuxCommand() missing %q in %q", want, got)
		}
	}
}

func TestWithTrackingEnvReplacesExistingValues(t *testing.T) {
	got := WithTrackingEnv([]string{
		"PATH=/bin",
		"KITMUX_AGENT_ID=old",
		"KITMUX_TMUX_SESSION=old",
		"KITMUX_TMUX_PANE=%1",
		"KITMUX_TMUX_THREAD=1",
	}, "codex", "work", "%2", false)
	want := []string{"PATH=/bin", "KITMUX_AGENT_ID=codex", "KITMUX_TMUX_SESSION=work", "KITMUX_TMUX_PANE=%2"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("WithTrackingEnv() = %#v", got)
	}
}

func TestWrapHookCommandOnlyUsesExplicitTrackingEnv(t *testing.T) {
	got := WrapHookCommand("droid", "kitmux hook agent-event --agent droid")
	for _, want := range []string{
		"KITMUX_AGENT_ID='droid'",
		`KITMUX_TMUX_SESSION="${KITMUX_TMUX_SESSION:-}"`,
		`KITMUX_TMUX_PANE="${KITMUX_TMUX_PANE:-}"`,
		`KITMUX_TMUX_THREAD="${KITMUX_TMUX_THREAD:-}"`,
		"kitmux hook agent-event --agent droid",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("WrapHookCommand() missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "tmux display-message") || strings.Contains(got, "${TMUX_PANE") {
		t.Fatalf("WrapHookCommand() should not infer ambient tmux context: %q", got)
	}
}
