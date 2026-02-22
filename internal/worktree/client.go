package worktree

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// List returns all worktrees via `wt list --format=json`.
func List() ([]Worktree, error) {
	return ListInDir("")
}

// ListInDir returns all worktrees for the repo at the given directory.
// If dir is empty, it uses the current working directory.
func ListInDir(dir string) ([]Worktree, error) {
	cmd := exec.Command("wt", "list", "--format=json")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wt list: %w", err)
	}
	var wts []Worktree
	if err := json.Unmarshal(out, &wts); err != nil {
		return nil, fmt.Errorf("wt list parse: %w", err)
	}
	return wts, nil
}

// SwitchTo switches to a worktree branch via `wt switch <branch>`.
func SwitchTo(branch string) error {
	return exec.Command("wt", "switch", branch).Run()
}

// Create creates and switches to a new worktree branch.
func Create(branch string) error {
	return exec.Command("wt", "switch", "--create", branch).Run()
}

// Remove removes a worktree branch.
func Remove(branch string) error {
	return exec.Command("wt", "remove", branch).Run()
}
