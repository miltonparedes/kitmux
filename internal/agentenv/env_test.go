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

func TestWrapTmuxCommandDefaultsCodexThreadColorTerm(t *testing.T) {
	got := WrapTmuxCommand("codex", "codex-app", "codex", true)
	for _, want := range []string{
		`COLORTERM="${COLORTERM:-truecolor}"`,
		"FORCE_COLOR='3'",
		"CLICOLOR='1'",
		"CLICOLOR_FORCE='1'",
		"KITMUX_AGENT_ID='codex'",
		"KITMUX_TMUX_THREAD=1",
		"export COLORTERM FORCE_COLOR CLICOLOR CLICOLOR_FORCE KITMUX_AGENT_ID KITMUX_TMUX_SESSION KITMUX_TMUX_PANE KITMUX_TMUX_THREAD",
		`tmux list-clients -t "$KITMUX_TMUX_SESSION"`,
		"exec codex",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("WrapTmuxCommand() missing %q in %q", want, got)
		}
	}
}

func TestWrapRegisteredTmuxCommandExportsCodexThreadColorTerm(t *testing.T) {
	got := WrapRegisteredTmuxCommand("codex", "codex-app", "codex", true, "/tmp/kitmux")
	for _, want := range []string{
		`COLORTERM="${COLORTERM:-truecolor}"`,
		"FORCE_COLOR='3'",
		"CLICOLOR='1'",
		"CLICOLOR_FORCE='1'",
		"export COLORTERM FORCE_COLOR CLICOLOR CLICOLOR_FORCE KITMUX_AGENT_ID KITMUX_TMUX_SESSION KITMUX_TMUX_PANE KITMUX_TMUX_THREAD",
		`tmux list-clients -t "$KITMUX_TMUX_SESSION"`,
		"exec codex",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("WrapRegisteredTmuxCommand() missing %q in %q", want, got)
		}
	}
}

func TestColorTermDefaultOnlyAppliesToCodexThreads(t *testing.T) {
	for name, got := range map[string]string{
		"droid thread":     WrapTmuxCommand("droid", "droid-app", "droid", true),
		"codex non-thread": WrapTmuxCommand("codex", "codex-app", "codex", false),
	} {
		if strings.Contains(got, "COLORTERM") || strings.Contains(got, "FORCE_COLOR") {
			t.Fatalf("%s should not set color defaults: %q", name, got)
		}
	}
}

func TestWaitForClientOnlyAppliesToCodexThreads(t *testing.T) {
	for name, got := range map[string]string{
		"droid thread":     WrapTmuxCommand("droid", "droid-app", "droid", true),
		"codex non-thread": WrapTmuxCommand("codex", "codex-app", "codex", false),
	} {
		if strings.Contains(got, "list-clients") {
			t.Fatalf("%s should not wait for client: %q", name, got)
		}
	}
}

func TestWrapTmuxCommandPreservesCodexResumeCommand(t *testing.T) {
	got := WrapTmuxCommand("codex", "codex-app", "codex resume 'abc'", true)
	want := "exec codex resume 'abc'"
	if !strings.Contains(got, want) {
		t.Fatalf("WrapTmuxCommand() missing %q in %q", want, got)
	}
}

func TestWrapTmuxCommandDoesNotOverrideCodexStatusLineColors(t *testing.T) {
	got := WrapTmuxCommand("codex", "codex-app", "codex", true)
	if strings.Contains(got, "tui.status_line_use_colors") {
		t.Fatalf("WrapTmuxCommand() should not override status line colors: %q", got)
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

func TestWithTrackingEnvDefaultsCodexThreadColors(t *testing.T) {
	got := WithTrackingEnv([]string{
		"PATH=/bin",
		"COLORTERM=24bit",
		"FORCE_COLOR=0",
		"CLICOLOR=0",
		"CLICOLOR_FORCE=0",
	}, "codex", "work", "%2", true)
	want := []string{
		"PATH=/bin",
		"COLORTERM=24bit",
		"FORCE_COLOR=3",
		"CLICOLOR=1",
		"CLICOLOR_FORCE=1",
		"KITMUX_AGENT_ID=codex",
		"KITMUX_TMUX_SESSION=work",
		"KITMUX_TMUX_PANE=%2",
		"KITMUX_TMUX_THREAD=1",
	}
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
