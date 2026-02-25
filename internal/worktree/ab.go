package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func PrepareABWorktrees(cwd, baseBranch string) (string, string, error) {
	root, err := gitOutput(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("resolve repo root: %w", err)
	}
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
	if !branchExists(root, baseBranch) {
		return "", "", fmt.Errorf("base branch %q does not exist", baseBranch)
	}

	repoName := filepath.Base(root)
	parent := filepath.Dir(root)
	codexPath := filepath.Join(parent, repoName+"-ab-codex")
	claudePath := filepath.Join(parent, repoName+"-ab-claude")

	codexBranch := abBranchName(baseBranch, "codex")
	claudeBranch := abBranchName(baseBranch, "claude")

	if err := ensureWorktree(root, codexPath, codexBranch, baseBranch); err != nil {
		return "", "", err
	}
	if err := ensureWorktree(root, claudePath, claudeBranch, baseBranch); err != nil {
		return "", "", err
	}

	return codexPath, claudePath, nil
}

func ensureWorktree(repoRoot, path, branch, baseBranch string) error {
	ok, err := isKnownWorktree(repoRoot, path)
	if err != nil {
		return fmt.Errorf("check worktree %q: %w", path, err)
	}
	if ok {
		return nil
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("path exists and is not a git worktree: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat worktree path: %w", err)
	}

	var cmd *exec.Cmd
	if branchExists(repoRoot, branch) {
		cmd = exec.Command("git", "-C", repoRoot, "worktree", "add", path, branch)
	} else {
		cmd = exec.Command("git", "-C", repoRoot, "worktree", "add", "-b", branch, path, baseBranch)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add %s: %w (%s)", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isKnownWorktree(repoRoot, path string) (bool, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}

	normalized := filepath.Clean(path)
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		listed := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if filepath.Clean(listed) == normalized {
			return true, nil
		}
	}
	return false, nil
}

func gitOutput(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func branchExists(repoRoot, branch string) bool {
	return exec.Command("git", "-C", repoRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil
}

func abBranchName(baseBranch, agent string) string {
	base := strings.TrimSpace(baseBranch)
	if base == "" {
		base = "main"
	}
	base = strings.ReplaceAll(base, " ", "-")
	return "ab/" + base + "-" + agent
}
