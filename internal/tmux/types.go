package tmux

// Session represents a tmux session.
type Session struct {
	Name     string
	Windows  int
	Attached bool
	Path     string // session working directory
	Activity int64  // unix timestamp of last activity
}

// Window represents a tmux window within a session.
type Window struct {
	SessionName string
	Index       int
	Name        string
	Active      bool
}
