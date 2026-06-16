package agenthooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miltonparedes/kitmux/internal/agentenv"
)

const legacyBellCommand = `sh -c 'printf "\007" > /dev/tty 2>/dev/null || printf "\007"'`

var ErrUnsupportedAgent = errors.New("unsupported agent hooks")

type hookSpec struct {
	Event           string
	Matcher         string
	Command         string
	ReplaceCommands []string
	AgentID         string
	AgentEvent      string
}

type Result struct {
	AgentID string
	Path    string
	Changed bool
}

type Installer struct {
	HomeDir string
}

func InstallAll(homeDir string) ([]Result, error) {
	return Installer{HomeDir: homeDir}.InstallAll()
}

func Install(agentID, homeDir string) (Result, error) {
	return Installer{HomeDir: homeDir}.Install(agentID)
}

func (i Installer) InstallAll() ([]Result, error) {
	home, err := i.homeDir()
	if err != nil {
		return nil, err
	}

	installers := []func(string) (Result, error){
		installDroid,
		installClaude,
		installCodex,
		installOpenCode,
	}
	results := make([]Result, 0, len(installers))
	for _, install := range installers {
		result, err := install(home)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (i Installer) Install(agentID string) (Result, error) {
	home, err := i.homeDir()
	if err != nil {
		return Result{AgentID: agentID}, err
	}

	install, ok := installerForAgent(agentID)
	if !ok {
		return Result{AgentID: agentID}, ErrUnsupportedAgent
	}
	return install(home)
}

func (i Installer) homeDir() (string, error) {
	if i.HomeDir != "" {
		return i.HomeDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return home, nil
}

func installerForAgent(agentID string) (func(string) (Result, error), bool) {
	switch agentID {
	case "droid":
		return installDroid, true
	case "claude":
		return installClaude, true
	case "codex", "codex-cloud":
		return installCodex, true
	case "opencode":
		return installOpenCode, true
	default:
		return nil, false
	}
}

func installDroid(home string) (Result, error) {
	hooks := droidHookSpecs()

	// Standalone hooks.json (documented primary location for newer droid builds).
	hooksPath := filepath.Join(home, ".factory", "hooks.json")
	changedHooks, err := installJSONHooks(hooksPath, hooks)
	if err != nil {
		return Result{AgentID: "droid", Path: hooksPath, Changed: changedHooks}, err
	}

	// settings.json "hooks" key + enableHooks. Current droid builds load hooks from
	// here and ignore the standalone hooks.json, so this is the operative location.
	settingsPath := filepath.Join(home, ".factory", "settings.json")
	changedSettings, err := installDroidSettingsHooks(settingsPath, hooks)
	if err != nil {
		return Result{AgentID: "droid", Path: settingsPath, Changed: changedHooks || changedSettings}, err
	}
	return Result{AgentID: "droid", Path: settingsPath, Changed: changedHooks || changedSettings}, nil
}

func droidHookSpecs() []hookSpec {
	return []hookSpec{
		eventHook("droid", "SessionStart", "", "session-start", stateIdle, false),
		eventHook("droid", "UserPromptSubmit", "", "user-prompt-submit", stateWorking, false),
		eventHook("droid", "PreToolUse", "*", "pre-tool-use", stateWorking, false),
		eventHook("droid", "PostToolUse", "*", "post-tool-use", stateWorking, false),
		eventHook("droid", "Notification", "", "notification", stateInput, true),
		eventHook("droid", "Stop", "", "stop", stateIdle, true),
		eventHook("droid", "SessionEnd", "", "session-end", stateIdle, false),
	}
}

func installDroidSettingsHooks(path string, hooks []hookSpec) (bool, error) {
	doc, err := readJSON(path)
	if err != nil {
		return false, err
	}
	changed := setBool(doc, "enableHooks", true)
	if addCommandHooks(doc, hooks) {
		changed = true
	}
	if changed {
		if err := writeJSON(path, doc); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func installClaude(home string) (Result, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	doc, err := readJSON(path)
	if err != nil {
		return Result{AgentID: "claude", Path: path}, err
	}
	changed := setString(doc, "preferredNotifChannel", "terminal_bell")
	if addCommandHooks(doc, []hookSpec{
		eventHook("claude", "SessionStart", "", "session-start", stateIdle, false),
		eventHook("claude", "UserPromptSubmit", "", "user-prompt-submit", stateWorking, false),
		eventHook("claude", "PreToolUse", "*", "pre-tool-use", stateWorking, false),
		eventHook("claude", "PostToolUse", "*", "post-tool-use", stateWorking, false),
		eventHook("claude", "PostToolUseFailure", "*", "post-tool-use-failure", stateWorking, false),
		eventHook("claude", "PostToolBatch", "", "post-tool-batch", stateWorking, false),
		eventHook("claude", "PermissionRequest", "", "permission-request", statePermission, true),
		eventHook("claude", "PermissionDenied", "", "permission-denied", stateError, true),
		eventHook("claude", "Elicitation", "", "elicitation", stateInput, true),
		eventHook("claude", "ElicitationResult", "", "elicitation-result", stateWorking, false),
		eventHook("claude", "Notification", "", "notification", stateInput, true),
		eventHook("claude", "Stop", "", "stop", stateIdle, true),
		eventHook("claude", "StopFailure", "", "stop-failure", stateIdle, true),
		eventHook("claude", "SessionEnd", "", "session-end", stateIdle, false),
	}) {
		changed = true
	}
	if changed {
		if err := writeJSON(path, doc); err != nil {
			return Result{AgentID: "claude", Path: path}, err
		}
	}
	return Result{AgentID: "claude", Path: path, Changed: changed}, nil
}

func installCodex(home string) (Result, error) {
	path := filepath.Join(home, ".codex", "hooks.json")
	changed, err := installJSONHooks(path, []hookSpec{
		eventHook("codex", "SessionStart", "", "session-start", stateIdle, false),
		eventHook("codex", "UserPromptSubmit", "", "user-prompt-submit", stateWorking, false),
		eventHook("codex", "PreToolUse", "", "pre-tool-use", stateWorking, false),
		eventHook("codex", "PermissionRequest", "", "permission-request", statePermission, true),
		eventHook("codex", "PostToolUse", "", "post-tool-use", stateWorking, false),
		eventHook("codex", "Stop", "", "stop", stateIdle, true),
	})
	return Result{AgentID: "codex", Path: path, Changed: changed}, err
}

func installOpenCode(home string) (Result, error) {
	path := filepath.Join(home, ".config", "opencode", "plugins", "kitmux-zed-bell.js")
	content := []byte(openCodePlugin(kitmuxCommand()))
	// #nosec G304 -- path is derived from the user's home directory and a fixed agent config path.
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return Result{AgentID: "opencode", Path: path}, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{AgentID: "opencode", Path: path}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Result{AgentID: "opencode", Path: path}, err
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return Result{AgentID: "opencode", Path: path}, err
	}
	return Result{AgentID: "opencode", Path: path, Changed: true}, nil
}

func installJSONHooks(path string, hooks []hookSpec) (bool, error) {
	doc, err := readJSON(path)
	if err != nil {
		return false, err
	}
	changed := addCommandHooks(doc, hooks)
	if changed {
		if err := writeJSON(path, doc); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func readJSON(path string) (map[string]any, error) {
	// #nosec G304 -- path is derived from the user's home directory and fixed agent config paths.
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]any), nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if doc == nil {
		doc = make(map[string]any)
	}
	return doc, nil
}

func writeJSON(path string, doc map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func setString(doc map[string]any, key, value string) bool {
	if got, ok := doc[key].(string); ok && got == value {
		return false
	}
	doc[key] = value
	return true
}

func setBool(doc map[string]any, key string, value bool) bool {
	if got, ok := doc[key].(bool); ok && got == value {
		return false
	}
	doc[key] = value
	return true
}

func addCommandHooks(doc map[string]any, hooks []hookSpec) bool {
	rawHooks, _ := doc["hooks"].(map[string]any)
	if rawHooks == nil {
		rawHooks = make(map[string]any)
		doc["hooks"] = rawHooks
	}

	changed := false
	for _, spec := range hooks {
		if addCommandHook(rawHooks, spec) {
			changed = true
		}
	}
	return changed
}

func addCommandHook(rawHooks map[string]any, spec hookSpec) bool {
	groups, _ := rawHooks[spec.Event].([]any)
	changed := replaceCommands(groups, spec.ReplaceCommands, spec.Command)
	if normalizedGroups, normalized := normalizeAgentEventHooks(groups, spec); normalized {
		groups = normalizedGroups
		rawHooks[spec.Event] = groups
		changed = true
	}
	if dedupedGroups, deduped := dedupeCommandHooks(groups, spec.Command); deduped {
		groups = dedupedGroups
		rawHooks[spec.Event] = groups
		changed = true
	}
	if hasCommand(groups, spec.Command) {
		return changed
	}

	group := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": spec.Command,
				"timeout": float64(5),
			},
		},
	}
	if spec.Matcher != "" {
		group["matcher"] = spec.Matcher
	}
	rawHooks[spec.Event] = append(groups, group)
	return true
}

func normalizeAgentEventHooks(groups []any, spec hookSpec) ([]any, bool) {
	if spec.AgentID == "" || spec.AgentEvent == "" {
		return groups, false
	}
	seenCurrent := false
	changed := false
	out := make([]any, 0, len(groups))
	for _, group := range groups {
		groupMap, ok := group.(map[string]any)
		if !ok {
			out = append(out, group)
			continue
		}
		hooks, ok := groupMap["hooks"].([]any)
		if !ok {
			out = append(out, group)
			continue
		}
		nextHooks, groupChanged := normalizeAgentEventGroup(hooks, spec, &seenCurrent)
		if len(nextHooks) == 0 {
			changed = true
			continue
		}
		if groupChanged {
			groupMap["hooks"] = nextHooks
			changed = true
		}
		out = append(out, group)
	}
	if len(out) != len(groups) {
		changed = true
	}
	return out, changed
}

func normalizeAgentEventGroup(hooks []any, spec hookSpec, seenCurrent *bool) ([]any, bool) {
	changed := false
	nextHooks := make([]any, 0, len(hooks))
	for _, hook := range hooks {
		if shouldDropAgentEventHook(hook, spec, seenCurrent) {
			changed = true
			continue
		}
		nextHooks = append(nextHooks, hook)
	}
	return nextHooks, changed
}

func shouldDropAgentEventHook(hook any, spec hookSpec, seenCurrent *bool) bool {
	hookMap, ok := hook.(map[string]any)
	command, _ := hookMap["command"].(string)
	if !ok || hookMap["type"] != "command" || !isAgentEventCommand(command, spec) {
		return false
	}
	if command == spec.Command && !*seenCurrent {
		*seenCurrent = true
		return false
	}
	return true
}

func isAgentEventCommand(command string, spec hookSpec) bool {
	return strings.Contains(command, "hook agent-event") &&
		strings.Contains(command, "--agent "+spec.AgentID) &&
		strings.Contains(command, "--event "+spec.AgentEvent)
}

func dedupeCommandHooks(groups []any, command string) ([]any, bool) {
	seen := false
	changed := false
	out := make([]any, 0, len(groups))
	for _, group := range groups {
		groupMap, ok := group.(map[string]any)
		if !ok {
			out = append(out, group)
			continue
		}
		hooks, ok := groupMap["hooks"].([]any)
		if !ok {
			out = append(out, group)
			continue
		}
		nextHooks := make([]any, 0, len(hooks))
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if ok && hookMap["type"] == "command" && hookMap["command"] == command {
				if seen {
					changed = true
					continue
				}
				seen = true
			}
			nextHooks = append(nextHooks, hook)
		}
		if len(nextHooks) == 0 {
			changed = true
			continue
		}
		if len(nextHooks) != len(hooks) {
			groupMap["hooks"] = nextHooks
			changed = true
		}
		out = append(out, group)
	}
	if len(out) != len(groups) {
		changed = true
	}
	return out, changed
}

func replaceCommands(groups []any, oldCommands []string, newCommand string) bool {
	if len(oldCommands) == 0 {
		return false
	}
	replace := make(map[string]struct{}, len(oldCommands))
	for _, command := range oldCommands {
		replace[command] = struct{}{}
	}

	changed := false
	for _, group := range groups {
		groupMap, ok := group.(map[string]any)
		if !ok {
			continue
		}
		hooks, _ := groupMap["hooks"].([]any)
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			command, _ := hookMap["command"].(string)
			if _, ok := replace[command]; ok {
				hookMap["command"] = newCommand
				changed = true
			}
		}
	}
	return changed
}

