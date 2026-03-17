package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestLoadSessionCache_EmptyDBReturnsNil(t *testing.T) {
	useTempHome(t)

	snap, err := LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache: %v", err)
	}
	if snap != nil {
		t.Fatalf("expected nil session cache, got %+v", snap)
	}
}

func TestSaveSessionCache_RoundTrip(t *testing.T) {
	useTempHome(t)

	updatedAt := time.Unix(100, 0)
	repoRootsAt := time.Unix(200, 0)
	statsTTL := time.Unix(300, 0)
	want := &SessionCache{
		UpdatedAt: updatedAt,
		Sessions: []tmux.Session{{
			Name:     "kitmux-main",
			Windows:  2,
			Attached: true,
			Path:     "/tmp/kitmux",
			Activity: 123,
		}},
		RepoRoots:            map[string]string{"kitmux-main": "/tmp/kitmux"},
		RepoRootsRefreshedAt: repoRootsAt,
		Stats:                map[string]DiffStat{"kitmux-main": {RepoRoot: "/tmp/kitmux", Added: 3, Deleted: 1}},
		StatsTTL:             statsTTL,
	}

	if err := SaveSessionCache(want); err != nil {
		t.Fatalf("SaveSessionCache: %v", err)
	}

	got, err := LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected session cache\nwant: %+v\n got: %+v", want, got)
	}
}

func TestSaveSessionCache_OverwritesPreviousSnapshot(t *testing.T) {
	useTempHome(t)

	first := &SessionCache{
		UpdatedAt: time.Unix(10, 0),
		Sessions:  []tmux.Session{{Name: "one", Path: "/tmp/one", Windows: 1, Activity: 1}},
		RepoRoots: map[string]string{"one": "/tmp/one"},
		Stats:     map[string]DiffStat{"one": {RepoRoot: "/tmp/one", Added: 1, Deleted: 1}},
		StatsTTL:  time.Unix(20, 0),
	}
	second := &SessionCache{
		UpdatedAt: time.Unix(30, 0),
		Sessions:  []tmux.Session{{Name: "two", Path: "/tmp/two", Windows: 2, Activity: 2}},
		RepoRoots: map[string]string{"two": "/tmp/two"},
		Stats:     map[string]DiffStat{"two": {RepoRoot: "/tmp/two", Added: 4, Deleted: 0}},
		StatsTTL:  time.Unix(40, 0),
	}

	if err := SaveSessionCache(first); err != nil {
		t.Fatalf("SaveSessionCache first: %v", err)
	}
	if err := SaveSessionCache(second); err != nil {
		t.Fatalf("SaveSessionCache second: %v", err)
	}

	got, err := LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache: %v", err)
	}
	if !reflect.DeepEqual(got, second) {
		t.Fatalf("expected second snapshot only\nwant: %+v\n got: %+v", second, got)
	}
}

func TestLoadSessionCache_ImportsLegacyJSONOnce(t *testing.T) {
	home := useTempHome(t)

	payload := legacySessionCachePayload{
		Version:              1,
		UpdatedAt:            time.Unix(100, 0),
		Sessions:             []tmux.Session{{Name: "kitmux-main", Path: "/tmp/kitmux", Windows: 2, Activity: 123}},
		RepoRoots:            map[string]string{"kitmux-main": "/tmp/kitmux"},
		RepoRootsRefreshedAt: time.Unix(200, 0),
		Stats:                map[string]DiffStat{"kitmux-main": {RepoRoot: "/tmp/kitmux", Added: 3, Deleted: 1}},
		StatsTTL:             time.Unix(300, 0),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal legacy session cache: %v", err)
	}
	writeFile(t, legacySessionCacheJSONPath(home), data)

	got, err := LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache: %v", err)
	}
	if got == nil || len(got.Sessions) != 1 || got.Sessions[0].Name != "kitmux-main" {
		t.Fatalf("expected imported session cache, got %+v", got)
	}

	if err := os.Remove(legacySessionCacheJSONPath(home)); err != nil {
		t.Fatalf("remove legacy session cache: %v", err)
	}

	got, err = LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache after removing legacy file: %v", err)
	}
	if got == nil || len(got.Sessions) != 1 || got.RepoRoots["kitmux-main"] != "/tmp/kitmux" {
		t.Fatalf("expected sqlite-backed session cache after import, got %+v", got)
	}
}

func TestLoadSessionCache_IgnoresInvalidLegacyJSON(t *testing.T) {
	home := useTempHome(t)
	writeFile(t, legacySessionCacheJSONPath(home), []byte("not json"))

	got, err := LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil session cache for invalid legacy json, got %+v", got)
	}
}

func TestLoadSessionCache_IgnoresUnsupportedLegacyVersion(t *testing.T) {
	home := useTempHome(t)

	data, err := json.Marshal(legacySessionCachePayload{Version: 2})
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	writeFile(t, legacySessionCacheJSONPath(home), data)

	got, err := LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil session cache for unsupported version, got %+v", got)
	}
}

func TestInsertWorktreeStats_FillsRepoRootFromRepoRoots(t *testing.T) {
	useTempHome(t)

	db, err := open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	repoRoots := map[string]string{"kitmux-main": "/tmp/kitmux"}
	stats := map[string]DiffStat{"kitmux-main": {Added: 3, Deleted: 1}}
	if err := insertWorktreeStats(tx, repoRoots, stats, time.Unix(50, 0), time.Unix(60, 0)); err != nil {
		t.Fatalf("insertWorktreeStats: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	loaded, err := loadWorktreeStats(db)
	if err != nil {
		t.Fatalf("loadWorktreeStats: %v", err)
	}
	if loaded["kitmux-main"].RepoRoot != "/tmp/kitmux" {
		t.Fatalf("expected repo root to be filled from repoRoots, got %+v", loaded["kitmux-main"])
	}
}

func TestWorkspacesAndSessionCache_CoexistInSameStateDB(t *testing.T) {
	home := useTempHome(t)

	if _, err := AddWorkspace("kitmux", "/tmp/kitmux"); err != nil {
		t.Fatalf("AddWorkspace: %v", err)
	}
	if err := SaveSessionCache(&SessionCache{
		UpdatedAt: time.Unix(100, 0),
		Sessions:  []tmux.Session{{Name: "kitmux-main", Path: "/tmp/kitmux", Windows: 2, Activity: 123}},
		RepoRoots: map[string]string{"kitmux-main": "/tmp/kitmux"},
		Stats:     map[string]DiffStat{"kitmux-main": {RepoRoot: "/tmp/kitmux", Added: 3, Deleted: 1}},
		StatsTTL:  time.Unix(120, 0),
	}); err != nil {
		t.Fatalf("SaveSessionCache: %v", err)
	}

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces: %v", err)
	}
	cache, err := LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache: %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].Path != "/tmp/kitmux" {
		t.Fatalf("unexpected workspaces: %+v", workspaces)
	}
	if cache == nil || len(cache.Sessions) != 1 || cache.Sessions[0].Name != "kitmux-main" {
		t.Fatalf("unexpected session cache: %+v", cache)
	}

	db, err := sql.Open("sqlite", stateDBPath(home))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	var workspaceCount, sessionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&workspaceCount); err != nil {
		t.Fatalf("count workspaces: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM session_snapshots`).Scan(&sessionCount); err != nil {
		t.Fatalf("count session_snapshots: %v", err)
	}
	if workspaceCount != 1 || sessionCount != 1 {
		t.Fatalf("expected shared state.db with 1 workspace and 1 session, got workspaces=%d sessions=%d", workspaceCount, sessionCount)
	}
}
