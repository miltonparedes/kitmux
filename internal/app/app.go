package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
	agentsview "github.com/miltonparedes/kitmux/internal/views/agents"
	"github.com/miltonparedes/kitmux/internal/views/palette"
	"github.com/miltonparedes/kitmux/internal/views/sessions"
	"github.com/miltonparedes/kitmux/internal/views/windows"
	"github.com/miltonparedes/kitmux/internal/views/worktrees"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// Mode determines how kitmux starts.
type Mode int

const (
	ModeSessions  Mode = iota // Session tree view
	ModePalette               // Command palette-only mode
	ModeWorktrees             // Worktrees view
	ModeAgents                // Agents view
	ModeProjects              // Project picker (direct access)
	ModeWindows               // Windows for current session
	ModeRun                   // Execute a palette command directly
)

type activeView int

const (
	viewSessions  activeView = iota
	viewWindows              // Windows drill-down
	viewWorktrees            // Worktree list
	viewAgents               // Agent launcher
)

type Model struct {
	mode          Mode
	view          activeView
	sessions      sessions.Model
	windows       windows.Model
	worktreeView  worktrees.Model
	agentsView    agentsview.Model
	palette       palette.Model
	paletteActive bool
	paletteReturn bool        // return to palette after sub-action completes
	pendingKey    *tea.KeyMsg // key to inject after sessions load
	width         int
	height        int
	runCommandID  string // for ModeRun: the command to execute
}

func New(mode Mode, opts ...Option) Model {
	m := Model{
		mode:         mode,
		view:         viewSessions,
		sessions:     sessions.New(),
		windows:      windows.New(),
		worktreeView: worktrees.New(),
		agentsView:   agentsview.New(),
		palette:      palette.New(),
	}
	for _, opt := range opts {
		opt(&m)
	}
	switch mode {
	case ModePalette:
		m.paletteActive = true
	case ModeWorktrees:
		m.view = viewWorktrees
	case ModeAgents:
		m.view = viewAgents
	case ModeProjects:
		m.view = viewSessions
		m.sessions.SetPickingMode()
	case ModeWindows:
		m.view = viewWindows
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
	case ModeProjects:
		return m.sessions.ProjectPickerCmds()
	case ModeWindows:
		return m.initCurrentSessionWindows()
	default:
		switch m.view {
		case viewWorktrees:
			return m.worktreeView.Init()
		case viewAgents:
			return m.agentsView.Init()
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sessions.SetSize(m.width, m.height-1)
		m.windows.SetSize(m.width, m.height-1)
		m.worktreeView.SetSize(m.width, m.height-1)
		m.agentsView.SetSize(m.width, m.height-1)
		m.palette.SetSize(m.width, m.height)
		return m, nil

	case tea.MouseMsg:
		if m.paletteActive {
			var cmd tea.Cmd
			m.palette, cmd = m.palette.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		if updated, cmd, handled := m.handleKeyMsg(msg); handled {
			return updated, cmd
		}

	case messages.TogglePaletteMsg:
		if m.mode == ModePalette {
			return m, tea.Quit
		}
		m.paletteActive = !m.paletteActive
		if m.paletteActive {
			m.palette.Reset()
		}
		return m, nil

	case messages.ExecuteCommandMsg:
		m.paletteActive = false
		return m.executeCommand(msg.ID)

	case messages.SwitchSessionMsg:
		_ = tmux.SwitchClient(msg.Name)
		return m, tea.Quit

	case messages.SwitchWindowMsg:
		_ = tmux.SwitchClient(msg.Target)
		return m, tea.Quit

	case messages.DrillWindowsMsg:
		m.view = viewWindows
		cmd := m.windows.LoadSession(msg.SessionName)
		return m, cmd

	case messages.BackToSessionsMsg:
		if m.paletteReturn {
			return m, m.returnToPalette()
		}
		m.view = viewSessions
		return m, nil

	case messages.SessionCursorMsg:
		return m, nil

	case messages.ReloadSessionsMsg:
		m.view = viewSessions
		return m, m.sessions.Reload()

	case messages.CreateSessionInDirMsg:
		_ = tmux.NewSessionInDir(msg.Name, msg.Dir)
		_ = tmux.SwitchClient(msg.Name)
		return m, tea.Quit

	case messages.SwitchWorktreeMsg:
		_ = worktree.SwitchTo(msg.Branch)
		return m, tea.Quit

	case messages.CreateWorktreeMsg:
		_ = worktree.Create(msg.Branch)
		return m, tea.Quit

	case messages.RemoveWorktreeMsg:
		_ = worktree.Remove(msg.Branch)
		return m, m.worktreeView.Reload()

	case messages.ReloadWorktreesMsg:
		return m, m.worktreeView.Reload()

	case messages.LaunchAgentMsg:
		return m.launchAgent(msg)

	case messages.SwitchViewMsg:
		switch msg.View {
		case "sessions":
			if m.paletteReturn {
				return m, m.returnToPalette()
			}
			m.view = viewSessions
			return m, nil
		case "worktrees":
			m.view = viewWorktrees
			return m, m.worktreeView.Init()
		case "agents":
			m.view = viewAgents
			return m, nil
		}
		return m, nil

	case messages.RunPopupMsg:
		_ = tmux.DisplayPopup(msg.Command, msg.Width, msg.Height)
		return m, tea.Quit
	}

	return m.routeToView(msg)
}

// handleKeyMsg processes keyboard input.
// Returns (model, cmd, handled); when handled is false the caller
// should route the original message to the active view.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	if m.paletteActive {
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

	isEditing := m.sessions.IsEditing() || m.worktreeView.IsEditing()
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit, true
	case "q":
		if !isEditing {
			return m, tea.Quit, true
		}
	case "ctrl+p":
		if !isEditing {
			m.paletteActive = true
			m.palette.Reset()
			return m, nil, true
		}
	case "w":
		if !isEditing && m.view != viewWorktrees {
			m.view = viewWorktrees
			return m, m.worktreeView.Init(), true
		}
	case "a":
		if !isEditing && m.view != viewAgents {
			m.view = viewAgents
			return m, nil, true
		}
	case "esc":
		if m.paletteReturn && !isEditing {
			return m, m.returnToPalette(), true
		}
		switch m.view {
		case viewWindows:
			if m.mode == ModeWindows {
				return m, tea.Quit, true
			}
			m.view = viewSessions
			return m, nil, true
		case viewWorktrees:
			if m.mode == ModeWorktrees {
				return m, tea.Quit, true
			}
			m.view = viewSessions
			return m, nil, true
		case viewAgents:
			if m.mode == ModeAgents {
				return m, tea.Quit, true
			}
			m.view = viewSessions
			return m, nil, true
		default:
			if m.sessions.IsEditing() {
				if m.mode == ModeProjects {
					return m, tea.Quit, true
				}
				// esc in sessions editing → falls through to sessions.Update
			} else {
				return m, tea.Quit, true
			}
		}
	}
	return m, nil, false
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
	default:
		return m.sessions.View()
	}
}

