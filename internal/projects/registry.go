package projects

import (
	"sort"
	"sync"

	"github.com/miltonparedes/kitmux/internal/store"
)

// Project represents a registered project.
type Project struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	AddedAt int64  `json:"added_at"`
}

var registryMu sync.Mutex

// LoadRegistry reads the persisted project list.
func LoadRegistry() []Project {
	registryMu.Lock()
	defer registryMu.Unlock()

	records, err := store.LoadProjects()
	if err != nil {
		return nil
	}

	projects := make([]Project, 0, len(records))
	for _, record := range records {
		projects = append(projects, Project{
			Name:    record.Name,
			Path:    record.Path,
			AddedAt: record.AddedAt,
		})
	}
	return projects
}

// SaveRegistry persists the full project list.
func SaveRegistry(projects []Project) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	records := make([]store.Project, 0, len(projects))
	for _, project := range projects {
		records = append(records, store.Project{
			Name:       project.Name,
			Path:       project.Path,
			AddedAt:    project.AddedAt,
			LastSeenAt: project.AddedAt,
		})
	}
	return store.SaveProjects(records)
}

// AddProject adds a project if not already registered by path. Returns true if added.
func AddProject(name, path string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	added, err := store.AddProject(name, path)
	return err == nil && added
}

// RemoveProject removes a project by name. Returns true if found and removed.
func RemoveProject(name string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	removed, err := store.RemoveProject(name)
	return err == nil && removed
}

// HasPath reports whether a path is already registered.
func HasPath(path string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	hasPath, err := store.HasProjectPath(path)
	return err == nil && hasPath
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