func hasCommand(groups []any, command string) bool {
	for _, group := range groups {
		groupMap, ok := group.(map[string]any)
		if !ok {
			continue
		}
		hooks, _ := groupMap["hooks"].([]any)
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			if hookMap["type"] == "command" && hookMap["command"] == command {
				return true
			}
		}
	}
	return false
}

func eventHook(agent, hookEvent, matcher, event, state string, bell bool) hookSpec {
	return hookSpec{
		Event:           hookEvent,
		Matcher:         matcher,
		Command:         agentEventCommand(agent, event, state, bell),
		ReplaceCommands: commandReplacements(agent, event, state, bell, true),
		AgentID:         agent,
		AgentEvent:      event,
	}
}

func agentEventCommand(agent, event, state string, bell bool) string {
	return agentenv.WrapHookCommand(agent, rawAgentEventCommand(agent, event, state, bell, true))
}

func rawAgentEventCommand(agent, event, state string, bell, stdinJSON bool) string {
	parts := []string{kitmuxCommand(), "hook", "agent-event", "--agent", agent, "--event", event}
	if state != "" {
		parts = append(parts, "--state", state)
	}
	if bell {
		parts = append(parts, "--bell")
	}
	if stdinJSON {
		parts = append(parts, "--stdin-json")
	}
	return strings.Join(parts, " ")
}

