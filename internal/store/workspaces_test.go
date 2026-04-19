package store

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestLoadWorkspaces_EmptyDBReturnsEmptySlice(t *testing.T) {
	useTempHome(t)

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces: %v", err)
	}
	if len(workspaces) != 0 {
		t.Fatalf("expected empty workspace list, got %+v", workspaces)
	}
}

func TestLoadWorkspaces_ImportsLegacyJSONOnce(t *testing.T) {
	home := useTempHome(t)

	legacy := struct {
		Projects []Workspace `json:"projects"`
	}{Projects: []Workspace{{Name: "kitmux", Path: "/tmp/kitmux", AddedAt: 42}}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy workspaces: %v", err)
	}
	writeFile(t, legacyWorkspacesJSONPath(home), data)

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 imported workspace, got %d", len(workspaces))
	}
	if workspaces[0].AddedAt != 42 {
		t.Fatalf("expected AddedAt=42, got %d", workspaces[0].AddedAt)
	}

	if err := os.Remove(legacyWorkspacesJSONPath(home)); err != nil {
		t.Fatalf("remove legacy workspaces json: %v", err)
	}

	workspaces, err = LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces after removing legacy file: %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].Path != "/tmp/kitmux" {
		t.Fatalf("expected sqlite-backed workspace after import, got %+v", workspaces)
	}
}

func TestSaveWorkspaces_ReplacesDataset(t *testing.T) {
	useTempHome(t)

	first := []Workspace{{Name: "one", Path: "/tmp/one", AddedAt: 1, LastSeenAt: 1}}
	second := []Workspace{{Name: "two", Path: "/tmp/two", AddedAt: 2, LastSeenAt: 2}}

	if err := SaveWorkspaces(first); err != nil {
		t.Fatalf("SaveWorkspaces first: %v", err)
	}
	if err := SaveWorkspaces(second); err != nil {
		t.Fatalf("SaveWorkspaces second: %v", err)
	}

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces: %v", err)
	}
	if !reflect.DeepEqual(workspaces, second) {
		t.Fatalf("expected dataset %+v, got %+v", second, workspaces)
	}
}

func TestSaveWorkspaces_DefaultsLastSeenAtToAddedAt(t *testing.T) {
	useTempHome(t)

	if err := SaveWorkspaces([]Workspace{{Name: "kitmux", Path: "/tmp/kitmux", AddedAt: 99}}); err != nil {
		t.Fatalf("SaveWorkspaces: %v", err)
	}

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(workspaces))
	}
	if workspaces[0].LastSeenAt != workspaces[0].AddedAt {
		t.Fatalf("expected last_seen_at to default to added_at, got %+v", workspaces[0])
	}
}

func TestAddWorkspace_DedupesByPath(t *testing.T) {
	useTempHome(t)

	added, err := AddWorkspace("kitmux", "/tmp/kitmux")
	if err != nil {
		t.Fatalf("AddWorkspace first: %v", err)
	}
	if !added {
		t.Fatal("expected first AddWorkspace to insert")
	}

	added, err = AddWorkspace("kitmux-copy", "/tmp/kitmux")
	if err != nil {
		t.Fatalf("AddWorkspace second: %v", err)
	}
	if added {
		t.Fatal("expected duplicate AddWorkspace to be ignored")
	}

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces: %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].Name != "kitmux" {
		t.Fatalf("expected original workspace to remain, got %+v", workspaces)
	}
}

