package agenthooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const legacyBellCommand = `sh -c 'printf "\007" > /dev/tty 2>/dev/null || printf "\007"'`

var ErrUnsupportedAgent = errors.New("unsupported agent hooks")

type hookSpec struct {
	Event           string
	Matcher         string
	Command         string
	ReplaceCommands []string
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
	path := filepath.Join(home, ".factory", "hooks.json")
	changed, err := installJSONHooks(path, []hookSpec{
		{Event: "UserPromptSubmit", Command: stateCommand("working"), ReplaceCommands: workingReplacements()},
		{Event: "PreToolUse", Matcher: "*", Command: stateCommand("working"), ReplaceCommands: workingReplacements()},
		{Event: "Notification", Command: bellCommand("input"), ReplaceCommands: bellReplacements("input")},
		{Event: "Stop", Command: bellCommand("idle"), ReplaceCommands: bellReplacements("idle")},
	})
	return Result{AgentID: "droid", Path: path, Changed: changed}, err
}

func installClaude(home string) (Result, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	doc, err := readJSON(path)
	if err != nil {
		return Result{AgentID: "claude", Path: path}, err
	}
	changed := setString(doc, "preferredNotifChannel", "terminal_bell")
	if addCommandHooks(doc, []hookSpec{
		{Event: "UserPromptSubmit", Command: stateCommand("working"), ReplaceCommands: workingReplacements()},
		{Event: "PreToolUse", Matcher: "*", Command: stateCommand("working"), ReplaceCommands: workingReplacements()},
		{Event: "PermissionRequest", Command: bellCommand("input"), ReplaceCommands: bellReplacements("input")},
		{Event: "Notification", Command: bellCommand("input"), ReplaceCommands: bellReplacements("input")},
		{Event: "Stop", Command: bellCommand("idle"), ReplaceCommands: bellReplacements("idle")},
		{Event: "StopFailure", Command: bellCommand("idle"), ReplaceCommands: bellReplacements("idle")},
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
		{Event: "UserPromptSubmit", Command: stateCommand("working"), ReplaceCommands: workingReplacements()},
		{Event: "PreToolUse", Command: stateCommand("working"), ReplaceCommands: workingReplacements()},
		{Event: "PermissionRequest", Command: bellCommand("input"), ReplaceCommands: bellReplacements("input")},
		{Event: "Stop", Command: bellCommand("idle"), ReplaceCommands: bellReplacements("idle")},
	})
	return Result{AgentID: "codex", Path: path, Changed: changed}, err
}

func installOpenCode(home string) (Result, error) {
	path := filepath.Join(home, ".config", "opencode", "plugins", "kitmux-zed-bell.js")
	content := []byte(openCodePlugin())
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

func stateCommand(state string) string {
	return fmt.Sprintf("kitmux hook agent-state --state %s", state)
}

func bellCommand(state string) string {
	return stateCommand(state) + " --bell"
}

func workingReplacements() []string {
	return []string{legacyStateCommand("working")}
}

func bellReplacements(state string) []string {
	return []string{legacyBellCommand, legacyStateBellCommand(state)}
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

func openCodePlugin() string {
	return `export const KitmuxZedBell = async () => {
  const setState = async (state, bell = false) => {
    const args = ["kitmux", "hook", "agent-state", "--state", state]
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
        await setState("input", true)
      } else if (event.type === "session.idle") {
        await setState("idle", true)
      } else if (event.type === "permission.replied") {
        await setState("working")
      }
    },
    "tool.execute.before": async () => {
      await setState("working")
    },
  }
}
`
}
