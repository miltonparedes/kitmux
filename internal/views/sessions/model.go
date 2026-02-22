package sessions

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// Model is the sessions tree view.
type Model struct {
	roots   []*TreeNode
	visible []*TreeNode // flattened visible nodes
	cursor  int
	scroll  int
	height  int
	width   int

	confirming  bool // kill confirmation
	renaming    bool
	renameInput textinput.Model
	searching   bool
	searchInput textinput.Model
	picking     bool // project picker active
	picker      projectPicker
}

func New() Model {
	ri := textinput.New()
	ri.Prompt = "Rename: "
	ri.CharLimit = 64

	si := textinput.New()
	si.Prompt = "/ "
	si.Placeholder = "search sessions..."
	si.CharLimit = 128

	return Model{
		renameInput: ri,
		searchInput: si,
		picker:      newProjectPicker(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadSessions
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// IsEditing returns true when the user is in an input mode (rename, confirm, picking).
func (m Model) IsEditing() bool {
	return m.confirming || m.renaming || m.searching || m.picking
}

// SelectedSessionName returns the session name under the cursor, or empty string.
func (m Model) SelectedSessionName() string {
	if node := m.selected(); node != nil && node.Kind == KindSession {
		return node.SessionName
	}
	return ""
}

type sessionStats struct {
	Added   int
	Deleted int
}

type sessionsLoadedMsg struct {
	sessions  []tmux.Session
	repoRoots map[string]string
}

type statsLoadedMsg struct {
	stats map[string]sessionStats
}

func (m Model) loadSessions() tea.Msg {
	sessions, err := tmux.ListSessions()
	if err != nil {
		return sessionsLoadedMsg{}
	}
	repoRoots := resolveRepoRoots(sessions)
	return sessionsLoadedMsg{sessions: sessions, repoRoots: repoRoots}
}

// resolveWorktreeStats collects diff stats from `wt list` for each unique repo root.
func resolveWorktreeStats(sessions []tmux.Session, repoRoots map[string]string) map[string]sessionStats {
	stats := make(map[string]sessionStats)

	// Collect unique repo roots
	roots := make(map[string]bool)
	for _, root := range repoRoots {
		roots[root] = true
	}

	// Build path→session name lookup
	pathToName := make(map[string]string)
	for _, s := range sessions {
		if s.Path != "" {
			pathToName[s.Path] = s.Name
		}
	}

	for root := range roots {
		wts, err := worktree.ListInDir(root)
		if err != nil {
			continue
		}
		for _, wt := range wts {
			name, ok := pathToName[wt.Path]
			if !ok {
				continue
			}
			d := wt.WorkingTree.Diff
			if d.Added > 0 || d.Deleted > 0 {
				stats[name] = sessionStats{Added: d.Added, Deleted: d.Deleted}
			}
		}
	}
	return stats
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsLoadedMsg:
		m.roots = BuildTree(msg.sessions, msg.repoRoots)
		m.visible = Flatten(m.roots)
		m.clampCursor()
		// Fire async stats load
		sessions := msg.sessions
		repoRoots := msg.repoRoots
		return m, tea.Batch(m.emitCursorChange(), func() tea.Msg {
			return statsLoadedMsg{stats: resolveWorktreeStats(sessions, repoRoots)}
		})

	case statsLoadedMsg:
		applyStats(m.roots, msg.stats)
		return m, nil

	case projectsLoadedMsg:
		m.picker.setProjects(msg.projects)
		return m, nil

	case tea.KeyMsg:
		if m.picking {
			return m.handlePicker(msg)
		}
		if m.searching {
			return m.handleSearch(msg)
		}
		if m.confirming {
			return m.handleConfirm(msg)
		}
		if m.renaming {
			return m.handleRename(msg)
		}
		return m.handleNormal(msg)
	}
	return m, nil
}

func applyStats(roots []*TreeNode, stats map[string]sessionStats) {
	if len(stats) == 0 {
		return
	}
	for _, r := range roots {
		if st, ok := stats[r.SessionName]; ok {
			r.Added, r.Deleted = st.Added, st.Deleted
		}
		for _, c := range r.Children {
			if st, ok := stats[c.SessionName]; ok {
				c.Added, c.Deleted = st.Added, st.Deleted
			}
		}
	}
}

func (m Model) handleNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	prevCursor := m.cursor

	switch msg.String() {
	case "j", "down":
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
	case "k", "up":
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
	case "g", "home":
		m.cursor = 0
		m.scroll = 0
	case "G", "end":
		m.cursor = len(m.visible) - 1
		m.ensureVisible()

	case "J": // next root/group
		for i := m.cursor + 1; i < len(m.visible); i++ {
			if m.visible[i].Depth == 0 {
				m.cursor = i
				break
			}
		}
		m.clampCursor()
		m.ensureVisible()

	case "K": // prev root/group
		for i := m.cursor - 1; i >= 0; i-- {
			if m.visible[i].Depth == 0 {
				m.cursor = i
				break
			}
		}
		m.clampCursor()
		m.ensureVisible()

	case "enter":
		if node := m.selected(); node != nil && node.Kind == KindSession {
			return m, func() tea.Msg {
				return messages.SwitchSessionMsg{Name: node.SessionName}
			}
		}

	case " ":
		if node := m.selected(); node != nil && len(node.Children) > 0 {
			node.Expanded = !node.Expanded
			m.visible = Flatten(m.roots)
			m.clampCursor()
		}

	case "/":
		m.searching = true
		m.searchInput.SetValue("")
		m.searchInput.Focus()
		m.filterSessions()
		return m, textinput.Blink

	case "d":
		if node := m.selected(); node != nil && node.Kind == KindSession {
			m.confirming = true
		}

	case "r":
		if node := m.selected(); node != nil && node.Kind == KindSession {
			m.renaming = true
			m.renameInput.SetValue(node.SessionName)
			m.renameInput.Focus()
			return m, textinput.Blink
		}

	case "n":
		m.picking = true
		m.picker.input.SetValue("")
		m.picker.input.Focus()
		return m, tea.Batch(textinput.Blink, loadProjects)

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		if msg.Alt && idx < len(m.visible) {
			m.cursor = idx
			m.ensureVisible()
		}
	}

	// Emit cursor change if cursor moved
	if m.cursor != prevCursor {
		return m, m.emitCursorChange()
	}
	return m, nil
}

func (m Model) handleConfirm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirming = false
		if node := m.selected(); node != nil && node.Kind == KindSession {
			name := node.SessionName
			return m, func() tea.Msg {
				_ = tmux.KillSession(name)
				// Reload sessions from tmux after kill
				sessions, _ := tmux.ListSessions()
				repoRoots := resolveRepoRoots(sessions)
				return sessionsLoadedMsg{sessions: sessions, repoRoots: repoRoots}
			}
		}
	case "n", "N", "esc":
		m.confirming = false
	}
	return m, nil
}