func TestRemoveWorkspace_RemovesOnlyMatchingPath(t *testing.T) {
	useTempHome(t)

	if err := SaveWorkspaces([]Workspace{
		{Name: "api", Path: "/tmp/acme/api", AddedAt: 1, LastSeenAt: 1},
		{Name: "api", Path: "/tmp/internal/api", AddedAt: 2, LastSeenAt: 2},
	}); err != nil {
		t.Fatalf("SaveWorkspaces: %v", err)
	}

	removed, err := RemoveWorkspace("/tmp/acme/api")
	if err != nil {
		t.Fatalf("RemoveWorkspace: %v", err)
	}
	if !removed {
		t.Fatal("expected RemoveWorkspace to return true")
	}

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces: %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].Path != "/tmp/internal/api" {
		t.Fatalf("expected only the matching path to be removed, got %+v", workspaces)
	}

	removed, err = RemoveWorkspace("/tmp/acme/api")
	if err != nil {
		t.Fatalf("RemoveWorkspace second: %v", err)
	}
	if removed {
		t.Fatal("expected second RemoveWorkspace to return false")
	}
}

func TestHasWorkspacePath_ReflectsCurrentState(t *testing.T) {
	useTempHome(t)

	hasPath, err := HasWorkspacePath("/tmp/kitmux")
	if err != nil {
		t.Fatalf("HasWorkspacePath before insert: %v", err)
	}
	if hasPath {
		t.Fatal("expected path to be absent before insert")
	}

	if _, err := AddWorkspace("kitmux", "/tmp/kitmux"); err != nil {
		t.Fatalf("AddWorkspace: %v", err)
	}

	hasPath, err = HasWorkspacePath("/tmp/kitmux")
	if err != nil {
		t.Fatalf("HasWorkspacePath after insert: %v", err)
	}
	if !hasPath {
		t.Fatal("expected path to be present after insert")
	}
}

func TestArchivedWorktrees_RoundTrip(t *testing.T) {
	useTempHome(t)

	if err := AddArchivedWorktree("/tmp/ws", "/tmp/ws-feature"); err != nil {
		t.Fatalf("AddArchivedWorktree: %v", err)
	}
	if err := AddArchivedWorktree("/tmp/ws", "/tmp/ws-feature"); err != nil {
		t.Fatalf("AddArchivedWorktree duplicate: %v", err)
	}

	all, err := LoadArchivedWorktrees()
	if err != nil {
		t.Fatalf("LoadArchivedWorktrees: %v", err)
	}
	if all["/tmp/ws"] == nil || !all["/tmp/ws"]["/tmp/ws-feature"] {
		t.Fatalf("expected archived worktree in map, got %+v", all)
	}

	if err := RemoveArchivedWorktree("/tmp/ws", "/tmp/ws-feature"); err != nil {
		t.Fatalf("RemoveArchivedWorktree: %v", err)
	}
	all, err = LoadArchivedWorktrees()
	if err != nil {
		t.Fatalf("LoadArchivedWorktrees after remove: %v", err)
	}
	if all["/tmp/ws"] != nil && all["/tmp/ws"]["/tmp/ws-feature"] {
		t.Fatalf("expected worktree removed from archive map, got %+v", all)
	}
}

func TestPurgeArchivedWorktreesForWorkspace(t *testing.T) {
	useTempHome(t)

	_ = AddArchivedWorktree("/tmp/ws-a", "/tmp/ws-a-f1")
	_ = AddArchivedWorktree("/tmp/ws-a", "/tmp/ws-a-f2")
	_ = AddArchivedWorktree("/tmp/ws-b", "/tmp/ws-b-f1")

	if err := PurgeArchivedWorktreesForWorkspace("/tmp/ws-a"); err != nil {
		t.Fatalf("PurgeArchivedWorktreesForWorkspace: %v", err)
	}
	all, err := LoadArchivedWorktrees()
	if err != nil {
		t.Fatalf("LoadArchivedWorktrees: %v", err)
	}
	if len(all["/tmp/ws-a"]) > 0 {
		t.Fatalf("expected ws-a archived items purged, got %+v", all["/tmp/ws-a"])
	}
	if all["/tmp/ws-b"] == nil || !all["/tmp/ws-b"]["/tmp/ws-b-f1"] {
		t.Fatalf("expected ws-b archive untouched, got %+v", all)
	}
}
