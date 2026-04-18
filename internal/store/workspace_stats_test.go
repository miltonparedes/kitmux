package store

import (
	"testing"
	"time"
)

func TestReplaceAndLoadWorkspaceStats(t *testing.T) {
	useTempHome(t)

	now := time.Now()
	stats := []WorktreeStat{
		{
			Branch:       "main",
			WorktreePath: "/repo",
			Added:        0,
			Deleted:      0,
			IsMain:       true,
			CommitSHA:    "abc",
			CommitTS:     1,
		},
		{
			Branch:       "feature",
			WorktreePath: "/repo-feature",
			Added:        5,
			Deleted:      2,
			Modified:     true,
			Ahead:        1,
			CommitSHA:    "def",
			CommitTS:     2,
		},
	}
	if err := ReplaceWorkspaceStats("/repo", stats, now); err != nil {
		t.Fatalf("replace: %v", err)
	}

	got, err := LoadWorkspaceStats("/repo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(got))
	}

	meta, err := LoadWorkspaceMeta("/repo")
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	if meta.LastStatsRefresh.IsZero() {
		t.Error("expected LastStatsRefresh to be populated")
	}
}

func TestReplaceWorkspaceStatsOverwrites(t *testing.T) {
	useTempHome(t)

	now := time.Now()
	first := []WorktreeStat{{Branch: "a", WorktreePath: "/r-a"}}
	if err := ReplaceWorkspaceStats("/r", first, now); err != nil {
		t.Fatalf("first: %v", err)
	}
	second := []WorktreeStat{{Branch: "b", WorktreePath: "/r-b"}}
	if err := ReplaceWorkspaceStats("/r", second, now.Add(time.Second)); err != nil {
		t.Fatalf("second: %v", err)
	}
	got, err := LoadWorkspaceStats("/r")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].WorktreePath != "/r-b" {
		t.Fatalf("expected only /r-b, got %+v", got)
	}
}

func TestPurgeWorkspaceStatsRemovesBoth(t *testing.T) {
	useTempHome(t)

	now := time.Now()
	if err := ReplaceWorkspaceStats("/r", []WorktreeStat{{Branch: "a", WorktreePath: "/r"}}, now); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := PurgeWorkspaceStats("/r"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	got, err := LoadWorkspaceStats("/r")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no stats after purge, got %d", len(got))
	}
	meta, err := LoadWorkspaceMeta("/r")
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	if !meta.LastStatsRefresh.IsZero() {
		t.Errorf("expected meta purged, got %v", meta.LastStatsRefresh)
	}
}

func TestLoadAllWorkspaceStatsReturnsByPath(t *testing.T) {
	useTempHome(t)

	now := time.Now()
	_ = ReplaceWorkspaceStats("/a", []WorktreeStat{{Branch: "main", WorktreePath: "/a"}}, now)
	_ = ReplaceWorkspaceStats("/b", []WorktreeStat{{Branch: "main", WorktreePath: "/b"}}, now)

	all, err := LoadAllWorkspaceStats()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
	if _, ok := all["/a"]; !ok {
		t.Error("expected /a in results")
	}
	if _, ok := all["/b"]; !ok {
		t.Error("expected /b in results")
	}
}

func TestTouchWorkspaceOpenedUpdatesMeta(t *testing.T) {
	useTempHome(t)

	now := time.Now()
	if err := TouchWorkspaceOpened("/r", now); err != nil {
		t.Fatalf("touch: %v", err)
	}
	meta, err := LoadWorkspaceMeta("/r")
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	if meta.LastOpenedAt.IsZero() {
		t.Error("expected LastOpenedAt populated")
	}
}
