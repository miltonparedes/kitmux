package workspaces

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/tmux"
	"github.com/miltonparedes/kitmux/internal/views/sessions"
	wsreg "github.com/miltonparedes/kitmux/internal/workspaces"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

type dashMode int

const (
	modeNormal dashMode = iota
	modeFiltering
	modeProjectSearch
	modeWorktreePicker
	modeNewBranch
	modeConfirm
)

const keyEnter = "enter"

type Model struct {
	width  int
	height int

	mode dashMode

	// Main tree
	roots     []*sessions.TreeNode
	visible   []*sessions.TreeNode
	allFlat   []*sessions.TreeNode // unfiltered flat list
	collapsed map[string]bool
	stats     map[string]sessionStats
	cursor    int
	scroll    int

	// Filter (/)
	filter textinput.Model

	// Project search (n) — zoxide
	zoxide zoxidePicker

	// Worktree picker
	wtPicker wtPicker

	// New branch input
	newBranch textinput.Model
}

type sessionStats struct {
	Added   int
	Deleted int
}

type zoxideEntry struct {
	Score float64
	Path  string
	Short string
}

type zoxidePicker struct {
	all      []zoxideEntry
	filtered []zoxideEntry
	input    textinput.Model
	cursor   int
	scroll   int
}

type wtEntry struct {
	Branch    string
	Path      string
	IsMain    bool
	HasSess   bool
	SessName  string
	Attached  bool
	DiffAdded int
	DiffDel   int
}

type wtPicker struct {
	project  string
	projPath string
	entries  []wtEntry
	cursor   int
}

func New() Model {
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.Placeholder = "filter..."
	fi.CharLimit = 64

	zi := textinput.New()
	zi.Prompt = "> "
	zi.Placeholder = "search project..."
	zi.CharLimit = 128

	bi := textinput.New()
	bi.Prompt = "Branch: "
	bi.Placeholder = "new-feature"
	bi.CharLimit = 128

	return Model{
		collapsed: make(map[string]bool),
		stats:     make(map[string]sessionStats),
		filter:    fi,
		zoxide:    zoxidePicker{input: zi},
		newBranch: bi,
	}
}

func (m Model) Init() tea.Cmd {
	return loadTree
}

// Messages

type treeLoadedMsg struct {
	roots []*sessions.TreeNode
}

type statsLoadedMsg struct {
	stats map[string]sessionStats
}

type zoxideLoadedMsg struct {
	entries []zoxideEntry
}

type wtLoadedMsg struct {
	project  string
	projPath string
	entries  []wtEntry
}

type (
	actionDoneMsg struct{}
	switchDoneMsg struct{}
)

// Data loading

func loadTree() tea.Msg {
	sess, _ := tmux.ListSessions()
	repoRoots := resolveRepoRoots(sess)

	projs := wsreg.LoadRegistry()
	wtByPath := loadAllWorktrees(projs)
	roots := buildProjectTree(projs, sess, repoRoots, wtByPath)
	return treeLoadedMsg{roots: roots}
}

// loadAllWorktrees loads worktree branches for each workspace.
// Falls back to resolveGitBranch for repos without wt support.
func loadAllWorktrees(projs []wsreg.Workspace) map[string][]worktree.Worktree {
	result := make(map[string][]worktree.Worktree, len(projs))
	for _, p := range projs {
		wts, err := worktree.ListInDir(p.Path)
		if err == nil && len(wts) > 0 {
			result[p.Path] = wts
			continue
		}
		if branch := resolveGitBranch(p.Path); branch != "" {
			result[p.Path] = []worktree.Worktree{
				{Branch: branch, Path: p.Path, IsMain: isMainBranch(branch)},
			}
		}
	}
	return result
}

