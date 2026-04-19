package workspaces

import (
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
	"github.com/miltonparedes/kitmux/internal/workspaces/data"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

type column int

const (
	colWorkspaces column = iota
	colDetail
)

type dashMode int

const (
	modeNormal dashMode = iota
	modeFiltering
	modeWorkspaceSearch
	modeNewBranch
	modeNewBranchAgent
	modeAgentAttachChoice
	modeAttachBranchPicker
	modeConfirm
	modeAgentPicker
	modeActionPicker
	modeHelp
)

type confirmAction int

const (
	confirmActionNone confirmAction = iota
	confirmActionRemoveWorkspace
	confirmActionRemoveWorktree
)

type actionKind int

const (
	actionKindRemoveWorkspace actionKind = iota
	actionKindArchiveWorktree
	actionKindDeleteWorktree
)

type actionMenuItem struct {
	Label string
	Kind  actionKind
}

// agentTarget describes where the agent process should land within the
// target tmux session.
type agentTarget int

const (
	agentTargetWindow agentTarget = iota
	agentTargetSplit
)

// agentPickerIntent describes why the agent picker is open so handleAgentPicker
// can route the selection to the correct action on Enter.
type agentPickerIntent int

const (
	// agentIntentAttachBranch attaches the picked agent to a specific branch
	// (session may already exist or be created on the fly).
	agentIntentAttachBranch agentPickerIntent = iota
	// agentIntentNewWorktreeAgent runs after the "new worktree" flow and
	// launches the picked agent in window 0 of the freshly created session.
	agentIntentNewWorktreeAgent
)

const keyEnter = "enter"

// workspaceEntry represents a workspace in the left column.
type workspaceEntry struct {
	Name       string
	Path       string
	Active     bool
	Activity   int64
	Added      int
	Deleted    int
	Worktrees  int
	DirtyCount int
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
	IsMain      bool
	Staged      bool
	Modified    bool
	Untracked   bool
	Ahead       int
	Behind      int
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

// sessionStats is the legacy per-session diff view used by tests and renderers.
type sessionStats struct {
	Added   int
	Deleted int
}

// dataLoadedMsg is dispatched when the initial snapshot finishes loading.
type dataLoadedMsg struct {
	workspaces []workspaceEntry
	sessions   []tmux.Session
	repoRoots  map[string]string
	wtByPath   map[string][]worktree.Worktree
	panes      []tmux.Pane
	archived   map[string]map[string]bool
}

// statsLoadedMsg is dispatched when live worktree stats arrive from StatsService.
type statsLoadedMsg struct {
	stats    map[string]sessionStats
	wsStats  map[string]data.WorkspaceStats
	refresh  time.Time
	workPath string // "" for full reload, else single-workspace delta
}

// zoxideLoadedMsg delivers zoxide query results to the workspace picker.
type zoxideLoadedMsg struct {
	entries []zoxideEntry
}

// toastMsg is a transient status-line message.
type toastMsg struct {
	text  string
	level toastLevel
}

type toastLevel int

const (
	toastInfo toastLevel = iota
	toastWarn
	toastError
)

// toastClearMsg clears any active toast after its timeout expires.
type toastClearMsg struct {
	seq int
}

type (
	actionDoneMsg struct{}
	switchDoneMsg struct{}
)
