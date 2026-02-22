package agents

// Agent represents a coding agent that can be launched.
type Agent struct {
	ID      string
	Name    string
	Command string
	Modes   []AgentMode
}

// AgentMode represents a launch mode for an agent.
type AgentMode struct {
	ID    string
	Name  string
	Flags string // additional flags appended to command
}

// DefaultAgents returns the built-in agent registry.
func DefaultAgents() []Agent {
	return []Agent{
		{
			ID:      "claude",
			Name:    "Claude Code",
			Command: "claude",
			Modes: []AgentMode{
				{ID: "default", Name: "Default", Flags: ""},
				{ID: "skip-perms", Name: "Skip Permissions", Flags: "--dangerously-skip-permissions"},
			},
		},
		{
			ID:      "gemini",
			Name:    "Gemini CLI",
			Command: "gemini",
			Modes: []AgentMode{
				{ID: "default", Name: "Default", Flags: ""},
			},
		},
		{
			ID:      "codex",
			Name:    "Codex CLI",
			Command: "codex",
			Modes: []AgentMode{
				{ID: "default", Name: "Default", Flags: ""},
				{ID: "exec", Name: "Exec", Flags: "--approval-mode full-auto"},
				{ID: "review", Name: "Review", Flags: "--approval-mode review"},
				{ID: "apply", Name: "Apply", Flags: "--approval-mode auto-edit"},
			},
		},
		{
			ID:      "aichat",
			Name:    "AIChat",
			Command: "aichat",
			Modes: []AgentMode{
				{ID: "default", Name: "Interactive", Flags: ""},
				{ID: "execute", Name: "Execute", Flags: "-e"},
			},
		},
		{
			ID:      "opencode",
			Name:    "OpenCode",
			Command: "opencode",
			Modes: []AgentMode{
				{ID: "default", Name: "Default", Flags: ""},
			},
		},
	}
}

// FullCommand returns the complete command string for an agent with the given mode.
func (a Agent) FullCommand(mode AgentMode) string {
	if mode.Flags == "" {
		return a.Command
	}
	return a.Command + " " + mode.Flags
}
