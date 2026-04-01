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

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/tmux"
	wsreg "github.com/miltonparedes/kitmux/internal/workspaces"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

type column int

const (
	colProjects column = iota
	colDetail
)

type dashMode int

const (
	modeNormal dashMode = iota
	modeFiltering
	modeProjectSearch
	modeNewBranch
	modeConfirm
	modeAgentPicker
)

const keyEnter = "enter"

// projectEntry represents a workspace in the left column.
type projectEntry struct {
	Name     string
	Path     string
	Active   bool
	Activity int64
}

// branchEntry represents a session or worktree in the right column.
type branchEntry struct {
	Name        string
	SessionName string
	Path        string
	Windows     int
	Attached    bool
	IsSession   bool
	DiffAdded   int
	DiffDel     int
}

// agentEntry represents a detected running agent or the launch action.
type agentEntry struct {
	Name        string
	AgentID     string
	SessionName string
	WindowIndex int
	PaneIndex   int
	IsLauncher  bool // "+ launch agent..." action
}

type Model struct {
	width  int
	height int

	focus column
	mode  dashMode

	// Left column — projects
	projects   []projectEntry
	projCursor int
	projScroll int

	// Right column — detail for selected project
	branches     []branchEntry
	agentEntries []agentEntry
	detailItems  int // len(branches) + len(agentEntries)
	detCursor    int
	detScroll    int

	// Diff stats cache
	stats map[string]sessionStats

	// All panes for agent detection
	panes []tmux.Pane

	// Session data for building detail
	sessions  []tmux.Session
	repoRoots map[string]string
	wtByPath  map[string][]worktree.Worktree

	// Filter (/)
	filter textinput.Model

	// Project search (n) — zoxide
	zoxide zoxidePicker

	// New branch input
	newBranch     textinput.Model
	newBranchProj projectEntry

	// Agent picker
	agentPicker agentPickerState

	// Confirm delete
	confirmName string
	confirmPath string
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

type agentPickerState struct {
	agents    []agents.Agent
	cursor    int
	modeIndex []int
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

	agentList := agents.DefaultAgents()
	return Model{
		stats:     make(map[string]sessionStats),
		filter:    fi,
		zoxide:    zoxidePicker{input: zi},
		newBranch: bi,
		agentPicker: agentPickerState{
			agents:    agentList,
			modeIndex: make([]int, len(agentList)),
		},
	}
}

func (m Model) Init() tea.Cmd {
	return loadData
}

// Messages

type dataLoadedMsg struct {
	projects  []projectEntry
	sessions  []tmux.Session
	repoRoots map[string]string
	wtByPath  map[string][]worktree.Worktree
	panes     []tmux.Pane
}

type statsLoadedMsg struct {
	stats map[string]sessionStats
}

type zoxideLoadedMsg struct {
	entries []zoxideEntry
}

type (
	actionDoneMsg struct{}
	switchDoneMsg struct{}
)

// Data loading

func loadData() tea.Msg {
	sess, _ := tmux.ListSessions()
	panes, _ := tmux.ListPanes()
	projs := wsreg.LoadRegistry()

	repoRoots := resolveRepoRoots(sess)

	activePaths := make(map[string]int64)
	for _, s := range sess {
		if root, ok := repoRoots[s.Name]; ok {
			if s.Activity > activePaths[root] {
				activePaths[root] = s.Activity
			}
		}
	}

	wsreg.SortWorkspaces(projs, activePaths)

	wtByPath := loadAllWorktrees(projs)

	entries := make([]projectEntry, len(projs))
	for i, p := range projs {
		act := activePaths[p.Path]
		entries[i] = projectEntry{
			Name:     p.Name,
			Path:     p.Path,
			Active:   act > 0,
			Activity: act,
		}
	}

	return dataLoadedMsg{
		projects:  entries,
		sessions:  sess,
		repoRoots: repoRoots,
		wtByPath:  wtByPath,
		panes:     panes,
	}
}

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

// Agent detection

func detectAgents(panes []tmux.Pane, projectPath string, repoRoots map[string]string, sessions []tmux.Session) []agentEntry {
	agentCommands := make(map[string]agents.Agent)
	for _, a := range agents.DefaultAgents() {
		agentCommands[a.Command] = a
	}

	sessPathMap := make(map[string]string, len(sessions))
	for _, s := range sessions {
		if root, ok := repoRoots[s.Name]; ok {
			sessPathMap[s.Name] = root
		}
	}

	var detected []agentEntry
	for _, p := range panes {
		root := sessPathMap[p.SessionName]
		if root != projectPath {
			continue
		}
		a, ok := agentCommands[p.Command]
		if !ok {
			continue
		}
		detected = append(detected, agentEntry{
			Name:        a.Name,
			AgentID:     a.ID,
			SessionName: p.SessionName,
			WindowIndex: p.WindowIndex,
			PaneIndex:   p.PaneIndex,
		})
	}

	detected = append(detected, agentEntry{
		Name:       "+ launch agent...",
		IsLauncher: true,
	})
	return detected
}

// Build detail for selected project

func (m *Model) rebuildDetail() {
	if len(m.projects) == 0 {
		m.branches = nil
		m.agentEntries = nil
		m.detailItems = 0
		return
	}

	proj := m.projects[m.projCursor]
	m.branches = m.buildBranches(proj)
	m.agentEntries = detectAgents(m.panes, proj.Path, m.repoRoots, m.sessions)
	m.detailItems = len(m.branches) + len(m.agentEntries)
	m.detCursor = 0
	m.detScroll = 0
}

func (m *Model) buildBranches(proj projectEntry) []branchEntry {
	var active []branchEntry
	sessionPaths := make(map[string]bool)

	for _, s := range m.sessions {
		root := m.repoRoots[s.Name]
		if root != proj.Path {
			continue
		}
		childName := trimPrefix(s.Name, proj.Name)
		if s.Path == proj.Path {
			if branch := resolveGitBranch(s.Path); branch != "" {
				childName = branch
			}
		}
		st := m.stats[s.Name]
		active = append(active, branchEntry{
			Name:        childName,
			SessionName: s.Name,
			Path:        s.Path,
			Windows:     s.Windows,
			Attached:    s.Attached,
			IsSession:   true,
			DiffAdded:   st.Added,
			DiffDel:     st.Deleted,
		})
		sessionPaths[s.Path] = true
	}

	sort.SliceStable(active, func(i, j int) bool {
		mi := isMainBranch(active[i].Name)
		mj := isMainBranch(active[j].Name)
		if mi != mj {
			return mi
		}
		return false
	})

	var inactive []branchEntry
	for _, wt := range m.wtByPath[proj.Path] {
		if sessionPaths[wt.Path] {
			continue
		}
		inactive = append(inactive, branchEntry{
			Name: wt.Branch,
			Path: wt.Path,
		})
	}

	sort.SliceStable(inactive, func(i, j int) bool {
		mi := isMainBranch(inactive[i].Name)
		mj := isMainBranch(inactive[j].Name)
		if mi != mj {
			return mi
		}
		return inactive[i].Name < inactive[j].Name
	})

	result := make([]branchEntry, 0, len(active)+len(inactive))
	result = append(result, active...)
	result = append(result, inactive...)
	return result
}

// Model methods

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case dataLoadedMsg:
		m.projects = msg.projects
		m.sessions = msg.sessions
		m.repoRoots = msg.repoRoots
		m.wtByPath = msg.wtByPath
		m.panes = msg.panes
		m.clampProjCursor()
		m.rebuildDetail()
		projs := wsreg.LoadRegistry()
		return m, loadStats(projs)

	case statsLoadedMsg:
		m.stats = msg.stats
		m.rebuildDetail()
		return m, nil

	case switchDoneMsg:
		return m, tea.Quit

	case actionDoneMsg:
		return m, loadData

	case zoxideLoadedMsg:
		m.zoxide.all = msg.entries
		m.zoxide.filtered = msg.entries
		m.zoxide.cursor = 0
		m.zoxide.scroll = 0
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
		case modeNewBranch:
			return m.handleNewBranch(msg)
		case modeAgentPicker:
			return m.handleAgentPicker(msg)
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
		if m.focus == colProjects {
			m.projCursor--
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		} else {
			m.detCursor--
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if m.focus == colProjects {
			m.projCursor++
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		} else {
			m.detCursor++
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.focus == colProjects {
			m.projCursor++
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		} else {
			m.detCursor++
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil

	case "k", "up":
		if m.focus == colProjects {
			m.projCursor--
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		} else {
			m.detCursor--
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil

	case "g", "home":
		if m.focus == colProjects {
			m.projCursor = 0
			m.projScroll = 0
			m.rebuildDetail()
		} else {
			m.detCursor = 0
			m.detScroll = 0
		}
		return m, nil

	case "G", "end":
		if m.focus == colProjects {
			m.projCursor = len(m.projects) - 1
			m.clampProjCursor()
			m.ensureProjVisible()
			m.rebuildDetail()
		} else {
			m.detCursor = m.detailItems - 1
			m.clampDetCursor()
			m.ensureDetVisible()
		}
		return m, nil

	case "l", "right":
		if m.focus == colProjects && m.detailItems > 0 {
			m.focus = colDetail
		}
		return m, nil

	case "h", "left":
		if m.focus == colDetail {
			m.focus = colProjects
		}
		return m, nil

	case keyEnter:
		if m.focus == colProjects {
			if m.detailItems > 0 {
				m.focus = colDetail
			}
			return m, nil
		}
		return m.activateDetailItem()

	case "/":
		m.mode = modeFiltering
		m.filter.SetValue("")
		m.filter.Focus()
		return m, textinput.Blink

	case "n":
		if m.focus == colProjects {
			m.mode = modeProjectSearch
			m.zoxide.input.SetValue("")
			m.zoxide.input.Focus()
			m.zoxide.all = nil
			m.zoxide.filtered = nil
			m.zoxide.cursor = 0
			m.zoxide.scroll = 0
			return m, tea.Batch(textinput.Blink, loadZoxide())
		}
		return m, nil

	case "d":
		if m.focus == colProjects && len(m.projects) > 0 {
			p := m.projects[m.projCursor]
			m.confirmName = p.Name
			m.confirmPath = p.Path
			m.mode = modeConfirm
		}
		return m, nil

	case "c":
		if m.focus == colDetail && len(m.projects) > 0 {
			m.newBranchProj = m.projects[m.projCursor]
			m.mode = modeNewBranch
			m.newBranch.SetValue("")
			m.newBranch.Focus()
			return m, textinput.Blink
		}
		return m, nil

	case "r":
		return m, loadData

	case "q", "esc":
		if m.focus == colDetail {
			m.focus = colProjects
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) activateDetailItem() (tea.Model, tea.Cmd) {
	if m.detCursor < len(m.branches) {
		b := m.branches[m.detCursor]
		if b.IsSession && b.SessionName != "" {
			return m, m.switchTo(b.SessionName)
		}
		if len(m.projects) > 0 {
			proj := m.projects[m.projCursor]
			return m, m.openWorktreeSession(proj.Name, b)
		}
		return m, nil
	}

	agentIdx := m.detCursor - len(m.branches)
	if agentIdx >= 0 && agentIdx < len(m.agentEntries) {
		ae := m.agentEntries[agentIdx]
		if ae.IsLauncher {
			m.mode = modeAgentPicker
			m.agentPicker.cursor = 0
			return m, nil
		}
		target := fmt.Sprintf("%s:%d.%d", ae.SessionName, ae.WindowIndex, ae.PaneIndex)
		return m, m.switchTo(target)
	}
	return m, nil
}

// Filter mode

func (m Model) handleFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.filter.SetValue("")
		return m, nil
	case keyEnter:
		m.mode = modeNormal
		m.filter.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
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
			return m, loadData
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

// New branch mode

func (m Model) handleNewBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case keyEnter:
		branch := m.newBranch.Value()
		if branch == "" {
			return m, nil
		}
		return m, m.createWorktreeAndOpen(m.newBranchProj.Name, m.newBranchProj.Path, branch)
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
		wsreg.RemoveWorkspace(m.confirmPath)
		return m, loadData
	default:
		m.mode = modeNormal
		return m, nil
	}
}

// Agent picker mode

func (m Model) handleAgentPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case "j", "down":
		if m.agentPicker.cursor < len(m.agentPicker.agents)-1 {
			m.agentPicker.cursor++
		}
		return m, nil
	case "k", "up":
		if m.agentPicker.cursor > 0 {
			m.agentPicker.cursor--
		}
		return m, nil
	case "tab":
		if len(m.agentPicker.agents) > 0 {
			c := m.agentPicker.cursor
			a := m.agentPicker.agents[c]
			m.agentPicker.modeIndex[c] = (m.agentPicker.modeIndex[c] + 1) % len(a.Modes)
		}
		return m, nil
	case keyEnter:
		return m, m.launchSelectedAgent()
	}
	return m, nil
}

func (m Model) launchSelectedAgent() tea.Cmd {
	if len(m.agentPicker.agents) == 0 || len(m.projects) == 0 {
		return nil
	}
	a := m.agentPicker.agents[m.agentPicker.cursor]
	mode := a.Modes[m.agentPicker.modeIndex[m.agentPicker.cursor]]
	proj := m.projects[m.projCursor]
	command := a.FullCommand(mode)

	return func() tea.Msg {
		sessName := proj.Name + "-" + a.ID
		sessName = uniqueSessName(sessName)
		_ = tmux.NewSessionInDir(sessName, proj.Path)
		_ = tmux.SendKeys(sessName, command)
		_ = tmux.SwitchClient(sessName)
		return switchDoneMsg{}
	}
}

// Actions

func (m Model) switchTo(name string) tea.Cmd {
	return func() tea.Msg {
		_ = tmux.SwitchClient(name)
		return switchDoneMsg{}
	}
}

func (m Model) openWorktreeSession(project string, b branchEntry) tea.Cmd {
	return func() tea.Msg {
		sessName := project + "-" + b.Name
		sessName = uniqueSessName(sessName)
		path := b.Path
		if path == "" {
			return actionDoneMsg{}
		}
		_ = tmux.NewSessionInDir(sessName, path)
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

func isMainBranch(name string) bool {
	n := normalize(name)
	return n == "main" || n == "master" ||
		strings.HasSuffix(n, "-main") || strings.HasSuffix(n, "-master")
}

func trimPrefix(sessionName, projectName string) string {
	norm := normalize(sessionName)
	normProj := normalize(projectName)
	if strings.HasPrefix(norm, normProj+"-") {
		return sessionName[len(normProj)+1:]
	}
	return sessionName
}

// Cursor helpers

func (m *Model) clampProjCursor() {
	if m.projCursor < 0 {
		m.projCursor = 0
	}
	if m.projCursor >= len(m.projects) {
		m.projCursor = len(m.projects) - 1
	}
	if m.projCursor < 0 {
		m.projCursor = 0
	}
}

func (m *Model) clampDetCursor() {
	if m.detCursor < 0 {
		m.detCursor = 0
	}
	if m.detCursor >= m.detailItems {
		m.detCursor = m.detailItems - 1
	}
	if m.detCursor < 0 {
		m.detCursor = 0
	}
}

func (m *Model) ensureProjVisible() {
	avail := m.contentHeight()
	if avail < 1 {
		avail = 1
	}
	if m.projCursor < m.projScroll {
		m.projScroll = m.projCursor
	}
	if m.projCursor >= m.projScroll+avail {
		m.projScroll = m.projCursor - avail + 1
	}
}

func (m *Model) ensureDetVisible() {
	avail := m.contentHeight()
	if avail < 1 {
		avail = 1
	}
	if m.detCursor < m.detScroll {
		m.detScroll = m.detCursor
	}
	if m.detCursor >= m.detScroll+avail {
		m.detScroll = m.detCursor - avail + 1
	}
}

func (m Model) contentHeight() int {
	// Height minus top border, bottom border, footer line
	h := m.height - 4
	if h < 1 {
		h = 1
	}
	return h
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

func (m Model) selectedProject() *projectEntry {
	if m.projCursor >= 0 && m.projCursor < len(m.projects) {
		return &m.projects[m.projCursor]
	}
	return nil
}