func buildProjectTree(projs []wsreg.Workspace, sess []tmux.Session, repoRoots map[string]string, wtByPath map[string][]worktree.Worktree) []*sessions.TreeNode {
	activePaths := make(map[string]int64)
	for _, s := range sess {
		if root, ok := repoRoots[s.Name]; ok {
			if s.Activity > activePaths[root] {
				activePaths[root] = s.Activity
			}
		}
	}

	wsreg.SortWorkspaces(projs, activePaths)

	roots := make([]*sessions.TreeNode, 0, len(projs))
	for _, proj := range projs {
		group := &sessions.TreeNode{
			Kind:     sessions.KindGroupHeader,
			Name:     proj.Name,
			Path:     proj.Path,
			Expanded: true,
			Depth:    0,
		}

		// Collect active sessions for this workspace
		var activeChildren []*sessions.TreeNode
		sessionPaths := make(map[string]bool)
		for _, s := range sess {
			root := repoRoots[s.Name]
			if root != proj.Path {
				continue
			}
			childName := trimPrefix(s.Name, proj.Name)
			if s.Path == proj.Path {
				if branch := resolveGitBranch(s.Path); branch != "" {
					childName = branch
				}
			}
			activeChildren = append(activeChildren, &sessions.TreeNode{
				Kind:        sessions.KindSession,
				Name:        childName,
				SessionName: s.Name,
				Windows:     s.Windows,
				Attached:    s.Attached,
				Activity:    s.Activity,
				Depth:       1,
			})
			if s.Activity > group.Activity {
				group.Activity = s.Activity
			}
			sessionPaths[s.Path] = true
		}

		sort.SliceStable(activeChildren, func(i, j int) bool {
			mi := isMainBranch(activeChildren[i].Name)
			mj := isMainBranch(activeChildren[j].Name)
			if mi != mj {
				return mi
			}
			return activeChildren[i].Activity > activeChildren[j].Activity
		})

		// Add inactive worktree branches that don't have a running session
		var inactiveChildren []*sessions.TreeNode
		for _, wt := range wtByPath[proj.Path] {
			if sessionPaths[wt.Path] {
				continue
			}
			inactiveChildren = append(inactiveChildren, &sessions.TreeNode{
				Kind:  sessions.KindWorktree,
				Name:  wt.Branch,
				Path:  wt.Path,
				Depth: 1,
			})
		}

		sort.SliceStable(inactiveChildren, func(i, j int) bool {
			mi := isMainBranch(inactiveChildren[i].Name)
			mj := isMainBranch(inactiveChildren[j].Name)
			if mi != mj {
				return mi
			}
			return inactiveChildren[i].Name < inactiveChildren[j].Name
		})

		children := make([]*sessions.TreeNode, 0, len(activeChildren)+len(inactiveChildren))
		children = append(children, activeChildren...)
		children = append(children, inactiveChildren...)
		group.Children = children
		roots = append(roots, group)
	}
	return roots
}

func trimPrefix(sessionName, projectName string) string {
	norm := normalize(sessionName)
	normProj := normalize(projectName)
	if strings.HasPrefix(norm, normProj+"-") {
		return sessionName[len(normProj)+1:]
	}
	return sessionName
}

func isMainBranch(name string) bool {
	n := normalize(name)
	return n == "main" || n == "master" ||
		strings.HasSuffix(n, "-main") || strings.HasSuffix(n, "-master")
}

func loadStats(projs []wsreg.Workspace) tea.Cmd {
	return func() tea.Msg {
		sess, _ := tmux.ListSessions()
		pathToName := make(map[string]string, len(sess))
		for _, s := range sess {
			if s.Path != "" {
				pathToName[s.Path] = s.Name
			}
		}

		stats := make(map[string]sessionStats)
		for _, proj := range projs {
			wts, err := worktree.ListInDir(proj.Path)
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
		return statsLoadedMsg{stats: stats}
	}
}

func loadZoxide() tea.Cmd {
	return func() tea.Msg {
		entries, _ := queryZoxide()
		return zoxideLoadedMsg{entries: entries}
	}
}

func queryZoxide() ([]zoxideEntry, error) {
	out, err := exec.Command("zoxide", "query", "-ls").Output()
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()
	var entries []zoxideEntry
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
		entries = append(entries, zoxideEntry{Score: score, Path: path, Short: short})
	}
	return entries, nil
}

