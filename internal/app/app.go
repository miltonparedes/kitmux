package app

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/config"
	"github.com/miltonparedes/kitmux/internal/openlocal"
	"github.com/miltonparedes/kitmux/internal/recency"
	"github.com/miltonparedes/kitmux/internal/tmux"
	agentabview "github.com/miltonparedes/kitmux/internal/views/agentab"
	agentsview "github.com/miltonparedes/kitmux/internal/views/agents"
	"github.com/miltonparedes/kitmux/internal/views/palette"
	"github.com/miltonparedes/kitmux/internal/views/sessions"
	"github.com/miltonparedes/kitmux/internal/views/windows"
	workspacesview "github.com/miltonparedes/kitmux/internal/views/workspaces"
	"github.com/miltonparedes/kitmux/internal/views/worktrees"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// Mode determines how kitmux starts.
type Mode int

const (
	ModeSessions   Mode = iota // Session tree view
	ModePalette                // Command palette-only mode
	ModeWorktrees              // Worktrees view
	ModeAgents                 // Agents view
	ModeWindows                // Windows for current session
	ModeRun                    // Execute a palette command directly
	ModeWorkspaces             // Workspaces dashboard
)

type activeView int

const (
	viewSessions   activeView = iota
	viewWindows               // Windows drill-down
	viewWorktrees             // Worktree list
	viewAgents                // Agent launcher
	viewAgentAB               // A/B launcher form
	viewWorkspaces            // Workspaces dashboard
)

type Model struct {
	mode           Mode
	view           activeView
	sessions       sessions.Model
	windows        windows.Model
	worktreeView   worktrees.Model
	agentsView     agentsview.Model
	agentABView    agentabview.Model
	workspacesView workspacesview.Model
	palette        palette.Model
	paletteActive  bool
	paletteReturn  bool        // return to palette after sub-action completes
	returnView     activeView  // view to return to from transient forms
	pendingKey     *tea.KeyMsg // key to inject after sessions load
	width          int
	height         int
	runCommandID   string // for ModeRun: the command to execute
}

func New(mode Mode, opts ...Option) Model {
	m := Model{
		mode:           mode,
		view:           viewSessions,
		sessions:       sessions.New(),
		windows:        windows.New(),
		worktreeView:   worktrees.New(),
		agentsView:     agentsview.New(),
		agentABView:    agentabview.New(),
		workspacesView: workspacesview.New(),
		palette:        palette.New(),
	}
	for _, opt := range opts {
		opt(&m)
	}
	switch mode {
	case ModePalette:
		m.paletteActive = true
		m.palette.Reset()
	case ModeWorktrees:
		m.view = viewWorktrees
	case ModeAgents:
		m.view = viewAgents
	case ModeWindows:
		m.view = viewWindows
	case ModeWorkspaces:
		m.view = viewWorkspaces
	}
	return m
}

// Option configures a Model.
type Option func(*Model)

// WithRunCommand sets the command ID for ModeRun.
func WithRunCommand(id string) Option {
	return func(m *Model) {
		m.runCommandID = id
	}
}

func (m Model) Init() tea.Cmd {
	switch m.mode {
	case ModeRun:
		id := m.runCommandID
		return func() tea.Msg {
			return messages.ExecuteCommandMsg{ID: id}
		}
	case ModeWindows:
		return m.initCurrentSessionWindows()
	default:
		switch m.view {
		case viewWorktrees:
			return m.worktreeView.Init()
		case viewAgents:
			return m.agentsView.Init()
		case viewAgentAB:
			return m.agentABView.Init()
		case viewWorkspaces:
			return m.workspacesView.Init()
		default:
			return m.sessions.Init()
		}
	}
}

