package workspaces

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/tmux"
	wsdata "github.com/miltonparedes/kitmux/internal/workspaces/data"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// Model is the workspaces dashboard. It presents a two-column view: the
// left column lists registered workspaces; the right column shows the
// sessions, worktrees, and running agents for the selected workspace.
//
// IO is delegated to the internal/workspaces/data package so the Model
// itself is straightforward to test.
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

	// Diff stats cache (legacy view keyed by session name; still used by
	// some callers and tests). Prefer wsStats when available.
	stats map[string]sessionStats

	// Cached workspace-level stats keyed by workspace path.
	wsStats map[string]wsdata.WorkspaceStats

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

	// Transient status line (errors, hints).
	toast    string
	toastLvl toastLevel
	toastSeq int

	// Injected stats service; can be swapped in tests.
	stats_svc *wsdata.StatsService
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

// New builds a Model with default inputs and the production stats service.
func New() Model {
	return newWithService(wsdata.NewStatsService())
}

func newWithService(svc *wsdata.StatsService) Model {
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
		wsStats:   make(map[string]wsdata.WorkspaceStats),
		filter:    fi,
		zoxide:    zoxidePicker{input: zi},
		newBranch: bi,
		agentPicker: agentPickerState{
			agents:    agentList,
			modeIndex: make([]int, len(agentList)),
		},
		stats_svc: svc,
	}
}

func (m Model) Init() tea.Cmd {
	svc := m.stats_svc
	return loadDataCmd(svc)
}

// InitAddMode returns an Init command that also opens the zoxide "add workspace"
// picker once initial data has loaded.
func (m *Model) InitAddMode() tea.Cmd {
	m.mode = modeProjectSearch
	m.zoxide.input.SetValue("")
	m.zoxide.input.Focus()
	m.zoxide.all = nil
	m.zoxide.filtered = nil
	m.zoxide.cursor = 0
	m.zoxide.scroll = 0
	return tea.Batch(loadDataCmd(m.stats_svc), textinput.Blink, loadZoxide())
}

// SetSize is called by the host (app.Model or standalone program) on window
// size changes.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// IsEditing reports whether any input-capturing mode is active. Callers
// outside the view use this to decide whether a global shortcut should be
// intercepted.
func (m Model) IsEditing() bool {
	switch m.mode {
	case modeFiltering, modeProjectSearch, modeNewBranch, modeConfirm, modeAgentPicker:
		return true
	default:
		return false
	}
}

// Toast surfaces a transient status message for a few seconds. Used by the
// host after it dispatches actions that fail.
func (m *Model) Toast(text string, level toastLevel) tea.Cmd {
	return m.pushToast(text, level)
}

func (m *Model) pushToast(text string, level toastLevel) tea.Cmd {
	m.toast = text
	m.toastLvl = level
	m.toastSeq++
	seq := m.toastSeq
	return func() tea.Msg {
		time.Sleep(3 * time.Second)
		return toastClearMsg{seq: seq}
	}
}

func (m Model) selectedProject() *projectEntry {
	if m.projCursor >= 0 && m.projCursor < len(m.projects) {
		return &m.projects[m.projCursor]
	}
	return nil
}

// uniqueSessName is preserved at package scope for tests.
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
	return wsdata.IsMainBranch(name)
}

func normalize(name string) string {
	return wsdata.Normalize(name)
}

func resolveGitBranch(dir string) string {
	return wsdata.ResolveGitBranch(dir)
}

func trimPrefix(sessionName, projectName string) string {
	norm := normalize(sessionName)
	normProj := normalize(projectName)
	if len(norm) > len(normProj) && norm[:len(normProj)] == normProj && norm[len(normProj)] == '-' {
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
	h := m.height - 4
	if h < 1 {
		h = 1
	}
	return h
}