func loadWorktrees(project, projPath string) tea.Cmd {
	return func() tea.Msg {
		wts, err := worktree.ListInDir(projPath)
		if err != nil || len(wts) == 0 {
			return wtLoadedMsg{project: project, projPath: projPath}
		}

		sess, _ := tmux.ListSessions()
		sessMap := make(map[string]tmux.Session, len(sess))
		for _, s := range sess {
			if s.Path != "" {
				sessMap[s.Path] = s
			}
		}

		entries := make([]wtEntry, 0, len(wts))
		for _, wt := range wts {
			e := wtEntry{
				Branch:    wt.Branch,
				Path:      wt.Path,
				IsMain:    wt.IsMain,
				DiffAdded: wt.WorkingTree.Diff.Added,
				DiffDel:   wt.WorkingTree.Diff.Deleted,
			}
			if s, ok := sessMap[wt.Path]; ok {
				e.HasSess = true
				e.SessName = s.Name
				e.Attached = s.Attached
			}
			entries = append(entries, e)
		}

		// Main worktree first
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].IsMain != entries[j].IsMain {
				return entries[i].IsMain
			}
			return false
		})

		return wtLoadedMsg{project: project, projPath: projPath, entries: entries}
	}
}

// Model methods

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) applyCollapsedState() {
	for _, r := range m.roots {
		if m.collapsed[nodeKey(r)] {
			r.Expanded = false
		}
	}
}

func (m *Model) rebuildVisible() {
	m.allFlat = sessions.Flatten(m.roots)
	m.applyFilter()
}

func (m *Model) applyFilter() {
	if m.mode != modeFiltering || m.filter.Value() == "" {
		m.visible = m.allFlat
		return
	}

	query := m.filter.Value()
	names := make([]string, len(m.allFlat))
	for i, n := range m.allFlat {
		if n.SessionName != "" {
			names[i] = n.Name + " " + n.SessionName
		} else {
			names[i] = n.Name
		}
	}
	matches := fuzzy.Find(query, names)
	matched := make(map[int]bool, len(matches))
	for _, m := range matches {
		matched[m.Index] = true
	}

	// Include matched nodes and their parent groups
	parentOf := make(map[int]int, len(m.allFlat))
	lastGroup := -1
	for i, n := range m.allFlat {
		if n.Depth == 0 {
			lastGroup = i
		} else {
			parentOf[i] = lastGroup
		}
	}

	include := make(map[int]bool, len(m.allFlat))
	for idx := range matched {
		include[idx] = true
		if p, ok := parentOf[idx]; ok {
			include[p] = true
		}
	}

	filtered := make([]*sessions.TreeNode, 0, len(include))
	for i, n := range m.allFlat {
		if include[i] {
			filtered = append(filtered, n)
		}
	}
	m.visible = filtered
}

