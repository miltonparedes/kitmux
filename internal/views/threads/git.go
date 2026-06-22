package threads

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type gitMeta struct {
	project string
	branch  string
}

type gitCacheEntry struct {
	meta      gitMeta
	fetchedAt time.Time
}

const gitMetaTTL = 5 * time.Second

var (
	gitCacheMu sync.Mutex
	gitCache   = map[string]gitCacheEntry{}
)

// pathGitMeta resolves the project (repo basename) and branch for a directory,
// caching results per path so the threads refresh loop stays cheap.
func pathGitMeta(path string) gitMeta {
	if path == "" {
		return gitMeta{}
	}
	now := time.Now()

	gitCacheMu.Lock()
	if entry, ok := gitCache[path]; ok && now.Sub(entry.fetchedAt) < gitMetaTTL {
		gitCacheMu.Unlock()
		return entry.meta
	}
	gitCacheMu.Unlock()

	meta := fetchGitMeta(path)

	gitCacheMu.Lock()
	gitCache[path] = gitCacheEntry{meta: meta, fetchedAt: now}
	gitCacheMu.Unlock()

	return meta
}

func fetchGitMeta(path string) gitMeta {
	project := filepath.Base(filepath.Clean(path))
	if root := repoRoot(path); root != "" {
		project = filepath.Base(root)
	}
	return gitMeta{project: project, branch: gitBranch(path)}
}

// repoRoot returns the shared repository root for a directory, resolving
// worktrees to their common root so every worktree maps to the same project.
func repoRoot(dir string) string {
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

func gitBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return ""
	}
	return branch
}