func (m Model) initCurrentSessionWindows() tea.Cmd {
	return func() tea.Msg {
		name, err := tmux.CurrentSession()
		if err != nil || name == "" {
			return tea.QuitMsg{}
		}
		return messages.DrillWindowsMsg{SessionName: name}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.dispatchInput(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.dispatchNavigation(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.dispatchAction(msg); handled {
		return updated, cmd
	}
	return m.routeToView(msg)
}

// dispatchInput handles low-level input messages (window/mouse/keys) and the
// palette toggle. Returns handled=false if the caller should keep dispatching.
func (m Model) dispatchInput(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.applyWindowSize(msg), nil, true
	case tea.MouseMsg:
		if m.paletteActive {
			var cmd tea.Cmd
			m.palette, cmd = m.palette.Update(msg)
			return m, cmd, true
		}
	case tea.KeyMsg:
		if updated, cmd, handled := m.handleKeyMsg(msg); handled {
			return updated, cmd, true
		}
	case messages.TogglePaletteMsg:
		return m.handleTogglePalette()
	case messages.ExecuteCommandMsg:
		m.paletteActive = false
		updated, cmd := m.executeCommand(msg.ID)
		return updated, cmd, true
	}
	return m, nil, false
}

// dispatchNavigation handles session/window/view navigation messages.
func (m Model) dispatchNavigation(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case messages.SwitchSessionMsg:
		_ = tmux.SwitchClient(msg.Name)
		return m, tea.Quit, true
	case messages.SwitchWindowMsg:
		_ = tmux.SwitchClient(msg.Target)
		return m, tea.Quit, true
	case messages.DrillWindowsMsg:
		m.view = viewWindows
		return m, m.windows.LoadSession(msg.SessionName), true
	case messages.BackToSessionsMsg:
		if m.paletteReturn {
			return m, m.returnToPalette(), true
		}
		m.view = viewSessions
		return m, nil, true
	case messages.SessionCursorMsg:
		return m, nil, true
	case messages.ReloadSessionsMsg:
		m.view = viewSessions
		return m, m.sessions.Reload(), true
	case messages.SwitchViewMsg:
		return m.handleSwitchView(msg)
	case messages.OpenWorkspacesMsg:
		return m.handleOpenWorkspaces(msg)
	}
	return m, nil, false
}

// dispatchAction handles side-effect messages (create/kill/launch/popup/editor).
func (m Model) dispatchAction(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case messages.CreateSessionInDirMsg:
		_ = tmux.NewSessionInDir(msg.Name, msg.Dir)
		_ = tmux.SwitchClient(msg.Name)
		return m, tea.Quit, true
	case messages.SwitchWorktreeMsg:
		_ = worktree.SwitchTo(msg.Branch)
		return m, tea.Quit, true
	case messages.CreateWorktreeMsg:
		_ = worktree.Create(msg.Branch)
		return m, tea.Quit, true
	case messages.RemoveWorktreeMsg:
		_ = worktree.Remove(msg.Branch)
		return m, m.worktreeView.Reload(), true
	case messages.ReloadWorktreesMsg:
		return m, m.worktreeView.Reload(), true
	case messages.LaunchAgentMsg:
		updated, cmd := m.launchAgent(msg)
		return updated, cmd, true
	case messages.OpenAgentABMsg:
		m.returnView = m.view
		m.view = viewAgentAB
		m.agentABView.Reset()
		return m, m.agentABView.Init(), true
	case messages.BackFromAgentABMsg:
		return m.handleBackFromAgentAB()
	case messages.LaunchAgentABMsg:
		updated, cmd := m.launchAgentAB(msg)
		return updated, cmd, true
	case messages.RunPopupMsg:
		_ = tmux.DisplayPopup(msg.Command, msg.Width, msg.Height)
		return m, tea.Quit, true
	case messages.OpenLocalEditorMsg:
		return m.handleOpenLocalEditor(msg)
	}
	return m, nil, false
}

func (m Model) applyWindowSize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	m.sessions.SetSize(m.width, m.height-1)
	m.windows.SetSize(m.width, m.height-1)
	m.worktreeView.SetSize(m.width, m.height-1)
	m.agentsView.SetSize(m.width, m.height-1)
	m.agentABView.SetSize(m.width, m.height-1)
	m.workspacesView.SetSize(m.width, m.height-1)
	m.palette.SetSize(m.width, m.height)
	return m
}

func (m Model) handleTogglePalette() (tea.Model, tea.Cmd, bool) {
	if m.mode == ModePalette {
		return m, tea.Quit, true
	}
	m.paletteActive = !m.paletteActive
	if m.paletteActive {
		m.palette.Reset()
	}
	return m, nil, true
}

