package tmux

// Session represents a tmux session.
type Session struct {
	Name     string
	Windows  int
	Attached bool
	Path     string // session working directory
	Activity int64  // unix timestamp of last activity
	Thread   bool   // true when managed by kitmux as an agent thread
	AgentID  string // registered agent ID for kitmux-managed threads
}

// Window represents a tmux window within a session.
type Window struct {
	SessionName string
	Index       int
	Name        string
	Active      bool
}

// Pane represents a tmux pane with its running command.
type Pane struct {
	SessionName string
	WindowIndex int
	PaneIndex   int
	Command     string
	PID         int
	Path        string
}
