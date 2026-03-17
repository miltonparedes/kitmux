package workspaces

import (
	"sort"
	"sync"

	"github.com/miltonparedes/kitmux/internal/store"
)

// Workspace represents a registered workspace.
type Workspace struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	AddedAt int64  `json:"added_at"`
}

var registryMu sync.Mutex

// LoadRegistry reads the persisted workspace list.
func LoadRegistry() []Workspace {
	registryMu.Lock()
	defer registryMu.Unlock()

	records, err := store.LoadProjects()
	if err != nil {
		return nil
	}

	workspaces := make([]Workspace, 0, len(records))
	for _, record := range records {
		workspaces = append(workspaces, Workspace{
			Name:    record.Name,
			Path:    record.Path,
			AddedAt: record.AddedAt,
		})
	}
	return workspaces
}

// SaveRegistry persists the full workspace list.
func SaveRegistry(workspaces []Workspace) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	records := make([]store.Project, 0, len(workspaces))
	for _, ws := range workspaces {
		records = append(records, store.Project{
			Name:       ws.Name,
			Path:       ws.Path,
			AddedAt:    ws.AddedAt,
			LastSeenAt: ws.AddedAt,
		})
	}
	return store.SaveProjects(records)
}

// AddWorkspace adds a workspace if not already registered by path. Returns true if added.
func AddWorkspace(name, path string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	added, err := store.AddProject(name, path)
	return err == nil && added
}

// RemoveWorkspace removes a workspace by name. Returns true if found and removed.
func RemoveWorkspace(name string) bool {
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

// SortWorkspaces sorts active workspaces first (by activity desc), then inactive alphabetically.
func SortWorkspaces(workspaces []Workspace, activePaths map[string]int64) {
	sort.SliceStable(workspaces, func(i, j int) bool {
		ai := activePaths[workspaces[i].Path]
		aj := activePaths[workspaces[j].Path]
		if (ai > 0) != (aj > 0) {
			return ai > 0
		}
		if ai > 0 && aj > 0 {
			return ai > aj
		}
		return workspaces[i].Name < workspaces[j].Name
	})
}