// returnToPalette reactivates the palette or quits if in a terminal mode.
// Must be called on the m being returned (value receiver — mutations stay local).
func (m *Model) returnToPalette() tea.Cmd {
	m.paletteReturn = false
	if m.mode == ModePalette || m.mode == ModeRun || m.mode == ModeProjects {
		return tea.Quit
	}
	m.paletteActive = true
	m.palette.Reset()
	return nil
}

func (m Model) executeCommand(id string) (tea.Model, tea.Cmd) {
	m.paletteReturn = true

	switch id {
	// Session commands
	case "switch_session":
		m.paletteReturn = false
		m.view = viewSessions
		return m, m.sessions.Reload()
	case "open_project":
		m.view = viewSessions
		return m, m.sessions.InitProjectPicker()
	case "kill_session":
		m.view = viewSessions
		km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
		m.pendingKey = &km
		return m, m.sessions.Reload()
	case "kill_current_session":
		return m, func() tea.Msg {
			current, err := tmux.CurrentSession()
			if err != nil || current == "" {
				return tea.QuitMsg{}
			}
			sessions, _ := tmux.ListSessions()
			for _, s := range sessions {
				if s.Name != current {
					_ = tmux.SwitchClient(s.Name)
					break
				}
			}
			_ = tmux.KillSession(current)
			return tea.QuitMsg{}
		}
	case "rename_session":
		m.view = viewSessions
		km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
		m.pendingKey = &km
		return m, m.sessions.Reload()

	// Worktree commands — switch to worktrees view and simulate keys
	case "wt_switch":
		m.view = viewWorktrees
		return m, m.worktreeView.Init()
	case "wt_create":
		m.view = viewWorktrees
		cmd := m.worktreeView.Init()
		km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
		var cmd2 tea.Cmd
		m.worktreeView, cmd2 = m.worktreeView.Update(km)
		return m, tea.Batch(cmd, cmd2)
	case "wt_create_describe":
		m.view = viewWorktrees
		cmd := m.worktreeView.Init()
		km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}
		var cmd2 tea.Cmd
		m.worktreeView, cmd2 = m.worktreeView.Update(km)
		return m, tea.Batch(cmd, cmd2)
	case "wt_remove":
		m.view = viewWorktrees
		cmd := m.worktreeView.Init()
		km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
		var cmd2 tea.Cmd
		m.worktreeView, cmd2 = m.worktreeView.Update(km)
		return m, tea.Batch(cmd, cmd2)
	case "wt_merge":
		return m, func() tea.Msg {
			return messages.RunPopupMsg{Command: "wt merge", Width: "80%", Height: "80%"}
		}
	case "wt_commit":
		return m, func() tea.Msg {
			return messages.RunPopupMsg{Command: "wt step commit", Width: "80%", Height: "80%"}
		}

	// Agent commands — launch directly
	case "launch_claude":
		return m, func() tea.Msg {
			return messages.LaunchAgentMsg{AgentID: "claude", ModeID: "default", Target: "pane"}
		}
	case "launch_gemini":
		return m, func() tea.Msg {
			return messages.LaunchAgentMsg{AgentID: "gemini", ModeID: "default", Target: "pane"}
		}
	case "launch_codex":
		return m, func() tea.Msg {
			return messages.LaunchAgentMsg{AgentID: "codex", ModeID: "default", Target: "pane"}
		}
	case "launch_aichat":
		return m, func() tea.Msg {
			return messages.LaunchAgentMsg{AgentID: "aichat", ModeID: "default", Target: "pane"}
		}
	case "launch_opencode":
		return m, func() tea.Msg {
			return messages.LaunchAgentMsg{AgentID: "opencode", ModeID: "default", Target: "pane"}
		}

	// Tool commands
	case "tool_lazygit":
		return m, func() tea.Msg {
			return messages.RunPopupMsg{Command: "lazygit", Width: "100%", Height: "100%"}
		}
	case "tool_lumen_diff":
		return m, func() tea.Msg {
			return messages.RunPopupMsg{Command: "lumen diff", Width: "100%", Height: "100%"}
		}

	// View commands
	case "view_sessions":
		m.view = viewSessions
		return m, nil
	case "view_worktrees":
		m.view = viewWorktrees
		return m, m.worktreeView.Init()
	case "view_agents":
		m.view = viewAgents
		return m, nil
	}
	return m, nil
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
