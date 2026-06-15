package threads

import (
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestBuildRowsKeepsHeadlessDetailedAndSkipsDuplicatePane(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "droid-app", Path: "/repo/app", Activity: 10, Thread: true, AgentID: "droid", AgentState: "idle"},
		{Name: "work", Path: "/repo/app"},
	}
	panes := []tmux.Pane{
		{SessionName: "droid-app", WindowIndex: 0, PaneIndex: 0, Command: "droid", Path: "/repo/app", Title: "feat/threads", AgentState: "working", AgentUpdated: time.Now().UnixMilli()},
		{SessionName: "work", WindowIndex: 1, PaneIndex: 2, Command: "codex", Path: "/repo/app", Title: "codex review", AgentState: "input"},
	}

	rows := buildRows(sessions, panes)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, rows = %#v", len(rows), rows)
	}
	if rows[0].Kind != RowHeadless || rows[0].AgentID != "droid" {
		t.Fatalf("headless row = %#v", rows[0])
	}
	if rows[0].Title != "feat/threads" || rows[0].AgentState != "working" || rows[0].AgentSymbol != "⛬" {
		t.Fatalf("headless title/state/symbol = %#v", rows[0])
	}
	if rows[1].Kind != RowEphemeral || rows[1].AgentID != "codex" {
		t.Fatalf("ephemeral row = %#v", rows[1])
	}
	if rows[1].AgentState != "input" {
		t.Fatalf("ephemeral state = %q", rows[1].AgentState)
	}
}

func TestRowLabelUsesStateIconAndAvoidsIdleSymbolDuplication(t *testing.T) {
	working := Row{AgentSymbol: "⛬", AgentState: "working", Title: "feat/threads"}
	if got := rowLabel(working, 0); got != "⠋ feat/threads" {
		t.Fatalf("working rowLabel = %q", got)
	}

	input := Row{AgentSymbol: "✳", AgentState: "input", Title: "fix title"}
	if got := rowLabel(input, 0); got != "? fix title" {
		t.Fatalf("input rowLabel = %q", got)
	}

	permission := Row{AgentSymbol: "✳", AgentState: "permission", Title: "needs approval"}
	if got := rowLabel(permission, 0); got != "! needs approval" {
		t.Fatalf("permission rowLabel = %q", got)
	}

	codexWorking := Row{AgentID: "codex", AgentSymbol: "›", AgentState: "working", Title: "⠹ kitmux"}
	if got := rowLabel(codexWorking, 0); got != "⠋ kitmux" {
		t.Fatalf("codex working rowLabel = %q", got)
	}

	droidWorkingWithNativePrefix := Row{AgentID: "droid", AgentSymbol: "⛬", AgentState: "working", Title: "⠂ ⛬ Android app"}
	if got := rowLabel(droidWorkingWithNativePrefix, 0); got != "⠋ Android app" {
		t.Fatalf("droid working rowLabel = %q", got)
	}

	idle := Row{AgentSymbol: "⛬", AgentState: "idle", Title: "⛬ Droid · app"}
	if got := rowLabel(idle, 0); got != "⛬ Droid · app" {
		t.Fatalf("idle rowLabel = %q", got)
	}
}
