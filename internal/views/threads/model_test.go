package threads

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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

func TestRowIconAndTitleComposeStateGlyphWithoutDuplication(t *testing.T) {
	compose := func(row Row) string {
		return rowIcon(row, 0) + " " + rowTitle(row)
	}

	working := Row{AgentSymbol: "⛬", AgentState: "working", Title: "feat/threads"}
	if got := compose(working); got != "⠋ feat/threads" {
		t.Fatalf("working = %q", got)
	}

	input := Row{AgentSymbol: "✳", AgentState: "input", Title: "fix title"}
	if got := compose(input); got != "⮞ fix title" {
		t.Fatalf("input = %q", got)
	}

	permission := Row{AgentSymbol: "✳", AgentState: "permission", Title: "needs approval"}
	if got := compose(permission); got != "! needs approval" {
		t.Fatalf("permission = %q", got)
	}

	codexWorking := Row{AgentID: "codex", AgentSymbol: "›", AgentState: "working", Title: "⠹ kitmux"}
	if got := compose(codexWorking); got != "⠋ kitmux" {
		t.Fatalf("codex working = %q", got)
	}

	droidWorkingWithNativePrefix := Row{AgentID: "droid", AgentSymbol: "⛬", AgentState: "working", Title: "⠂ ⛬ Android app"}
	if got := compose(droidWorkingWithNativePrefix); got != "⠋ Android app" {
		t.Fatalf("droid working = %q", got)
	}

	idle := Row{AgentSymbol: "⛬", AgentState: "idle", Title: "⛬ Droid · app"}
	if got := compose(idle); got != "⛬ Droid · app" {
		t.Fatalf("idle = %q", got)
	}
}

func TestModelClampsScrollAfterEmptyEndKeyThenLoad(t *testing.T) {
	m := New()
	m.SetSize(80, 5)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m, _ = m.Update(loadedMsg{rows: []Row{{Kind: RowHeadless, SessionName: "droid-app", AgentName: "Droid"}}})

	if m.scroll != 0 || m.cursor != 0 {
		t.Fatalf("cursor=%d scroll=%d, want both 0", m.cursor, m.scroll)
	}
	if got := m.View(); got == "" {
		t.Fatal("expected non-empty view")
	}
}
