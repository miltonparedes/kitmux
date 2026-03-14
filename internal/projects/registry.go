package projects

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const registryFile = "projects.json"

// Project represents a registered project.
type Project struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	AddedAt int64  `json:"added_at"`
}

type registry struct {
	Projects []Project `json:"projects"`
}

var registryMu sync.Mutex

// LoadRegistry reads the persisted project list.
func LoadRegistry() []Project {
	registryMu.Lock()
	defer registryMu.Unlock()

	data, err := os.ReadFile(registryPath()) //nolint:gosec // path is derived from user home dir
	if err != nil {
		return nil
	}
	var r registry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil
	}
	return r.Projects
}

// SaveRegistry persists the full project list.
func SaveRegistry(projects []Project) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	return saveRegistryLocked(projects)
}

// AddProject adds a project if not already registered by path. Returns true if added.
func AddProject(name, path string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	projects := loadRegistryLocked()
	for _, p := range projects {
		if p.Path == path {
			return false
		}
	}
	projects = append(projects, Project{
		Name:    name,
		Path:    path,
		AddedAt: time.Now().Unix(),
	})
	_ = saveRegistryLocked(projects)
	return true
}

// RemoveProject removes a project by name. Returns true if found and removed.
func RemoveProject(name string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	projects := loadRegistryLocked()
	for i, p := range projects {
		if p.Name == name {
			projects = append(projects[:i], projects[i+1:]...)
			_ = saveRegistryLocked(projects)
			return true
		}
	}
	return false
}

// HasPath reports whether a path is already registered.
func HasPath(path string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	for _, p := range loadRegistryLocked() {
		if p.Path == path {
			return true
		}
	}
	return false
}

// SortProjects sorts active projects first (by activity desc), then inactive alphabetically.
func SortProjects(projects []Project, activePaths map[string]int64) {
	sort.SliceStable(projects, func(i, j int) bool {
		ai := activePaths[projects[i].Path]
		aj := activePaths[projects[j].Path]
		if (ai > 0) != (aj > 0) {
			return ai > 0
		}
		if ai > 0 && aj > 0 {
			return ai > aj
		}
		return projects[i].Name < projects[j].Name
	})
}

func loadRegistryLocked() []Project {
	data, err := os.ReadFile(registryPath()) //nolint:gosec // path is derived from user home dir
	if err != nil {
		return nil
	}
	var r registry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil
	}
	return r.Projects
}

func saveRegistryLocked(projects []Project) error {
	p := registryPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(registry{Projects: projects})
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func registryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "kitmux", registryFile)
}
