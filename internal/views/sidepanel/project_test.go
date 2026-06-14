package sidepanel

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

	staged, unstaged, untracked := gitStatus(root)
	if staged != 0 || unstaged != 1 || untracked != 0 {
		t.Fatalf("expected unstaged-only change, got staged=%d unstaged=%d untracked=%d", staged, unstaged, untracked)
	}
}

func TestGitDiffLinesCountsUnstagedChanges(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")

	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, root, "add", "file.txt")
	runGit(t, root, "commit", "-m", "init")
	if err := os.WriteFile(path, []byte("one\nthree\nfour\n"), 0o600); err != nil {
		t.Fatalf("modify file: %v", err)
	}

	added, deleted := gitDiffLines(root)
	if added != 2 || deleted != 1 {
		t.Fatalf("expected +2/-1, got +%d/-%d", added, deleted)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
