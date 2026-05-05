package workbench

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type projectStats struct {
	Name         string
	Path         string
	Branch       string
	Files        int
	Lines        int
	Added        int
	Deleted      int
	Staged       int
	Unstaged     int
	Untracked    int
	ChangedFiles []string
	Err          string
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
	files := gitFiles(root)
	added, deleted := gitDiffLines(root)
	staged, unstaged, untracked, changed := gitStatus(root)
	return projectStats{
		Name:         filepath.Base(root),
		Path:         root,
		Branch:       branch,
		Files:        len(files),
		Lines:        countTextLines(root, files),
		Added:        added,
		Deleted:      deleted,
		Staged:       staged,
		Unstaged:     unstaged,
		Untracked:    untracked,
		ChangedFiles: changed,
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

func gitFiles(root string) []string {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return nil
	}
	parts := bytes.Split(bytes.TrimRight(out, "\x00"), []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			files = append(files, string(part))
		}
	}
	return files
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

func gitStatus(root string) (staged, unstaged, untracked int, changed []string) {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return 0, 0, 0, nil
	}
	seen := make(map[string]bool)
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
		file := strings.TrimSpace(line[3:])
		if idx := strings.LastIndex(file, " -> "); idx >= 0 {
			file = file[idx+4:]
		}
		if file != "" && !seen[file] {
			changed = append(changed, file)
			seen[file] = true
		}
	}
	return staged, unstaged, untracked, changed
}

func countTextLines(root string, files []string) int {
	total := 0
	for _, file := range files {
		if strings.Contains(file, "..") || filepath.IsAbs(file) {
			continue
		}
		path := filepath.Join(root, file)
		cleanRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		cleanPath, err := filepath.Abs(path)
		if err != nil || !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
			continue
		}
		data, err := readTextFile(cleanPath)
		if err != nil || bytes.Contains(data, []byte{0}) {
			continue
		}
		total += bytes.Count(data, []byte{'\n'})
		if len(data) > 0 && data[len(data)-1] != '\n' {
			total++
		}
	}
	return total
}

func readTextFile(path string) ([]byte, error) {
	return fs.ReadFile(os.DirFS(filepath.Dir(path)), filepath.Base(path))
}
