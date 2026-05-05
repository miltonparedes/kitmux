package workbench

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitStatusPreservesLeadingStatusColumns(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")

	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, root, "add", "file.txt")
	runGit(t, root, "commit", "-m", "init")
	if err := os.WriteFile(path, []byte("two\n"), 0o600); err != nil {
		t.Fatalf("modify file: %v", err)
	}

	staged, unstaged, untracked, changed := gitStatus(root)
	if staged != 0 || unstaged != 1 || untracked != 0 {
		t.Fatalf("expected unstaged-only change, got staged=%d unstaged=%d untracked=%d", staged, unstaged, untracked)
	}
	if len(changed) != 1 || changed[0] != "file.txt" {
		t.Fatalf("expected file.txt changed, got %+v", changed)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
