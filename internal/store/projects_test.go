package store

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestLoadProjects_EmptyDBReturnsEmptySlice(t *testing.T) {
	useTempHome(t)

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected empty project list, got %+v", projects)
	}
}

func TestLoadProjects_ImportsLegacyJSONOnce(t *testing.T) {
	home := useTempHome(t)

	legacy := struct {
		Projects []Project `json:"projects"`
	}{Projects: []Project{{Name: "kitmux", Path: "/tmp/kitmux", AddedAt: 42}}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy projects: %v", err)
	}
	writeFile(t, legacyProjectsJSONPath(home), data)

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 imported project, got %d", len(projects))
	}

	if err := os.Remove(legacyProjectsJSONPath(home)); err != nil {
		t.Fatalf("remove legacy projects json: %v", err)
	}

	projects, err = LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects after removing legacy file: %v", err)
	}
	if len(projects) != 1 || projects[0].Path != "/tmp/kitmux" {
		t.Fatalf("expected sqlite-backed project after import, got %+v", projects)
	}
}

func TestSaveProjects_ReplacesDataset(t *testing.T) {
	useTempHome(t)

	first := []Project{{Name: "one", Path: "/tmp/one", AddedAt: 1, LastSeenAt: 1}}
	second := []Project{{Name: "two", Path: "/tmp/two", AddedAt: 2, LastSeenAt: 2}}

	if err := SaveProjects(first); err != nil {
		t.Fatalf("SaveProjects first: %v", err)
	}
	if err := SaveProjects(second); err != nil {
		t.Fatalf("SaveProjects second: %v", err)
	}

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if !reflect.DeepEqual(projects, second) {
		t.Fatalf("expected dataset %+v, got %+v", second, projects)
	}
}

func TestSaveProjects_DefaultsLastSeenAtToAddedAt(t *testing.T) {
	useTempHome(t)

	if err := SaveProjects([]Project{{Name: "kitmux", Path: "/tmp/kitmux", AddedAt: 99}}); err != nil {
		t.Fatalf("SaveProjects: %v", err)
	}

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].LastSeenAt != projects[0].AddedAt {
		t.Fatalf("expected last_seen_at to default to added_at, got %+v", projects[0])
	}
}

func TestAddProject_DedupesByPath(t *testing.T) {
	useTempHome(t)

	added, err := AddProject("kitmux", "/tmp/kitmux")
	if err != nil {
		t.Fatalf("AddProject first: %v", err)
	}
	if !added {
		t.Fatal("expected first AddProject to insert")
	}

	added, err = AddProject("kitmux-copy", "/tmp/kitmux")
	if err != nil {
		t.Fatalf("AddProject second: %v", err)
	}
	if added {
		t.Fatal("expected duplicate AddProject to be ignored")
	}

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "kitmux" {
		t.Fatalf("expected original project to remain, got %+v", projects)
	}
}

func TestRemoveProject_RemovesByName(t *testing.T) {
	useTempHome(t)

	if err := SaveProjects([]Project{{Name: "kitmux", Path: "/tmp/kitmux", AddedAt: 1, LastSeenAt: 1}}); err != nil {
		t.Fatalf("SaveProjects: %v", err)
	}

	removed, err := RemoveProject("kitmux")
	if err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}
	if !removed {
		t.Fatal("expected RemoveProject to return true")
	}

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected empty projects after remove, got %+v", projects)
	}

	removed, err = RemoveProject("kitmux")
	if err != nil {
		t.Fatalf("RemoveProject second: %v", err)
	}
	if removed {
		t.Fatal("expected second RemoveProject to return false")
	}
}

func TestHasProjectPath_ReflectsCurrentState(t *testing.T) {
	useTempHome(t)

	hasPath, err := HasProjectPath("/tmp/kitmux")
	if err != nil {
		t.Fatalf("HasProjectPath before insert: %v", err)
	}
	if hasPath {
		t.Fatal("expected path to be absent before insert")
	}

	if _, err := AddProject("kitmux", "/tmp/kitmux"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	hasPath, err = HasProjectPath("/tmp/kitmux")
	if err != nil {
		t.Fatalf("HasProjectPath after insert: %v", err)
	}
	if !hasPath {
		t.Fatal("expected path to be present after insert")
	}
}
