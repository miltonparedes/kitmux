package workspaces

import (
	"sort"
	"sync"

	"github.com/miltonparedes/kitmux/internal/store"
)

// Workspace is an alias for store.Workspace so callers keep a clean import.
type Workspace = store.Workspace

var registryMu sync.Mutex

// LoadRegistry reads the persisted workspace list.
func LoadRegistry() []Workspace {
	registryMu.Lock()
	defer registryMu.Unlock()

	workspaces, err := store.LoadWorkspaces()
	if err != nil {
		return nil
	}
	return workspaces
}

// SaveRegistry persists the full workspace list.
func SaveRegistry(workspaces []Workspace) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	return store.SaveWorkspaces(workspaces)
}

// AddWorkspace adds a workspace if not already registered by path. Returns true if added.
func AddWorkspace(name, path string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	added, err := store.AddWorkspace(name, path)
	return err == nil && added
}

// RemoveWorkspace removes a workspace by path. Returns true if found and removed.
func RemoveWorkspace(path string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	removed, err := store.RemoveWorkspace(path)
	return err == nil && removed
}

// HasPath reports whether a path is already registered.
func HasPath(path string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	hasPath, err := store.HasWorkspacePath(path)
	return err == nil && hasPath
}

func AddArchivedWorktree(workspacePath, worktreePath string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	err := store.AddArchivedWorktree(workspacePath, worktreePath)
	return err == nil
}

func RemoveArchivedWorktree(workspacePath, worktreePath string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	err := store.RemoveArchivedWorktree(workspacePath, worktreePath)
	return err == nil
}

func LoadArchivedWorktrees() map[string]map[string]bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	archived, err := store.LoadArchivedWorktrees()
	if err != nil {
		return nil
	}
	return archived
}

func PurgeArchivedWorktreesForWorkspace(workspacePath string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	err := store.PurgeArchivedWorktreesForWorkspace(workspacePath)
	return err == nil
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
