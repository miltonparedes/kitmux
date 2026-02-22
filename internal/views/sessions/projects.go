package sessions

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// Project represents a directory entry from zoxide.
type Project struct {
	Score float64
	Path  string
	Short string // path relative to $HOME
}

// projectPicker is the state for the zoxide-based project selector.
type projectPicker struct {
	all      []Project
	filtered []Project
	input    textinput.Model
	cursor   int
	scroll   int
}

func newProjectPicker() projectPicker {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Search project..."
	ti.CharLimit = 128
	ti.Focus()
	return projectPicker{input: ti}
}

type projectsLoadedMsg struct {
	projects []Project
}

func loadProjects() tea.Msg {
	projects, err := queryZoxide()
	if err != nil {
		return projectsLoadedMsg{}
	}
	return projectsLoadedMsg{projects: projects}
}

func queryZoxide() ([]Project, error) {
	out, err := exec.Command("zoxide", "query", "-ls").Output()
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()
	var projects []Project
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		line = strings.TrimSpace(line)
		idx := strings.IndexByte(line, ' ')
		if idx < 0 {
			continue
		}
		scoreStr := line[:idx]
		path := strings.TrimSpace(line[idx+1:])

		score, _ := strconv.ParseFloat(scoreStr, 64)
		short := path
		if home != "" && strings.HasPrefix(path, home) {
			short = "~" + path[len(home):]
		}

		projects = append(projects, Project{
			Score: score,
			Path:  path,
			Short: short,
		})
	}
	return projects, nil
}

func (p *projectPicker) setProjects(projects []Project) {
	p.all = projects
	p.filtered = projects
	p.cursor = 0
	p.scroll = 0
}

func (p *projectPicker) filter() {
	query := p.input.Value()
	if query == "" {
		p.filtered = p.all
	} else {
		shorts := make([]string, len(p.all))
		for i, proj := range p.all {
			shorts[i] = proj.Short
		}
		matches := fuzzy.Find(query, shorts)
		filtered := make([]Project, len(matches))
		for i, m := range matches {
			filtered[i] = p.all[m.Index]
		}
		p.filtered = filtered
	}
	p.cursor = 0
	p.scroll = 0
}

func (p *projectPicker) selected() *Project {
	if p.cursor >= 0 && p.cursor < len(p.filtered) {
		return &p.filtered[p.cursor]
	}
	return nil
}

func (p *projectPicker) clampCursor() {
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *projectPicker) ensureVisible(maxVisible int) {
	if maxVisible < 1 {
		maxVisible = 1
	}
	if p.cursor < p.scroll {
		p.scroll = p.cursor
	}
	if p.cursor >= p.scroll+maxVisible {
		p.scroll = p.cursor - maxVisible + 1
	}
}

// resolveProject determines the session name and working directory for a project path.
// For git repos with worktrees, it finds the main worktree and appends "-main" to the name.
func resolveProject(dir string) (name, resolved string) {
	base := filepath.Base(dir)

	if !isGitRepo(dir) {
		return base, dir
	}

	wts, err := worktree.ListInDir(dir)
	if err != nil || len(wts) <= 1 {
		return base, dir
	}

	for _, wt := range wts {
		if wt.IsMain {
			return base + "-main", wt.Path
		}
	}

	return base, dir
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// openProject resolves the project and emits the message to create the session.
func openProject(proj Project) tea.Cmd {
	return func() tea.Msg {
		name, dir := resolveProject(proj.Path)
		return messages.CreateSessionInDirMsg{Name: name, Dir: dir}
	}
}
