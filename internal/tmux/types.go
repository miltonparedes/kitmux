package tmux

// Session represents a tmux session.
type Session struct {
	Name              string
	Windows           int
	Attached          bool
	Path              string // session working directory
	Activity          int64  // unix timestamp of last activity
	Thread            bool   // true when managed by kitmux as an agent thread
	AgentID           string // registered agent ID for kitmux-managed threads
	AgentState        string // optional state written by agent hooks: idle, working, input, permission, error
	AgentEvent        string // optional hook event that produced AgentState
	AgentDetail       string // optional short hook detail, such as a tool or notification reason
	AgentUpdated      int64  // unix milliseconds when AgentState was last written
	ThreadTitle       string // optional kitmux display title override for agent threads
	AgentTitlePrefix  string // optional prefix used to compose the thread title
	AgentTitleDisplay string // optional agent-provided display title
	AgentSessionID    string // optional persisted agent conversation/session id
}

// ThreadContext describes the tmux session hosting the current process.
type ThreadContext struct {
	SessionName string
	PaneID      string
	Thread      bool
	AgentID     string
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
	SessionName       string
	WindowIndex       int
	PaneIndex         int
	ID                string
	Command           string
	PID               int
	Path              string
	Title             string
	AgentState        string
	AgentEvent        string
	AgentDetail       string
	AgentUpdated      int64
	AgentTitlePrefix  string
	AgentTitleDisplay string
	AgentSessionID    string
}
