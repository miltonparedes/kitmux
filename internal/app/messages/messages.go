package messages

// SwitchSessionMsg requests switching to a session and exiting.
type SwitchSessionMsg struct {
	Name string
}

// DrillWindowsMsg requests focusing the windows panel for a session.
type DrillWindowsMsg struct {
	SessionName string
}

// BackToSessionsMsg returns focus to the sessions panel.
type BackToSessionsMsg struct{}

// SwitchWindowMsg requests switching to a specific window.
type SwitchWindowMsg struct {
	Target string // "session:window_index"
}

// TogglePaletteMsg toggles the command palette.
type TogglePaletteMsg struct{}

// ExecuteCommandMsg runs a palette command.
type ExecuteCommandMsg struct {
	ID string
}

// ReloadSessionsMsg signals that sessions should be reloaded.
type ReloadSessionsMsg struct{}

// SessionCursorMsg notifies that the session cursor changed (for auto-loading windows).
type SessionCursorMsg struct {
	SessionName string
}

// SwitchWorktreeMsg exits kitmux and runs wt switch to the given branch.
type SwitchWorktreeMsg struct {
	Branch string
}

// CreateWorktreeMsg exits kitmux and runs wt switch --create for a new branch.
type CreateWorktreeMsg struct {
	Branch string
}

// RemoveWorktreeMsg runs wt remove for a branch, then reloads.
type RemoveWorktreeMsg struct {
	Branch string
}

// ReloadWorktreesMsg signals that worktrees should be reloaded.
type ReloadWorktreesMsg struct{}

// LaunchAgentMsg launches a coding agent via tmux.
type LaunchAgentMsg struct {
	AgentID string
	ModeID  string
	Target  string // "pane", "split", "window"
	Prompt  string
}

// SwitchViewMsg switches between sessions/worktrees/agents views.
type SwitchViewMsg struct {
	View string // "sessions", "worktrees", "agents"
}

// CreateSessionInDirMsg creates a new session in the given directory and switches to it.
type CreateSessionInDirMsg struct {
	Name string
	Dir  string
}

// RunPopupMsg runs a command in a tmux popup and exits.
type RunPopupMsg struct {
	Command string
	Width   string
	Height  string
}

// OpenLocalEditorMsg signals the result of the open-in-local-editor action.
type OpenLocalEditorMsg struct {
	Fallback string // non-empty when bridge was unreachable; contains the manual command
	Err      error  // non-nil on hard failure
}
