package threads

import (
	"testing"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestBuildRowsKeepsHeadlessDetailedAndSkipsDuplicatePane(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "droid-app", Path: "/repo/app", Activity: 10, Thread: true, AgentID: "droid"},
		{Name: "work", Path: "/repo/app"},
	}
	panes := []tmux.Pane{
		{SessionName: "droid-app", WindowIndex: 0, PaneIndex: 0, Command: "droid", Path: "/repo/app"},
		{SessionName: "work", WindowIndex: 1, PaneIndex: 2, Command: "codex", Path: "/repo/app"},
	}

	rows := buildRows(sessions, panes)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, rows = %#v", len(rows), rows)
	}
	if rows[0].Kind != RowHeadless || rows[0].AgentID != "droid" {
		t.Fatalf("headless row = %#v", rows[0])
	}
	if rows[1].Kind != RowEphemeral || rows[1].AgentID != "codex" {
		t.Fatalf("ephemeral row = %#v", rows[1])
	}
}
