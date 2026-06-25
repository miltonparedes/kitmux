package agentenv

import "strings"

const (
	AgentIDKey     = "KITMUX_AGENT_ID"
	TmuxSessionKey = "KITMUX_TMUX_SESSION"
	TmuxPaneKey    = "KITMUX_TMUX_PANE"
	TmuxThreadKey  = "KITMUX_TMUX_THREAD"
	ColorTermKey   = "COLORTERM"
	ForceColorKey  = "FORCE_COLOR"
	CliColorKey    = "CLICOLOR"
	CliColorForce  = "CLICOLOR_FORCE"
)

func WrapTmuxCommand(agentID, sessionName, command string, thread bool) string {
	if shouldWaitForClient(agentID, thread) {
		parts := append(
			trackingAssignments(agentID, sessionName, thread),
			";", "export", strings.Join(exportKeys(agentID, thread), " "),
			";", waitForClientCommand(),
			";", "exec", command,
		)
		return strings.Join(parts, " ")
	}
	parts := append(trackingAssignments(agentID, sessionName, thread), "exec", command)
	return strings.Join(parts, " ")
}

func WrapRegisteredTmuxCommand(agentID, sessionName, command string, thread bool, kitmuxPath string) string {
	assignments := trackingAssignments(agentID, sessionName, thread)
	exports := "export " + strings.Join(exportKeys(agentID, thread), " ")
	register := shellQuote(kitmuxPath) + ` hook agent-register --pid "$$" --agent "$` + AgentIDKey +
		`" --session "$` + TmuxSessionKey + `" --pane "$` + TmuxPaneKey + `" --thread "$` + TmuxThreadKey + `"`
	assignments = append(assignments, ";", exports, ";", register, ">/dev/null", "2>&1", "||", "true")
	if shouldWaitForClient(agentID, thread) {
		assignments = append(assignments, ";", waitForClientCommand())
	}
	assignments = append(assignments, ";", "exec", command)
	return strings.Join(assignments, " ")
}

func trackingAssignments(agentID, sessionName string, thread bool) []string {
	defaults := colorEnvDefaults(agentID, thread)
	parts := make([]string, 0, 4+len(defaults))
	for _, item := range defaults {
		parts = append(parts, defaultAssignment(item))
	}
	parts = append(parts,
		AgentIDKey+"="+shellQuote(agentID),
		TmuxSessionKey+"="+shellValueOrTmuxFormat(sessionName, "session_name"),
		TmuxPaneKey+"="+tmuxPaneValue(),
		TmuxThreadKey+"="+shellQuote(""),
	)
	if thread {
		parts[len(parts)-1] = TmuxThreadKey + "=1"
	}
	return parts
}

func exportKeys(agentID string, thread bool) []string {
	keys := []string{AgentIDKey, TmuxSessionKey, TmuxPaneKey, TmuxThreadKey}
	defaults := colorEnvDefaults(agentID, thread)
	if len(defaults) == 0 {
		return keys
	}
	out := make([]string, 0, len(keys)+len(defaults))
	for _, item := range defaults {
		out = append(out, item.key)
	}
	return append(out, keys...)
}

type envDefault struct {
	key              string
	value            string
	preserveExisting bool
}

func colorEnvDefaults(agentID string, thread bool) []envDefault {
	if agentID != "codex" || !thread {
		return nil
	}
	return []envDefault{
		{key: ColorTermKey, value: "truecolor", preserveExisting: true},
		{key: ForceColorKey, value: "3"},
		{key: CliColorKey, value: "1"},
		{key: CliColorForce, value: "1"},
	}
}

func defaultAssignment(item envDefault) string {
	if !item.preserveExisting {
		return item.key + "=" + shellQuote(item.value)
	}
	return item.key + `="${` + item.key + `:-` + item.value + `}"`
}

func shouldWaitForClient(agentID string, thread bool) bool {
	return agentID == "codex" && thread
}

func waitForClientCommand() string {
	return `while [ -z "$(tmux list-clients -t "$` + TmuxSessionKey + `" -F '#{client_name}' 2>/dev/null)" ]; do sleep 0.05; done`
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
	env = withEnvDefaults(env, colorEnvDefaults(agentID, thread))
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

func withEnvDefaults(env []string, defaults []envDefault) []string {
	for _, item := range defaults {
		if item.preserveExisting && hasEnvKey(env, item.key) {
			continue
		}
		if !item.preserveExisting {
			env = withoutKeys(env, item.key)
		}
		env = append(env, item.key+"="+item.value)
	}
	return env
}

func hasEnvKey(env []string, key string) bool {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
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
