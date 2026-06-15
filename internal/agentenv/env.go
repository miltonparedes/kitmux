package agentenv

import "strings"

const (
	AgentIDKey     = "KITMUX_AGENT_ID"
	TmuxSessionKey = "KITMUX_TMUX_SESSION"
	TmuxPaneKey    = "KITMUX_TMUX_PANE"
	TmuxThreadKey  = "KITMUX_TMUX_THREAD"
)

func WrapTmuxCommand(agentID, sessionName, command string, thread bool) string {
	parts := []string{
		AgentIDKey + "=" + shellQuote(agentID),
		TmuxSessionKey + "=" + shellValueOrTmuxFormat(sessionName, "session_name"),
		TmuxPaneKey + "=" + tmuxPaneValue(),
	}
	if thread {
		parts = append(parts, TmuxThreadKey+"=1")
	}
	parts = append(parts, "exec", command)
	return strings.Join(parts, " ")
}

func WrapHookCommand(agentID, command string) string {
	parts := []string{
		AgentIDKey + "=" + shellQuote(agentID),
		TmuxSessionKey + `="${` + TmuxSessionKey + `:-$(tmux display-message -p '#{session_name}' 2>/dev/null)}"`,
		TmuxPaneKey + `="${` + TmuxPaneKey + `:-${TMUX_PANE:-$(tmux display-message -p '#{pane_id}' 2>/dev/null)}}"`,
		TmuxThreadKey + `="${` + TmuxThreadKey + `:-$(tmux display-message -p '#{@kitmux_thread}' 2>/dev/null)}"`,
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
