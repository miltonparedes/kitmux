package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	snap := &Snapshot{
		Sessions: []tmux.Session{
			{Name: "test", Windows: 2, Path: "/tmp/test"},
		},
		RepoRoots: map[string]string{"test": "/tmp/test"},
	}

	if err := Save(snap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := Load()
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if len(loaded.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(loaded.Sessions))
	}
	if loaded.Sessions[0].Name != "test" {
		t.Errorf("expected session name 'test', got %q", loaded.Sessions[0].Name)
	}
	if loaded.RepoRoots["test"] != "/tmp/test" {
		t.Errorf("expected repo root '/tmp/test', got %q", loaded.RepoRoots["test"])
	}
	if loaded.Version != version {
		t.Errorf("expected version %d, got %d", version, loaded.Version)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if snap := Load(); snap != nil {
		t.Errorf("expected nil for missing file, got %+v", snap)
	}
}

func TestLoad_ImportsLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	p := filepath.Join(dir, cacheDir, cacheFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatalf("mkdir legacy cache dir: %v", err)
	}
	data, err := json.Marshal(&Snapshot{
		Version:              version,
		UpdatedAt:            time.Unix(100, 0),
		Sessions:             []tmux.Session{{Name: "kitmux-main", Windows: 2, Path: "/tmp/kitmux", Activity: 123}},
		RepoRoots:            map[string]string{"kitmux-main": "/tmp/kitmux"},
		RepoRootsRefreshedAt: time.Unix(200, 0),
		Stats:                map[string]DiffStat{"kitmux-main": {Added: 3, Deleted: 1}},
		StatsTTL:             time.Unix(300, 0),
	})
	if err != nil {
		t.Fatalf("marshal legacy cache: %v", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write legacy cache: %v", err)
	}

	snap := Load()
	if snap == nil {
		t.Fatal("expected imported snapshot, got nil")
	}
	if len(snap.Sessions) != 1 || snap.Sessions[0].Name != "kitmux-main" {
		t.Fatalf("unexpected imported sessions: %+v", snap.Sessions)
	}

	if err := os.Remove(p); err != nil {
		t.Fatalf("remove legacy cache file: %v", err)
	}

	snap = Load()
	if snap == nil || snap.RepoRoots["kitmux-main"] != "/tmp/kitmux" {
		t.Fatalf("expected sqlite-backed snapshot after import, got %+v", snap)
	}
}

func TestStatsValid(t *testing.T) {
	snap := &Snapshot{
		Stats:    map[string]DiffStat{"a": {Added: 1}},
		StatsTTL: time.Now().Add(time.Minute),
	}
	if !snap.StatsValid() {
		t.Error("expected stats to be valid")
	}

	snap.StatsTTL = time.Now().Add(-time.Minute)
	if snap.StatsValid() {
		t.Error("expected stats to be expired")
	}
}

func TestStatsValid_NilSnapshot(t *testing.T) {
	var snap *Snapshot
	if snap.StatsValid() {
		t.Error("expected nil snapshot stats to be invalid")
	}
}

func TestUpdate_ReadModifyWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := Save(&Snapshot{RepoRoots: map[string]string{"a": "/repo/a"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	err := Update(func(s *Snapshot) {
		s.Stats = map[string]DiffStat{"a": {Added: 3, Deleted: 1}}
		s.StatsTTL = time.Now().Add(time.Minute)
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	loaded := Load()
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.RepoRoots["a"] != "/repo/a" {
		t.Errorf("expected existing repo root to be preserved, got %q", loaded.RepoRoots["a"])
	}
	if loaded.Stats["a"].Added != 3 || loaded.Stats["a"].Deleted != 1 {
		t.Errorf("expected stats to be updated, got %+v", loaded.Stats["a"])
	}
}
