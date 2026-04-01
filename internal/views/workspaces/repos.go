package workspaces

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

// resolveRepoRoots maps session names to their git repo root paths.
func resolveRepoRoots(sessions []tmux.Session) map[string]string {
	roots := make(map[string]string)
	for _, s := range sessions {
		if s.Path == "" {
			continue
		}
		root := resolveRepoRoot(s.Path)
		if root == "" {
			continue
		}
		base := filepath.Base(root)
		normName := normalize(s.Name)
		normBase := normalize(base)
		if normName == normBase || strings.HasPrefix(normName, normBase+"-") {
			roots[s.Name] = root
		}
	}
	return roots
}

func resolveRepoRoot(dir string) string {
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

func resolveGitBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func normalize(name string) string {
	s := strings.ReplaceAll(name, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}
