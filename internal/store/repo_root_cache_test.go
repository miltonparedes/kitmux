package store

import (
	"reflect"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestSaveRepoRoots_RoundTrip(t *testing.T) {
	useTempHome(t)

	now := time.Unix(1700000000, 0)
	mappings := map[string]string{
		"/home/me/repo-a": "/home/me/repo-a",
		"/home/me/repo-b": "/home/me/repo-b",
	}
	if err := SaveRepoRoots(mappings, now); err != nil {
		t.Fatalf("SaveRepoRoots: %v", err)
	}

	got, err := LoadRepoRootCache()
	if err != nil {
		t.Fatalf("LoadRepoRootCache: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	for path, want := range mappings {
		entry, ok := got[path]
		if !ok {
			t.Errorf("missing path %q", path)
			continue
		}
		if entry.RepoRoot != want {
			t.Errorf("path %q: got repo_root %q, want %q", path, entry.RepoRoot, want)
		}
		if !entry.RefreshedAt.Equal(now) {
			t.Errorf("path %q: got refreshed_at %v, want %v", path, entry.RefreshedAt, now)
		}
	}
}

func TestSaveRepoRoots_SkipsEmptyEntries(t *testing.T) {
	useTempHome(t)

	mappings := map[string]string{
		"":                "/should/skip",
		"/home/me/repo-a": "",
		"/home/me/repo-b": "/home/me/repo-b",
	}
	if err := SaveRepoRoots(mappings, time.Unix(10, 0)); err != nil {
		t.Fatalf("SaveRepoRoots: %v", err)
	}

	got, err := LoadRepoRootCache()
	if err != nil {
		t.Fatalf("LoadRepoRootCache: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(got), got)
	}
	if _, ok := got["/home/me/repo-b"]; !ok {
		t.Error("missing /home/me/repo-b")
	}
}

// TestSaveSessionCache_PreservesWorkspaceRepoRoots guards against the
// regression reported in PR #7 review: SaveSessionCache used to
// DELETE FROM repo_roots, wiping path-keyed workspace cache entries that
// shared the same table. Now that the workspace cache lives in its own
// table (workspace_repo_roots), SaveSessionCache must not affect it.
func TestSaveSessionCache_PreservesWorkspaceRepoRoots(t *testing.T) {
	useTempHome(t)

	now := time.Unix(1700000000, 0)
	workspaceMappings := map[string]string{
		"/home/me/repo-a": "/home/me/repo-a",
		"/home/me/repo-b": "/home/me/repo-b",
	}
	if err := SaveRepoRoots(workspaceMappings, now); err != nil {
		t.Fatalf("SaveRepoRoots: %v", err)
	}

	// Session cache writes its own session-name-keyed repo_roots entry.
	// Before the fix, this would DELETE the path-keyed entries above.
	snap := &SessionCache{
		UpdatedAt: time.Unix(100, 0),
		Sessions:  []tmux.Session{{Name: "repo-a-main", Path: "/home/me/repo-a", Windows: 1, Activity: 1}},
		RepoRoots: map[string]string{"repo-a-main": "/home/me/repo-a"},
	}
	if err := SaveSessionCache(snap); err != nil {
		t.Fatalf("SaveSessionCache: %v", err)
	}

	got, err := LoadRepoRootCache()
	if err != nil {
		t.Fatalf("LoadRepoRootCache: %v", err)
	}
	gotPaths := map[string]string{}
	for path, entry := range got {
		gotPaths[path] = entry.RepoRoot
	}
	if !reflect.DeepEqual(gotPaths, workspaceMappings) {
		t.Fatalf("workspace repo roots were not preserved\nwant: %+v\n got: %+v", workspaceMappings, gotPaths)
	}

	// Sanity: session cache's repo_roots still loaded separately.
	sc, err := LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache: %v", err)
	}
	if sc == nil || sc.RepoRoots["repo-a-main"] != "/home/me/repo-a" {
		t.Fatalf("session cache repo roots not persisted: %+v", sc)
	}
}
