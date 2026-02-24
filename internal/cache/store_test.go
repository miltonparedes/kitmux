package cache

import (
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

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	p := filepath.Join(dir, cacheDir, cacheFile)
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, []byte("not json"), 0o600)

	if snap := Load(); snap != nil {
		t.Errorf("expected nil for invalid JSON, got %+v", snap)
	}
}

func TestLoad_WrongVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	p := filepath.Join(dir, cacheDir, cacheFile)
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, []byte(`{"version":999}`), 0o600)

	if snap := Load(); snap != nil {
		t.Errorf("expected nil for wrong version, got %+v", snap)
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