func (m Model) handleSwitchView(msg messages.SwitchViewMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.View {
	case "sessions":
		if m.paletteReturn {
			return m, m.returnToPalette(), true
		}
		m.view = viewSessions
		return m, nil, true
	case "worktrees":
		m.view = viewWorktrees
		return m, m.worktreeView.Init(), true
	case "agents":
		m.view = viewAgents
		return m, nil, true
	}
	return m, nil, true
}

func (m Model) handleOpenWorkspaces(msg messages.OpenWorkspacesMsg) (tea.Model, tea.Cmd, bool) {
	m.view = viewWorkspaces
	m.workspacesView = workspacesview.New()
	m.workspacesView.SetSize(m.width, m.height-1)
	if msg.AddMode {
		return m, m.workspacesView.InitAddMode(), true
	}
	return m, m.workspacesView.Init(), true
}

func (m Model) handleBackFromAgentAB() (tea.Model, tea.Cmd, bool) {
	if m.paletteReturn {
		return m, m.returnToPalette(), true
	}
	m.view = m.returnView
	if m.view == viewAgentAB {
		m.view = viewSessions
	}
	return m, nil, true
}

func (m Model) handleOpenLocalEditor(msg messages.OpenLocalEditorMsg) (tea.Model, tea.Cmd, bool) {
	if msg.Err != nil {
		_ = tmux.DisplayMessage(fmt.Sprintf("open_local_editor error: %v", msg.Err))
		return m, tea.Quit, true
	}
	if msg.Fallback != "" {
		_ = tmux.DisplayMessage(fmt.Sprintf("bridge unavailable, run manually: %s", msg.Fallback))
		return m, tea.Quit, true
	}
	_ = tmux.DisplayMessage("opened local editor")
	return m, tea.Quit, true
}

// handleKeyMsg processes keyboard input.
// Returns (model, cmd, handled); when handled is false the caller
// should route the original message to the active view.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	if m.paletteActive {
		return m.handlePaletteKey(msg)
	}

	isEditing := m.isEditing()
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit, true
	case "q":
		return m.handleQuitKey(isEditing)
	case "ctrl+p":
		return m.handleOpenPaletteKey(isEditing)
	case "w":
		return m.handleWorktreeKey(isEditing)
	case "a":
		return m.handleAgentsKey(isEditing)
	case "esc":
		return m.handleEscKey(msg, isEditing)
	}
	return m, nil, false
}

func (m Model) handlePaletteKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "esc":
		if m.mode == ModePalette {
			return m, tea.Quit, true
		}
		m.paletteActive = false
		return m, nil, true
	case "ctrl+c":
		return m, tea.Quit, true
	}
	var cmd tea.Cmd
	m.palette, cmd = m.palette.Update(msg)
	return m, cmd, true
}

func (m Model) isEditing() bool {
	if m.sessions.IsEditing() || m.worktreeView.IsEditing() {
		return true
	}
	if m.view == viewAgentAB && m.agentABView.IsEditing() {
		return true
	}
	if m.view == viewWorkspaces && m.workspacesView.IsEditing() {
		return true
	}
	return false
}

func (m Model) handleQuitKey(isEditing bool) (Model, tea.Cmd, bool) {
	if isEditing {
		return m, nil, false
	}
	if m.view == viewWorkspaces {
		return m, nil, false
	}
	return m, tea.Quit, true
}

func (m Model) handleOpenPaletteKey(isEditing bool) (Model, tea.Cmd, bool) {
	if isEditing {
		return m, nil, false
	}
	m.paletteActive = true
	m.palette.Reset()
	return m, nil, true
}

func (m Model) handleWorktreeKey(isEditing bool) (Model, tea.Cmd, bool) {
	if isEditing || m.view == viewWorktrees || m.view == viewWorkspaces {
		return m, nil, false
	}
	m.view = viewWorktrees
	return m, m.worktreeView.Init(), true
}

func (m Model) handleAgentsKey(isEditing bool) (Model, tea.Cmd, bool) {
	if isEditing || m.view == viewAgents || m.view == viewWorkspaces {
		return m, nil, false
	}
	m.view = viewAgents
	return m, nil, true
}