// Update

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case treeLoadedMsg:
		m.roots = msg.roots
		m.applyCollapsedState()
		m.rebuildVisible()
		m.clampCursor()
		projs := wsreg.LoadRegistry()
		return m, loadStats(projs)

	case statsLoadedMsg:
		m.stats = msg.stats
		return m, nil

	case switchDoneMsg:
		return m, tea.Quit

	case actionDoneMsg:
		return m, loadTree

	case zoxideLoadedMsg:
		m.zoxide.all = msg.entries
		m.zoxide.filtered = msg.entries
		m.zoxide.cursor = 0
		m.zoxide.scroll = 0
		return m, nil

	case wtLoadedMsg:
		m.wtPicker.project = msg.project
		m.wtPicker.projPath = msg.projPath
		m.wtPicker.entries = msg.entries
		m.wtPicker.cursor = 0
		if len(msg.entries) == 0 {
			// No worktrees — open project directly
			return m, m.openProjectDirect(msg.project, msg.projPath)
		}
		m.mode = modeWorktreePicker
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		switch m.mode {
		case modeConfirm:
			return m.handleConfirm(msg)
		case modeFiltering:
			return m.handleFilter(msg)
		case modeProjectSearch:
			return m.handleProjectSearch(msg)
		case modeWorktreePicker:
			return m.handleWorktreePicker(msg)
		case modeNewBranch:
			return m.handleNewBranch(msg)
		default:
			return m.handleKey(msg)
		}
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case tea.MouseButtonWheelDown:
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionRelease {
			return m, nil
		}
		row := msg.Y
		if row <= 4 {
			return m, nil
		}
		idx := m.rowToIndex(row - 5)
		if idx < 0 || idx >= len(m.visible) {
			return m, nil
		}
		m.cursor = idx
		return m.activateNode(m.visible[idx])
	}
	return m, nil
}

func (m Model) activateNode(node *sessions.TreeNode) (tea.Model, tea.Cmd) {
	if node.Depth == 0 {
		if len(node.Children) > 0 {
			node.Expanded = !node.Expanded
			m.collapsed[nodeKey(node)] = !node.Expanded
			m.rebuildVisible()
			m.clampCursor()
			return m, nil
		}
		// Inactive project — load worktrees
		return m, m.enterWorktreePicker(node.Path)
	}
	if node.Kind == sessions.KindSession && node.SessionName != "" {
		return m, m.switchTo(node.SessionName)
	}
	if node.Kind == sessions.KindWorktree {
		if parent := m.parentGroup(m.cursor); parent != nil {
			return m, m.openWorktreeSession(parent.Name, wtEntry{
				Branch: node.Name,
				Path:   node.Path,
			})
		}
	}
	return m, nil
}

func (m Model) parentGroup(idx int) *sessions.TreeNode {
	for i := idx - 1; i >= 0; i-- {
		if m.visible[i].Depth == 0 {
			return m.visible[i]
		}
	}
	return nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case "k", "up":
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case "J":
		m.jumpGroup(1)
		m.ensureVisible()
		return m, nil
	case "K":
		m.jumpGroup(-1)
		m.ensureVisible()
		return m, nil
	case "g", "home":
		m.cursor = 0
		m.scroll = 0
		return m, nil
	case "G", "end":
		m.cursor = len(m.visible) - 1
		m.ensureVisible()
		return m, nil

	case " ":
		if node := m.selected(); node != nil && len(node.Children) > 0 {
			node.Expanded = !node.Expanded
			m.collapsed[nodeKey(node)] = !node.Expanded
			m.rebuildVisible()
			m.clampCursor()
		}
		return m, nil

	case keyEnter:
		if node := m.selected(); node != nil {
			return m.activateNode(node)
		}
		return m, nil

	case "/":
		m.mode = modeFiltering
		m.filter.SetValue("")
		m.filter.Focus()
		return m, textinput.Blink

	case "n":
		m.mode = modeProjectSearch
		m.zoxide.input.SetValue("")
		m.zoxide.input.Focus()
		m.zoxide.all = nil
		m.zoxide.filtered = nil
		m.zoxide.cursor = 0
		m.zoxide.scroll = 0
		return m, tea.Batch(textinput.Blink, loadZoxide())

	case "d":
		if node := m.selected(); node != nil && node.Depth == 0 {
			m.mode = modeConfirm
		}
		return m, nil

	case "r":
		return m, loadTree

	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// Filter mode

func (m Model) handleFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.filter.SetValue("")
		m.visible = m.allFlat
		m.clampCursor()
		return m, nil
	case keyEnter:
		m.mode = modeNormal
		m.filter.Blur()
		m.clampCursor()
		return m, nil
	case "up":
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case "down":
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyFilter()
	m.cursor = 0
	m.scroll = 0
	return m, cmd
}