func kitmuxCommand() string {
	exe, err := os.Executable()
	if err == nil && filepath.Base(exe) == "kitmux" {
		return exe
	}
	return "kitmux"
}

func stateCommand(state string) string {
	return fmt.Sprintf("kitmux hook agent-state --state %s", state)
}

func bellCommand(state string) string {
	return stateCommand(state) + " --bell"
}

func commandReplacements(agent, event, state string, bell, stdinJSON bool) []string {
	states := []string{state}
	if state == statePermission {
		states = append(states, stateInput)
	}
	var commands []string
	seen := make(map[string]struct{})
	add := func(values []string) {
		for _, value := range values {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			commands = append(commands, value)
		}
	}
	if bell {
		for _, state := range states {
			add(bellReplacements(state))
		}
		add(agentEventReplacements(agent, event, state, bell, stdinJSON))
		return commands
	}
	for _, state := range states {
		add(stateReplacements(state))
	}
	add(agentEventReplacements(agent, event, state, bell, stdinJSON))
	return commands
}

func agentEventReplacements(agent, event, state string, bell, stdinJSON bool) []string {
	return []string{
		rawAgentEventCommand(agent, event, state, bell, stdinJSON),
		legacyAmbientAgentEventCommand(agent, event, state, bell, stdinJSON),
	}
}

