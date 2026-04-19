package sessions

import (
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/store"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

// TestSharedStatsForSessions_UsesWorkspaceStatsByPath verifies that the
// sessions view reads from the shared workspace_stats table (keyed by
// worktree path), so it benefits from the dashboard's cache and vice-versa.
func TestSharedStatsForSessions_UsesWorkspaceStatsByPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store.ResetForTests()
	t.Cleanup(store.ResetForTests)

	// Seed workspace_stats as the dashboard would after a refresh.
	if err := store.ReplaceWorkspaceStats(
		"/repos/app",
		[]store.WorktreeStat{
			{WorkspacePath: "/repos/app", Branch: "main", WorktreePath: "/repos/app", Added: 2, Deleted: 1, IsMain: true},
			{WorkspacePath: "/repos/app", Branch: "feat-x", WorktreePath: "/repos/app-feat-x", Added: 7, Deleted: 3},
		},
		time.Now(),
	); err != nil {
		t.Fatalf("seed workspace stats: %v", err)
	}

	sessions := []tmux.Session{
		{Name: "app-main", Path: "/repos/app"},
		{Name: "app-feat-x", Path: "/repos/app-feat-x"},
		{Name: "no-path", Path: ""},
		{Name: "unknown", Path: "/not/in/cache"},
	}

	got := sharedStatsForSessions(sessions)
	if got == nil {
		t.Fatal("expected non-nil stats map")
	}
	if got["app-main"] != (sessionStats{Added: 2, Deleted: 1}) {
		t.Errorf("app-main: got %+v, want +2/-1", got["app-main"])
	}
	if got["app-feat-x"] != (sessionStats{Added: 7, Deleted: 3}) {
		t.Errorf("app-feat-x: got %+v, want +7/-3", got["app-feat-x"])
	}
	if _, ok := got["no-path"]; ok {
		t.Error("sessions without a path should be skipped")
	}
	if _, ok := got["unknown"]; ok {
		t.Error("sessions with an unknown path should be skipped")
	}
}

// TestSharedStatsForSessions_OmitsZeroDiffs keeps the sessions tree tidy:
// entries with +0/-0 diff are not emitted, matching the legacy behavior
// where only dirty workers surface a summary.
func TestSharedStatsForSessions_OmitsZeroDiffs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store.ResetForTests()
	t.Cleanup(store.ResetForTests)

	if err := store.ReplaceWorkspaceStats(
		"/repos/clean",
		[]store.WorktreeStat{
			{WorkspacePath: "/repos/clean", Branch: "main", WorktreePath: "/repos/clean", IsMain: true},
		},
		time.Now(),
	); err != nil {
		t.Fatalf("seed workspace stats: %v", err)
	}

	got := sharedStatsForSessions([]tmux.Session{{Name: "clean", Path: "/repos/clean"}})
	if _, ok := got["clean"]; ok {
		t.Errorf("expected clean session to be omitted from diff stats, got %+v", got)
	}
}

// TestSharedStatsForSessions_EmptyCacheReturnsNil exercises the fast path
// where no workspace stats have been recorded yet.
func TestSharedStatsForSessions_EmptyCacheReturnsNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store.ResetForTests()
	t.Cleanup(store.ResetForTests)

	got := sharedStatsForSessions([]tmux.Session{{Name: "app-main", Path: "/repos/app"}})
	if got != nil {
		t.Fatalf("expected nil on empty cache, got %+v", got)
	}
}
