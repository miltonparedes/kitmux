package sessions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// ZoxideEntry represents a directory entry returned by `zoxide query -ls`.
// The sessions view offers these as the target when the user invokes the
// "new session from recent directory" picker.
type ZoxideEntry struct {
	Score float64
	Path  string
	Short string // path relative to $HOME
}

// zoxidePicker is the state for the zoxide-backed directory picker used
// by the sessions view.
type zoxidePicker struct {
	all      []ZoxideEntry
	filtered []ZoxideEntry
	input    textinput.Model
	cursor   int
	scroll   int
}

func newZoxidePicker() zoxidePicker {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Search directory..."
	ti.CharLimit = 128
	ti.Focus()
	return zoxidePicker{input: ti}
}

type zoxideEntriesLoadedMsg struct {
	entries []ZoxideEntry
}

func loadZoxideEntries() tea.Msg {
	entries, err := queryZoxide()
	if err != nil {
		return zoxideEntriesLoadedMsg{}
	}
	return zoxideEntriesLoadedMsg{entries: entries}
}

func queryZoxide() ([]ZoxideEntry, error) {
	out, err := exec.Command("zoxide", "query", "-ls").Output()
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()
	var entries []ZoxideEntry
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

		entries = append(entries, ZoxideEntry{
			Score: score,
			Path:  path,
			Short: short,
		})
	}
	return entries, nil
}

func (p *zoxidePicker) setEntries(entries []ZoxideEntry) {
	p.all = entries
	p.filtered = entries
	p.cursor = 0
	p.scroll = 0
}

func (p *zoxidePicker) filter() {
	query := p.input.Value()
	if query == "" {
		p.filtered = p.all
	} else {
		shorts := make([]string, len(p.all))
		for i, e := range p.all {
			shorts[i] = e.Short
		}
		matches := fuzzy.Find(query, shorts)
		filtered := make([]ZoxideEntry, len(matches))
		for i, m := range matches {
			filtered[i] = p.all[m.Index]
		}
		p.filtered = filtered
	}
	p.cursor = 0
	p.scroll = 0
}

func (p *zoxidePicker) selected() *ZoxideEntry {
	if p.cursor >= 0 && p.cursor < len(p.filtered) {
		return &p.filtered[p.cursor]
	}
	return nil
}

func (p *zoxidePicker) clampCursor() {
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

func (p *zoxidePicker) ensureVisible(maxVisible int) {
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

// resolveWorkspace turns a directory path into a (session-name, working-dir)
// pair suitable for launching a tmux session. For git repos with multiple
// worktrees it prefers the main worktree and appends "-main" to the name.
func resolveWorkspace(dir string) (name, resolved string) {
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

// openZoxideEntry resolves the workspace and emits the message to create the
// session. If a session already exists at the same path it switches to it
// instead; on name collision a numeric suffix is appended.
func openZoxideEntry(entry ZoxideEntry) tea.Cmd {
	return func() tea.Msg {
		name, dir := resolveWorkspace(entry.Path)

		sessions, err := tmux.ListSessions()
		if err == nil {
			for _, s := range sessions {
				if s.Path == dir {
					return messages.SwitchSessionMsg{Name: s.Name}
				}
			}
		}

		if tmux.HasSession(name) {
			for i := 2; i <= 99; i++ {
				candidate := fmt.Sprintf("%s-%d", name, i)
				if !tmux.HasSession(candidate) {
					name = candidate
					break
				}
			}
		}

		return messages.CreateSessionInDirMsg{Name: name, Dir: dir}
	}
}