// Project search mode (zoxide)

func (m Model) handleProjectSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case keyEnter:
		if sel := m.zoxide.selected(); sel != nil {
			path := sel.Path
			name := filepath.Base(path)
			wsreg.AddWorkspace(name, path)
			m.mode = modeNormal
			return m, tea.Batch(loadTree, m.enterWorktreePicker(path))
		}
		return m, nil
	case "up", "ctrl+k":
		m.zoxide.cursor--
		m.zoxide.clampCursor()
		m.zoxide.ensureVisible(m.height - 4)
		return m, nil
	case "down", "ctrl+j":
		m.zoxide.cursor++
		m.zoxide.clampCursor()
		m.zoxide.ensureVisible(m.height - 4)
		return m, nil
	}
	var cmd tea.Cmd
	m.zoxide.input, cmd = m.zoxide.input.Update(msg)
	m.zoxide.filter()
	return m, cmd
}

// Worktree picker mode

func (m Model) handleWorktreePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case "j", "down":
		if m.wtPicker.cursor < len(m.wtPicker.entries)-1 {
			m.wtPicker.cursor++
		}
		return m, nil
	case "k", "up":
		if m.wtPicker.cursor > 0 {
			m.wtPicker.cursor--
		}
		return m, nil
	case keyEnter:
		if len(m.wtPicker.entries) == 0 {
			return m, nil
		}
		e := m.wtPicker.entries[m.wtPicker.cursor]
		if e.HasSess {
			return m, m.switchTo(e.SessName)
		}
		return m, m.openWorktreeSession(m.wtPicker.project, e)
	case "c":
		m.mode = modeNewBranch
		m.newBranch.SetValue("")
		m.newBranch.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

// New branch mode

func (m Model) handleNewBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeWorktreePicker
		return m, nil
	case keyEnter:
		branch := m.newBranch.Value()
		if branch == "" {
			return m, nil
		}
		return m, m.createWorktreeAndOpen(m.wtPicker.project, m.wtPicker.projPath, branch)
	}
	var cmd tea.Cmd
	m.newBranch, cmd = m.newBranch.Update(msg)
	return m, cmd
}

// Confirm mode

func (m Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.mode = modeNormal
		if node := m.selected(); node != nil && node.Depth == 0 {
			wsreg.RemoveWorkspace(node.Path)
			return m, loadTree
		}
		return m, nil
	default:
		m.mode = modeNormal
		return m, nil
	}
}

// Actions

func (m Model) enterWorktreePicker(projectPath string) tea.Cmd {
	projs := wsreg.LoadRegistry()
	for _, p := range projs {
		if p.Path == projectPath {
			return loadWorktrees(p.Name, p.Path)
		}
	}
	return nil
}

func (m Model) openProjectDirect(name, path string) tea.Cmd {
	return func() tea.Msg {
		sessName := name
		if isGitRepo(path) {
			sessName = name + "-main"
		}
		sessName = uniqueSessName(sessName)
		_ = tmux.NewSessionInDir(sessName, path)
		_ = tmux.SwitchClient(sessName)
		return switchDoneMsg{}
	}
}

func (m Model) openWorktreeSession(project string, e wtEntry) tea.Cmd {
	return func() tea.Msg {
		sessName := project + "-" + e.Branch
		sessName = uniqueSessName(sessName)
		_ = tmux.NewSessionInDir(sessName, e.Path)
		_ = tmux.SwitchClient(sessName)
		return switchDoneMsg{}
	}
}

