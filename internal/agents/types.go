package agents

// Agent represents a coding agent that can be launched.
type Agent struct {
	ID      string
	Name    string
	Symbol  string
	Command string
	Modes   []AgentMode
}

// AgentMode represents a launch mode for an agent.
type AgentMode struct {
	ID    string
	Name  string
	Flags string // additional flags appended to command
}

var defaultAgents = []Agent{
	{
		ID:      "droid",
		Name:    "Droid",
		Symbol:  "⛬",
		Command: "droid",
		Modes: []AgentMode{
			{ID: "default", Name: "Default", Flags: ""},
		},
	},
	{
		ID:      "codex",
		Name:    "Codex CLI",
		Symbol:  "⌾",
		Command: "codex",
		Modes: []AgentMode{
			{ID: "default", Name: "Default", Flags: ""},
			{ID: "exec", Name: "Exec", Flags: "--approval-mode full-auto"},
			{ID: "review", Name: "Review", Flags: "--approval-mode review"},
			{ID: "apply", Name: "Apply", Flags: "--approval-mode auto-edit"},
		},
	},
	{
		ID:      "cursor",
		Name:    "Cursor CLI",
		Symbol:  "⌬",
		Command: "cursor-agent",
		Modes: []AgentMode{
			{ID: "default", Name: "Default", Flags: ""},
		},
	},
	{
		ID:      "claude",
		Name:    "Claude Code",
		Symbol:  "✳",
		Command: "claude",
		Modes: []AgentMode{
			{ID: "default", Name: "Default", Flags: ""},
			{ID: "skip-perms", Name: "Skip Permissions", Flags: "--dangerously-skip-permissions"},
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

// DefaultAgents returns the built-in agent registry.
func DefaultAgents() []Agent {
	out := make([]Agent, len(defaultAgents))
	copy(out, defaultAgents)
	return out
}

func Find(id string) (Agent, bool) {
	for _, a := range DefaultAgents() {
		if a.ID == id {
			return a, true
		}
	}
	return Agent{}, false
}

func FindMode(a Agent, id string) (AgentMode, bool) {
	for _, mode := range a.Modes {
		if mode.ID == id {
			return mode, true
		}
	}
	return AgentMode{}, false
}

func CommandMap() map[string]Agent {
	byCommand := make(map[string]Agent)
	for _, a := range DefaultAgents() {
		byCommand[a.Command] = a
	}
	return byCommand
}

func IsAgentCommand(command string) bool {
	_, ok := CommandMap()[command]
	return ok
}

// FullCommand returns the complete command string for an agent with the given mode.
func (a Agent) FullCommand(mode AgentMode) string {
	if mode.Flags == "" {
		return a.Command
	}
	return a.Command + " " + mode.Flags
}

func (a Agent) DisplayName() string {
	if a.Symbol == "" {
		return a.Name
	}
	return a.Symbol + " " + a.Name
}
