package data

import (
	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

// AgentPane is a detected running-agent pane inside a workspace.
type AgentPane struct {
	AgentID     string
	Name        string
	SessionName string
	WindowIndex int
	PaneIndex   int
}

// DetectAgents finds panes running a registered agent command whose
// session root matches the given workspace path.
func DetectAgents(panes []tmux.Pane, sessionRoots map[string]string, workspacePath string) []AgentPane {
	byCmd := make(map[string]agents.Agent)
	for _, a := range agents.DefaultAgents() {
		byCmd[a.Command] = a
	}

	out := make([]AgentPane, 0)
	for _, p := range panes {
		if sessionRoots[p.SessionName] != workspacePath {
			continue
		}
		a, ok := byCmd[p.Command]
		if !ok {
			continue
		}
		out = append(out, AgentPane{
			AgentID:     a.ID,
			Name:        a.Name,
			SessionName: p.SessionName,
			WindowIndex: p.WindowIndex,
			PaneIndex:   p.PaneIndex,
		})
	}
	return out
}
