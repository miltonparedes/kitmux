package agenthooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const bellCommand = `sh -c 'printf "\007" > /dev/tty 2>/dev/null || printf "\007"'`

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

func installDroid(home string) (Result, error) {
	path := filepath.Join(home, ".factory", "hooks.json")
	changed, err := installJSONBellHooks(path, []string{"Notification", "Stop"})
	return Result{AgentID: "droid", Path: path, Changed: changed}, err
}

func installClaude(home string) (Result, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	doc, err := readJSON(path)
	if err != nil {
		return Result{AgentID: "claude", Path: path}, err
	}
	changed := setString(doc, "preferredNotifChannel", "terminal_bell")
	if addBellHooks(doc, []string{"Notification", "Stop"}) {
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
	changed, err := installJSONBellHooks(path, []string{"PermissionRequest", "Stop"})
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

func installJSONBellHooks(path string, events []string) (bool, error) {
	doc, err := readJSON(path)
	if err != nil {
		return false, err
	}
	changed := addBellHooks(doc, events)
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

func addBellHooks(doc map[string]any, events []string) bool {
	rawHooks, _ := doc["hooks"].(map[string]any)
	if rawHooks == nil {
		rawHooks = make(map[string]any)
		doc["hooks"] = rawHooks
	}

	changed := false
	for _, event := range events {
		if addBellHook(rawHooks, event) {
			changed = true
		}
	}
	return changed
}

func addBellHook(rawHooks map[string]any, event string) bool {
	groups, _ := rawHooks[event].([]any)
	if hasCommand(groups, bellCommand) {
		return false
	}

	group := map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": bellCommand,
				"timeout": float64(5),
			},
		},
	}
	rawHooks[event] = append(groups, group)
	return true
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

func openCodePlugin() string {
	return `export const KitmuxZedBell = async () => {
  const bell = () => process.stdout.write("\x07")

  return {
    event: async ({ event }) => {
      if (event.type === "session.idle" || event.type === "permission.asked") {
        bell()
      }
    },
  }
}
`
}
