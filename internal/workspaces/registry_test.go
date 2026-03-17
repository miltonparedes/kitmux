package workspaces

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistry_ImportsLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	legacyPath := filepath.Join(dir, ".config", "kitmux", "projects.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatalf("mkdir legacy path: %v", err)
	}
	data, err := json.Marshal(struct {
		Workspaces []Workspace `json:"projects"`
	}{Workspaces: []Workspace{{Name: "kitmux", Path: "/tmp/kitmux", AddedAt: 42}}})
	if err != nil {
		t.Fatalf("marshal legacy registry: %v", err)
	}
	if err := os.WriteFile(legacyPath, data, 0o600); err != nil {
		t.Fatalf("write legacy registry: %v", err)
	}

	loaded := LoadRegistry()
	if len(loaded) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(loaded))
	}
	if loaded[0].Name != "kitmux" || loaded[0].Path != "/tmp/kitmux" {
		t.Fatalf("unexpected workspace: %+v", loaded[0])
	}
	if loaded[0].AddedAt != 42 {
		t.Fatalf("expected AddedAt=42, got %d", loaded[0].AddedAt)
	}

	if err := os.Remove(legacyPath); err != nil {
		t.Fatalf("remove legacy registry: %v", err)
	}

	loaded = LoadRegistry()
	if len(loaded) != 1 {
		t.Fatalf("expected sqlite-backed workspace after legacy removal, got %d", len(loaded))
	}
	if loaded[0].Name != "kitmux" || loaded[0].Path != "/tmp/kitmux" {
		t.Fatalf("unexpected sqlite-backed workspace: %+v", loaded[0])
	}
}

func TestAddWorkspace_DedupesByPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if !AddWorkspace("kitmux", "/tmp/kitmux") {
		t.Fatal("expected first add to succeed")
	}
	if AddWorkspace("kitmux-copy", "/tmp/kitmux") {
		t.Fatal("expected duplicate path add to be ignored")
	}

	loaded := LoadRegistry()
	if len(loaded) != 1 {
		t.Fatalf("expected 1 workspace after dedupe, got %d", len(loaded))
	}
	if loaded[0].Name != "kitmux" {
		t.Fatalf("expected original workspace to remain, got %+v", loaded[0])
	}
}