func (m Model) handleRename(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.renaming = false
		if node := m.selected(); node != nil {
			old := node.SessionName
			newName := m.renameInput.Value()
			if newName != "" && newName != old {
				return m, func() tea.Msg {
					_ = tmux.RenameSession(old, newName)
					return m.loadSessions()
				}
			}
		}
		return m, nil
	case "esc":
		m.renaming = false
		return m, nil
	}
	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m Model) handlePicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if proj := m.picker.selected(); proj != nil {
			m.picking = false
			return m, openProject(*proj)
		}
		return m, nil
	case "esc":
		m.picking = false
		return m, nil
	case "up", "ctrl+k":
		m.picker.cursor--
		m.picker.clampCursor()
		m.picker.ensureVisible(m.pickerMaxVisible())
		return m, nil
	case "down", "ctrl+j":
		m.picker.cursor++
		m.picker.clampCursor()
		m.picker.ensureVisible(m.pickerMaxVisible())
		return m, nil
	}

	var cmd tea.Cmd
	m.picker.input, cmd = m.picker.input.Update(msg)
	m.picker.filter()
	return m, cmd
}

func (m Model) handleSearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if node := m.selected(); node != nil && node.Kind == KindSession {
			m.searching = false
			m.visible = Flatten(m.roots)
			m.clampCursor()
			return m, func() tea.Msg {
				return messages.SwitchSessionMsg{Name: node.SessionName}
			}
		}
		return m, nil
	case "esc":
		m.searching = false
		m.visible = Flatten(m.roots)
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case "up", "ctrl+k":
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case "down", "ctrl+j":
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.filterSessions()
	return m, cmd
}

func (m *Model) filterSessions() {
	allRoots := expandedCopy(m.roots)
	allNodes := Flatten(allRoots)

	// Collect only session nodes
	var sessions []*TreeNode
	for _, n := range allNodes {
		if n.Kind == KindSession {
			sessions = append(sessions, n)
		}
	}

	query := m.searchInput.Value()
	if query == "" {
		// Show all sessions flat
		flat := make([]*TreeNode, len(sessions))
		for i, s := range sessions {
			flat[i] = &TreeNode{
				Kind:        KindSession,
				Name:        s.SessionName,
				SessionName: s.SessionName,
				Windows:     s.Windows,
				Attached:    s.Attached,
				Depth:       0,
			}
		}
		m.visible = flat
	} else {
		names := make([]string, len(sessions))
		for i, s := range sessions {
			names[i] = s.SessionName
		}
		matches := fuzzy.Find(query, names)
		flat := make([]*TreeNode, len(matches))
		for i, match := range matches {
			s := sessions[match.Index]
			flat[i] = &TreeNode{
				Kind:        KindSession,
				Name:        s.SessionName,
				SessionName: s.SessionName,
				Windows:     s.Windows,
				Attached:    s.Attached,
				Depth:       0,
			}
		}
		m.visible = flat
	}
	m.cursor = 0
	m.scroll = 0
}

// expandedCopy returns a shallow copy of roots with all nodes expanded,
// so Flatten returns every node including children of collapsed groups.
func expandedCopy(roots []*TreeNode) []*TreeNode {
	out := make([]*TreeNode, len(roots))
	for i, r := range roots {
		cp := *r
		cp.Expanded = true
		out[i] = &cp
	}
	return out
}

func (m Model) pickerMaxVisible() int {
	avail := m.height - 4 // input + separator + footer separator + help
	if avail < 1 {
		avail = 1
	}
	return (avail + 1) / 2
}

func (m Model) selected() *TreeNode {
	if m.cursor >= 0 && m.cursor < len(m.visible) {
		return m.visible[m.cursor]
	}
	return nil
}

func (m *Model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) ensureVisible() {
	// Each item takes 2 lines (item + separator), last takes 1.
	// Available lines = height - 2 (footer sep + help)
	avail := m.height - 2
	if avail < 1 {
		avail = 1
	}
	maxVisible := (avail + 1) / 2
	if maxVisible < 1 {
		maxVisible = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+maxVisible {
		m.scroll = m.cursor - maxVisible + 1
	}
}

func (m Model) emitCursorChange() tea.Cmd {
	name := m.SelectedSessionName()
	if name == "" {
		return nil
	}
	return func() tea.Msg {
		return messages.SessionCursorMsg{SessionName: name}
	}
}

// Reload triggers a session reload from tmux.
func (m Model) Reload() tea.Cmd {
	return m.loadSessions
}