// handleEscKey implements esc semantics. Workspaces owns its own esc handling
// (detail→projects back-nav, then quit), so it is delegated first.
func (m Model) handleEscKey(msg tea.KeyMsg, isEditing bool) (Model, tea.Cmd, bool) {
	if m.view == viewWorkspaces && !m.workspacesView.IsEditing() {
		return m.handleEscWorkspaces(msg)
	}
	if m.paletteReturn && !isEditing {
		return m, m.returnToPalette(), true
	}
	return m.handleEscByView()
}

func (m Model) handleEscWorkspaces(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	model, cmd := m.workspacesView.Update(msg)
	m.workspacesView = model.(workspacesview.Model)
	if cmd == nil {
		return m, nil, true
	}
	if m.paletteReturn {
		return m, m.returnToPalette(), true
	}
	if m.mode == ModeWorkspaces {
		return m, cmd, true
	}
	m.view = viewSessions
	return m, nil, true
}

func (m Model) handleEscByView() (Model, tea.Cmd, bool) {
	switch m.view {
	case viewWindows:
		return m.escWithMode(ModeWindows)
	case viewWorktrees:
		return m.escWithMode(ModeWorktrees)
	case viewAgents:
		return m.escWithMode(ModeAgents)
	case viewAgentAB:
		return m, func() tea.Msg { return messages.BackFromAgentABMsg{} }, true
	case viewWorkspaces:
		// IsEditing() path: let the view cancel its own modal.
		return m, nil, false
	default:
		if m.sessions.IsEditing() {
			return m, nil, false
		}
		return m, tea.Quit, true
	}
}

func (m Model) escWithMode(startMode Mode) (Model, tea.Cmd, bool) {
	if m.mode == startMode {
		return m, tea.Quit, true
	}
	m.view = viewSessions
	return m, nil, true
}

// routeToView forwards a message to the active view.
func (m Model) routeToView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.view {
	case viewSessions:
		m.sessions, cmd = m.sessions.Update(msg)
		if m.pendingKey != nil && m.sessions.ConsumeLoaded() {
			km := *m.pendingKey
			m.pendingKey = nil
			var cmd2 tea.Cmd
			m.sessions, cmd2 = m.sessions.Update(km)
			cmd = tea.Batch(cmd, cmd2)
		}
		if m.paletteReturn && !m.sessions.IsEditing() {
			if cmd != nil {
				return m, cmd
			}
			return m, m.returnToPalette()
		}
	case viewWindows:
		m.windows, cmd = m.windows.Update(msg)
	case viewWorktrees:
		m.worktreeView, cmd = m.worktreeView.Update(msg)
		if m.paletteReturn && !m.worktreeView.IsEditing() {
			if cmd != nil {
				return m, cmd
			}
			return m, m.returnToPalette()
		}
	case viewAgents:
		m.agentsView, cmd = m.agentsView.Update(msg)
	case viewAgentAB:
		m.agentABView, cmd = m.agentABView.Update(msg)
	case viewWorkspaces:
		var model tea.Model
		model, cmd = m.workspacesView.Update(msg)
		m.workspacesView = model.(workspacesview.Model)
	}
	return m, cmd
}

func (m Model) View() string {
	if m.paletteActive {
		return m.palette.View()
	}

	switch m.view {
	case viewWindows:
		return m.windows.View()
	case viewWorktrees:
		return m.worktreeView.View()
	case viewAgents:
		return m.agentsView.View()
	case viewAgentAB:
		return m.agentABView.View()
	case viewWorkspaces:
		return m.workspacesView.View()
	default:
		return m.sessions.View()
	}
}

// returnToPalette reactivates the palette or quits if in a terminal mode.
// Must be called on the m being returned (value receiver — mutations stay local).
func (m *Model) returnToPalette() tea.Cmd {
	m.paletteReturn = false
	if m.mode == ModePalette || m.mode == ModeRun {
		return tea.Quit
	}
	m.paletteActive = true
	m.palette.Reset()
	return nil
}

func (m Model) executeCommand(id string) (tea.Model, tea.Cmd) {
	recency.RecordCommand(id)
	m.paletteReturn = true

	if updated, cmd, handled := m.execSessionCommand(id); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.execWorktreeCommand(id); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.execAgentCommand(id); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.execEditorCommand(id); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.execToolCommand(id); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.execViewCommand(id); handled {
		return updated, cmd
	}
	return m, nil
}