func legacyAmbientAgentEventCommand(agent, event, state string, bell, stdinJSON bool) string {
	parts := []string{
		agentenv.AgentIDKey + "=" + hookShellQuote(agent),
		agentenv.TmuxSessionKey + `="${` + agentenv.TmuxSessionKey +
			`:-$(tmux display-message -p '#{session_name}' 2>/dev/null)}"`,
		agentenv.TmuxPaneKey + `="${` + agentenv.TmuxPaneKey +
			`:-${TMUX_PANE:-$(tmux display-message -p '#{pane_id}' 2>/dev/null)}}"`,
		agentenv.TmuxThreadKey + `="${` + agentenv.TmuxThreadKey +
			`:-$(tmux display-message -p '#{@kitmux_thread}' 2>/dev/null)}"`,
		rawAgentEventCommand(agent, event, state, bell, stdinJSON),
	}
	return strings.Join(parts, " ")
}

func hookShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func stateReplacements(state string) []string {
	return []string{legacyStateCommand(state), stateCommand(state)}
}

func bellReplacements(state string) []string {
	return []string{legacyBellCommand, legacyStateBellCommand(state), bellCommand(state)}
}

func legacyStateCommand(state string) string {
	return fmt.Sprintf(`sh -c 'tmux set-option -q @kitmux_agent_state %s 2>/dev/null || true'`, state)
}

func legacyStateBellCommand(state string) string {
	return fmt.Sprintf(
		`sh -c 'tmux set-option -q @kitmux_agent_state %s 2>/dev/null || true; `+
			`printf "\007" > /dev/tty 2>/dev/null || printf "\007"'`,
		state,
	)
}

func openCodePlugin(kitmux string) string {
	kitmuxJSON, _ := json.Marshal(kitmux)
	return `export const KitmuxZedBell = async () => {
  const kitmux = ` + string(kitmuxJSON) + `

  const setEvent = async (event, state, bell = false) => {
    const args = [kitmux, "hook", "agent-event", "--agent", "opencode", "--event", event, "--state", state]
    if (bell) {
      args.push("--bell")
    }
    try {
      const proc = Bun.spawn(args, {
        stdout: "inherit",
        stderr: "ignore",
      })
      const code = await proc.exited
      if (bell && code !== 0) {
        process.stdout.write("\x07")
      }
    } catch {
      if (bell) {
        process.stdout.write("\x07")
      }
    }
  }

  return {
    event: async ({ event }) => {
      if (event.type === "permission.asked") {
        await setEvent("permission.asked", "permission", true)
      } else if (event.type === "session.idle") {
        await setEvent("session.idle", "idle", true)
      } else if (event.type === "session.error") {
        await setEvent("session.error", "error", true)
      } else if (event.type === "permission.replied") {
        await setEvent("permission.replied", "working")
      }
    },
    "tool.execute.before": async () => {
      await setEvent("tool.execute.before", "working")
    },
    "tool.execute.after": async () => {
      await setEvent("tool.execute.after", "working")
    },
  }
}
`
}
