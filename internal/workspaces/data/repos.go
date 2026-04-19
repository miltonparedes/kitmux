package data

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/miltonparedes/kitmux/internal/store"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

// repoRootCacheTTL controls how long a cached path→root mapping is trusted
// without re-verifying with git. Two hours covers the common case (rename,
// reinit) while keeping startup cost near zero.
const repoRootCacheTTL = 2 * time.Hour

// ResolveRepoRoots maps session names to their git repo root, keeping only
// sessions whose name lines up with the repo basename (exact or dash prefix).
//
// The resolution is backed by a persistent path→root cache so startup is
// typically zero git calls. Stale or missing entries are refreshed in
// parallel and the new mapping is persisted for next time.
func ResolveRepoRoots(sessions []tmux.Session) map[string]string {
	if len(sessions) == 0 {
		return map[string]string{}
	}

	type bySession struct {
		name string
		path string
	}
	uniquePaths := make(map[string]struct{})
	keep := make([]bySession, 0, len(sessions))
	for _, s := range sessions {
		if s.Path == "" {
			continue
		}
		keep = append(keep, bySession{name: s.Name, path: s.Path})
		uniquePaths[s.Path] = struct{}{}
	}

	cached, _ := store.LoadRepoRootCache()
	now := time.Now()

	// Build the path→root map: reuse cache when fresh, otherwise enqueue.
	resolved := make(map[string]string, len(uniquePaths))
	var toResolve []string
	for path := range uniquePaths {
		if entry, ok := cached[path]; ok && !entry.RefreshedAt.IsZero() && now.Sub(entry.RefreshedAt) < repoRootCacheTTL {
			resolved[path] = entry.RepoRoot
			continue
		}
		toResolve = append(toResolve, path)
	}

	// Resolve uncached paths in parallel.
	if len(toResolve) > 0 {
		var (
			mu  sync.Mutex
			wg  sync.WaitGroup
			out = make(map[string]string, len(toResolve))
		)
		for _, path := range toResolve {
			wg.Add(1)
			go func() {
				defer wg.Done()
				root := ResolveRepoRoot(path)
				mu.Lock()
				out[path] = root
				mu.Unlock()
			}()
		}
		wg.Wait()
		for path, root := range out {
			resolved[path] = root
		}
		// Persist freshly resolved mappings (skip empty roots — those are
		// non-git directories we don't want to re-try on every open).
		persist := make(map[string]string, len(out))
		for path, root := range out {
			if root != "" {
				persist[path] = root
			}
		}
		if len(persist) > 0 {
			_ = store.SaveRepoRoots(persist, now)
		}
	}

	roots := make(map[string]string, len(keep))
	for _, s := range keep {
		root := resolved[s.path]
		if root == "" {
			continue
		}
		base := filepath.Base(root)
		normName := Normalize(s.name)
		normBase := Normalize(base)
		if normName == normBase || strings.HasPrefix(normName, normBase+"-") {
			roots[s.name] = root
		}
	}
	return roots
}

// ResolveRepoRoot returns the git common repo root for a directory, or empty
// string when the path is not tracked by git.
func ResolveRepoRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return ""
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(dir, commonDir)
	}
	return filepath.Dir(filepath.Clean(commonDir))
}

// ResolveGitBranch returns the current branch for a directory, or empty.
func ResolveGitBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Normalize replaces _ and spaces with - so naming heuristics compare fairly.
func Normalize(name string) string {
	s := strings.ReplaceAll(name, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// IsMainBranch returns true for the canonical default branches.
func IsMainBranch(name string) bool {
	n := Normalize(name)
	return n == "main" || n == "master" ||
		strings.HasSuffix(n, "-main") || strings.HasSuffix(n, "-master")
}