func (m Model) execSessionCommand(id string) (tea.Model, tea.Cmd, bool) {
	switch id {
	case "switch_session":
		m.paletteReturn = false
		m.view = viewSessions
		return m, m.sessions.Reload(), true
	case "open_workspace":
		return m, openWorkspacesCmd(false), true
	case "add_workspace":
		return m, openWorkspacesCmd(true), true
	case "kill_session":
		return m.sessionsWithPendingRune('d'), m.sessions.Reload(), true
	case "kill_current_session":
		return m, killCurrentSessionCmd(), true
	case "rename_session":
		return m.sessionsWithPendingRune('r'), m.sessions.Reload(), true
	}
	return m, nil, false
}

func (m Model) execWorktreeCommand(id string) (tea.Model, tea.Cmd, bool) {
	switch id {
	case "wt_switch":
		m.view = viewWorktrees
		return m, m.worktreeView.Init(), true
	case "wt_create":
		return m.worktreesWithInjectedRune('n')
	case "wt_create_describe":
		return m.worktreesWithInjectedRune('N')
	case "wt_remove":
		return m.worktreesWithInjectedRune('d')
	case "wt_merge":
		return m, popupCmd("wt merge", "80%", "80%"), true
	case "wt_commit":
		return m, popupCmd("wt step commit", "80%", "80%"), true
	}
	return m, nil, false
}

func (m Model) execAgentCommand(id string) (tea.Model, tea.Cmd, bool) {
	switch id {
	case "launch_claude":
		return m, launchAgentCmd("claude"), true
	case "launch_gemini":
		return m, launchAgentCmd("gemini"), true
	case "launch_codex":
		return m, launchAgentCmd("codex"), true
	case "launch_aichat":
		return m, launchAgentCmd("aichat"), true
	case "launch_opencode":
		return m, launchAgentCmd("opencode"), true
	case "agent_ab":
		return m, func() tea.Msg { return messages.OpenAgentABMsg{Source: "palette"} }, true
	}
	return m, nil, false
}

func (m Model) execEditorCommand(id string) (tea.Model, tea.Cmd, bool) {
	if id == "open_local_editor" {
		return m, openLocalEditorCmd(), true
	}
	return m, nil, false
}

func (m Model) execToolCommand(id string) (tea.Model, tea.Cmd, bool) {
	switch id {
	case "tool_lazygit":
		return m, popupCmd("lazygit", "100%", "100%"), true
	case "tool_lumen_diff":
		return m, popupCmd("lumen diff", "100%", "100%"), true
	}
	return m, nil, false
}

func (m Model) execViewCommand(id string) (tea.Model, tea.Cmd, bool) {
	switch id {
	case "view_sessions":
		m.view = viewSessions
		return m, nil, true
	case "view_worktrees":
		m.view = viewWorktrees
		return m, m.worktreeView.Init(), true
	case "view_agents":
		m.view = viewAgents
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) sessionsWithPendingRune(r rune) Model {
	m.view = viewSessions
	km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
	m.pendingKey = &km
	return m
}

func (m Model) worktreesWithInjectedRune(r rune) (tea.Model, tea.Cmd, bool) {
	m.view = viewWorktrees
	cmd := m.worktreeView.Init()
	km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
	var cmd2 tea.Cmd
	m.worktreeView, cmd2 = m.worktreeView.Update(km)
	return m, tea.Batch(cmd, cmd2), true
}

func openWorkspacesCmd(addMode bool) tea.Cmd {
	return func() tea.Msg {
		return messages.OpenWorkspacesMsg{AddMode: addMode}
	}
}

func popupCmd(command, width, height string) tea.Cmd {
	return func() tea.Msg {
		return messages.RunPopupMsg{Command: command, Width: width, Height: height}
	}
}

func launchAgentCmd(agentID string) tea.Cmd {
	return func() tea.Msg {
		return messages.LaunchAgentMsg{AgentID: agentID, ModeID: "default", Target: "pane"}
	}
}

func killCurrentSessionCmd() tea.Cmd {
	return func() tea.Msg {
		current, err := tmux.CurrentSession()
		if err != nil || current == "" {
			return tea.QuitMsg{}
		}
		sessionList, _ := tmux.ListSessions()
		for _, s := range sessionList {
			if s.Name != current {
				_ = tmux.SwitchClient(s.Name)
				break
			}
		}
		_ = tmux.KillSession(current)
		return tea.QuitMsg{}
	}
}

func openLocalEditorCmd() tea.Cmd {
	return func() tea.Msg {
		path, err := openlocal.ResolveCurrentSessionPath()
		if err != nil {
			return messages.OpenLocalEditorMsg{Err: err}
		}
		editor := openlocal.ResolveEditor()
		if !openlocal.IsSSH() {
			return localEditorLaunch(editor, path)
		}
		return remoteEditorLaunch(editor, path)
	}
}

func localEditorLaunch(editor, path string) tea.Msg {
	bin, args := openlocal.LocalEditorCommand(editor, path)
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		return messages.OpenLocalEditorMsg{Err: fmt.Errorf("open editor: %w", err)}
	}
	go func() { _ = cmd.Wait() }()
	return messages.OpenLocalEditorMsg{}
}