func (m Model) createWorktreeAndOpen(project, projPath, branch string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("wt", "switch", "--create", branch)
		cmd.Dir = projPath
		if err := cmd.Run(); err != nil {
			return actionDoneMsg{}
		}

		wts, err := worktree.ListInDir(projPath)
		if err != nil {
			return actionDoneMsg{}
		}
		for _, wt := range wts {
			if wt.Branch == branch {
				sessName := uniqueSessName(project + "-" + branch)
				_ = tmux.NewSessionInDir(sessName, wt.Path)
				_ = tmux.SwitchClient(sessName)
				return switchDoneMsg{}
			}
		}
		return actionDoneMsg{}
	}
}

func (m Model) switchTo(name string) tea.Cmd {
	return func() tea.Msg {
		_ = tmux.SwitchClient(name)
		return switchDoneMsg{}
	}
}

func uniqueSessName(name string) string {
	if !tmux.HasSession(name) {
		return name
	}
	for i := 2; i <= 99; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if !tmux.HasSession(candidate) {
			return candidate
		}
	}
	return name
}

func isGitRepo(dir string) bool {
	return exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run() == nil
}

func nodeKey(node *sessions.TreeNode) string {
	if node == nil {
		return ""
	}
	if node.Path != "" {
		return node.Path
	}
	if node.SessionName != "" {
		return node.SessionName
	}
	return node.Name
}

func (m Model) selected() *sessions.TreeNode {
	if m.cursor >= 0 && m.cursor < len(m.visible) {
		return m.visible[m.cursor]
	}
	return nil
}

func (m *Model) jumpGroup(dir int) {
	if len(m.visible) == 0 {
		return
	}
	i := m.cursor + dir
	for i >= 0 && i < len(m.visible) {
		if m.visible[i].Depth == 0 {
			m.cursor = i
			return
		}
		i += dir
	}
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
	avail := m.height - 7
	if avail < 1 {
		avail = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	for {
		lines := m.linesFromScroll(m.cursor)
		if lines <= avail {
			break
		}
		m.scroll++
	}
}

func (m Model) linesFromScroll(idx int) int {
	if idx < m.scroll {
		return 0
	}
	lines := 0
	for i := m.scroll; i <= idx && i < len(m.visible); i++ {
		if i > m.scroll && m.visible[i].Depth == 0 {
			lines++
		}
		lines++
	}
	return lines
}

func (m Model) rowToIndex(treeRow int) int {
	y := 0
	for i := m.scroll; i < len(m.visible); i++ {
		if i > m.scroll && m.visible[i].Depth == 0 {
			if treeRow == y {
				return -1
			}
			y++
		}
		if treeRow == y {
			return i
		}
		y++
	}
	return -1
}

// Zoxide picker helpers

func (z *zoxidePicker) filter() {
	query := z.input.Value()
	if query == "" {
		z.filtered = z.all
	} else {
		shorts := make([]string, len(z.all))
		for i, e := range z.all {
			shorts[i] = e.Short
		}
		matches := fuzzy.Find(query, shorts)
		filtered := make([]zoxideEntry, len(matches))
		for i, m := range matches {
			filtered[i] = z.all[m.Index]
		}
		z.filtered = filtered
	}
	z.cursor = 0
	z.scroll = 0
}

func (z *zoxidePicker) selected() *zoxideEntry {
	if z.cursor >= 0 && z.cursor < len(z.filtered) {
		return &z.filtered[z.cursor]
	}
	return nil
}

func (z *zoxidePicker) clampCursor() {
	if z.cursor < 0 {
		z.cursor = 0
	}
	if z.cursor >= len(z.filtered) {
		z.cursor = len(z.filtered) - 1
	}
	if z.cursor < 0 {
		z.cursor = 0
	}
}

func (z *zoxidePicker) ensureVisible(maxVisible int) {
	if maxVisible < 1 {
		maxVisible = 1
	}
	if z.cursor < z.scroll {
		z.scroll = z.cursor
	}
	if z.cursor >= z.scroll+maxVisible {
		z.scroll = z.cursor - maxVisible + 1
	}
}
