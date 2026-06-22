package agentenv

import "strings"

const (
	AgentIDKey     = "KITMUX_AGENT_ID"
	TmuxSessionKey = "KITMUX_TMUX_SESSION"
	TmuxPaneKey    = "KITMUX_TMUX_PANE"
	TmuxThreadKey  = "KITMUX_TMUX_THREAD"
)

func WrapTmuxCommand(agentID, sessionName, command string, thread bool) string {
	parts := append(trackingAssignments(agentID, sessionName, thread), "exec", command)
	return strings.Join(parts, " ")
}

func WrapRegisteredTmuxCommand(agentID, sessionName, command string, thread bool, kitmuxPath string) string {
	assignments := trackingAssignments(agentID, sessionName, thread)
	exports := "export " + AgentIDKey + " " + TmuxSessionKey + " " + TmuxPaneKey + " " + TmuxThreadKey
	register := shellQuote(kitmuxPath) + ` hook agent-register --pid "$$" --agent "$` + AgentIDKey +
		`" --session "$` + TmuxSessionKey + `" --pane "$` + TmuxPaneKey + `" --thread "$` + TmuxThreadKey + `"`
	assignments = append(assignments, ";", exports, ";", register, ">/dev/null", "2>&1", "||", "true", ";", "exec", command)
	return strings.Join(assignments, " ")
}

func trackingAssignments(agentID, sessionName string, thread bool) []string {
	parts := []string{
		AgentIDKey + "=" + shellQuote(agentID),
		TmuxSessionKey + "=" + shellValueOrTmuxFormat(sessionName, "session_name"),
		TmuxPaneKey + "=" + tmuxPaneValue(),
		TmuxThreadKey + "=" + shellQuote(""),
	}
	if thread {
		parts[len(parts)-1] = TmuxThreadKey + "=1"
	}
	return parts
}

func WrapHookCommand(agentID, command string) string {
	parts := []string{
		AgentIDKey + "=" + shellQuote(agentID),
		TmuxSessionKey + `="${` + TmuxSessionKey + `:-}"`,
		TmuxPaneKey + `="${` + TmuxPaneKey + `:-}"`,
		TmuxThreadKey + `="${` + TmuxThreadKey + `:-}"`,
		command,
	}
	return strings.Join(parts, " ")
}

func WithTrackingEnv(env []string, agentID, sessionName, paneID string, thread bool) []string {
	env = withoutKeys(env, AgentIDKey, TmuxSessionKey, TmuxPaneKey, TmuxThreadKey)
	if agentID != "" {
		env = append(env, AgentIDKey+"="+agentID)
	}
	if sessionName != "" {
		env = append(env, TmuxSessionKey+"="+sessionName)
	}
	if paneID != "" {
		env = append(env, TmuxPaneKey+"="+paneID)
	}
	if thread {
		env = append(env, TmuxThreadKey+"=1")
	}
	return env
}

func shellValueOrTmuxFormat(value, format string) string {
	if value != "" {
		return shellQuote(value)
	}
	return tmuxFormatCommand(format)
}

func tmuxFormatCommand(format string) string {
	return `"$(tmux display-message -p '#{` + format + `}' 2>/dev/null)"`
}

func tmuxPaneValue() string {
	return `"${TMUX_PANE:-$(tmux display-message -p '#{pane_id}' 2>/dev/null)}"`
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func withoutKeys(env []string, keys ...string) []string {
	if len(env) == 0 {
		return env
	}
	remove := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		remove[key+"="] = struct{}{}
	}
	out := env[:0]
	for _, item := range env {
		keep := true
		for prefix := range remove {
			if strings.HasPrefix(item, prefix) {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, item)
		}
	}
	return out
}
