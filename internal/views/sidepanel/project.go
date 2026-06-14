package workbench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type projectStats struct {
	Name      string
	Path      string
	Branch    string
	Added     int
	Deleted   int
	Staged    int
	Unstaged  int
	Untracked int
	Err       string
}

func loadProjectStats() projectStats {
	path, err := tmuxCurrentPath()
	if err != nil {
		return projectStats{Err: err.Error()}
	}
	root, err := gitOutput(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return projectStats{Name: filepath.Base(path), Path: path, Err: "not a git repo"}
	}
	branch, _ := gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD")
	added, deleted := gitDiffLines(root)
	staged, unstaged, untracked := gitStatus(root)
	return projectStats{
		Name:      filepath.Base(root),
		Path:      root,
		Branch:    branch,
		Added:     added,
		Deleted:   deleted,
		Staged:    staged,
		Unstaged:  unstaged,
		Untracked: untracked,
	}
}

func tmuxCurrentPath() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_current_path}").Output()
	if err == nil {
		path := strings.TrimSpace(string(out))
		if path != "" {
			return path, nil
		}
	}
	return os.Getwd()
}

func gitOutput(dir string, args ...string) (string, error) {
	all := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", all...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitDiffLines(root string) (int, int) {
	out, err := exec.Command("git", "-C", root, "diff", "--numstat").Output()
	if err != nil {
		return 0, 0
	}
	var added, deleted int
	for _, line := range strings.Split(strings.TrimRight(string(out), "\r\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if n, err := strconv.Atoi(fields[0]); err == nil {
			added += n
		}
		if n, err := strconv.Atoi(fields[1]); err == nil {
			deleted += n
		}
	}
	return added, deleted
}

func gitStatus(root string) (staged, unstaged, untracked int) {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return 0, 0, 0
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\r\n"), "\n") {
		if len(line) < 3 {
			continue
		}
		if line[:2] == "??" {
			untracked++
		} else {
			if line[0] != ' ' {
				staged++
			}
			if line[1] != ' ' {
				unstaged++
			}
		}
	}
	return staged, unstaged, untracked
}