func remoteEditorLaunch(editor, path string) tea.Msg {
	host := openlocal.ResolveSSHHost()
	if host == "" {
		return messages.OpenLocalEditorMsg{
			Err: fmt.Errorf("SSH host not configured: set KITMUX_SSH_HOST or run from a session with a cached host"),
		}
	}
	_ = openlocal.CacheSSHHost(host)
	socketPath := openlocal.ResolveSocketPath()
	req := openlocal.Request{Editor: editor, Host: host, Path: path}
	if err := openlocal.SendOpenRequest(socketPath, req); err != nil {
		return messages.OpenLocalEditorMsg{Fallback: openlocal.FallbackCommand(editor, host, path)}
	}
	return messages.OpenLocalEditorMsg{}
}

func (m Model) launchAgent(msg messages.LaunchAgentMsg) (tea.Model, tea.Cmd) {
	// Find the agent and mode
	agentsList := agents.DefaultAgents()
	for _, a := range agentsList {
		if a.ID != msg.AgentID {
			continue
		}
		for _, mode := range a.Modes {
			if mode.ID != msg.ModeID {
				continue
			}
			command := a.FullCommand(mode)
			switch msg.Target {
			case "split":
				_ = tmux.SplitWindow(command)
			case "window":
				_ = tmux.NewWindowWithCommand(a.Name, command)
			default: // "pane"
				_ = tmux.SendKeys("!", command)
			}
			return m, tea.Quit
		}
	}
	return m, tea.Quit
}

func (m Model) launchAgentAB(msg messages.LaunchAgentABMsg) (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(msg.Prompt)
	if prompt == "" {
		_ = tmux.DisplayMessage("agent_ab: prompt is required")
		return m, nil
	}

	finalPrompt := prompt
	if msg.PlanMode {
		finalPrompt = config.ABPlanPrefix() + prompt
	}

	codexCmd, err := agents.RenderPromptTemplate(config.ABCodexTemplate(), finalPrompt)
	if err != nil {
		_ = tmux.DisplayMessage(fmt.Sprintf("agent_ab codex template error: %v", err))
		return m, nil
	}
	claudeCmd, err := agents.RenderPromptTemplate(config.ABClaudeTemplate(), finalPrompt)
	if err != nil {
		_ = tmux.DisplayMessage(fmt.Sprintf("agent_ab claude template error: %v", err))
		return m, nil
	}

	currentPath, err := tmux.CurrentPanePath()
	if err != nil {
		_ = tmux.DisplayMessage(fmt.Sprintf("agent_ab pane path error: %v", err))
		return m, nil
	}

	codexPath, claudePath, err := worktree.PrepareABWorktrees(currentPath, config.ABBaseBranch())
	if err != nil {
		_ = tmux.DisplayMessage(fmt.Sprintf("agent_ab worktree error: %v", err))
		return m, nil
	}

	paneID, err := tmux.NewWindowInDir("A/B", codexPath, codexCmd)
	if err != nil {
		_ = tmux.DisplayMessage(fmt.Sprintf("agent_ab new-window error: %v", err))
		return m, nil
	}
	if _, err := tmux.SplitWindowInDir(paneID, claudePath, claudeCmd); err != nil {
		_ = tmux.DisplayMessage(fmt.Sprintf("agent_ab split-window error: %v", err))
		return m, nil
	}
	_ = tmux.SelectLayout(paneID, "even-horizontal")

	return m, tea.Quit
}
