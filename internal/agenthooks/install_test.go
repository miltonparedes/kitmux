package agenthooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallAllWritesSupportedAgentHooks(t *testing.T) {
	home := t.TempDir()

	results, err := InstallAll(home)
	if err != nil {
		t.Fatalf("InstallAll() error = %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("len(results) = %d", len(results))
	}

	factoryPath := filepath.Join(home, ".factory", "hooks.json")
	assertJSONHook(t, factoryPath, "UserPromptSubmit", agentEventCommand("droid", "user-prompt-submit", stateWorking, false))
	assertJSONHook(t, factoryPath, "PreToolUse", agentEventCommand("droid", "pre-tool-use", stateWorking, false))
	assertJSONHook(t, factoryPath, "PostToolUse", agentEventCommand("droid", "post-tool-use", stateWorking, false))
	assertJSONHook(t, factoryPath, "Notification", agentEventCommand("droid", "notification", stateInput, true))
	assertJSONHook(t, factoryPath, "Stop", agentEventCommand("droid", "stop", stateIdle, true))
	assertJSONHook(t, factoryPath, "SessionEnd", agentEventCommand("droid", "session-end", stateIdle, false))

	claudePath := filepath.Join(home, ".claude", "settings.json")
	assertJSONHook(t, claudePath, "UserPromptSubmit", agentEventCommand("claude", "user-prompt-submit", stateWorking, false))
	assertJSONHook(t, claudePath, "PreToolUse", agentEventCommand("claude", "pre-tool-use", stateWorking, false))
	assertJSONHook(t, claudePath, "PermissionRequest", agentEventCommand("claude", "permission-request", statePermission, true))
	assertJSONHook(t, claudePath, "PermissionDenied", agentEventCommand("claude", "permission-denied", stateError, true))
	assertJSONHook(t, claudePath, "Elicitation", agentEventCommand("claude", "elicitation", stateInput, true))
	assertJSONHook(t, claudePath, "Notification", agentEventCommand("claude", "notification", stateInput, true))
	assertJSONHook(t, claudePath, "Stop", agentEventCommand("claude", "stop", stateIdle, true))
	claude := readJSONFile(t, claudePath)
	if claude["preferredNotifChannel"] != "terminal_bell" {
		t.Fatalf("preferredNotifChannel = %#v", claude["preferredNotifChannel"])
	}

	codexPath := filepath.Join(home, ".codex", "hooks.json")
	assertJSONHook(t, codexPath, "SessionStart", agentEventCommand("codex", "session-start", stateIdle, false))
	assertJSONHook(t, codexPath, "UserPromptSubmit", agentEventCommand("codex", "user-prompt-submit", stateWorking, false))
	assertJSONHook(t, codexPath, "PreToolUse", agentEventCommand("codex", "pre-tool-use", stateWorking, false))
	assertJSONHook(t, codexPath, "PermissionRequest", agentEventCommand("codex", "permission-request", statePermission, true))
	assertJSONHook(t, codexPath, "PostToolUse", agentEventCommand("codex", "post-tool-use", stateWorking, false))
	assertJSONHook(t, codexPath, "Stop", agentEventCommand("codex", "stop", stateIdle, true))

	pluginPath := filepath.Join(home, ".config", "opencode", "plugins", "kitmux-zed-bell.js")
	// #nosec G304 -- test path is under t.TempDir.
	plugin, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("read opencode plugin: %v", err)
	}
	if string(plugin) != openCodePlugin(kitmuxCommand()) {
		t.Fatalf("unexpected opencode plugin:\n%s", string(plugin))
	}
	if !bytes.Contains(plugin, []byte(`const kitmux = "kitmux"`)) {
		t.Fatalf("opencode plugin missing kitmux command constant:\n%s", string(plugin))
	}
}

func TestInstallAllIsIdempotent(t *testing.T) {
	home := t.TempDir()
	if _, err := InstallAll(home); err != nil {
		t.Fatalf("first InstallAll() error = %v", err)
	}
	results, err := InstallAll(home)
	if err != nil {
		t.Fatalf("second InstallAll() error = %v", err)
	}
	for _, result := range results {
		if result.Changed {
			t.Fatalf("%s changed on second install", result.AgentID)
		}
	}
}

func TestInstallAllUpgradesLegacyBellHooks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "hooks.json")
	legacyDoc := map[string]any{
		"hooks": map[string]any{
			"PermissionRequest": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": legacyStateBellCommand(stateInput),
						},
					},
				},
			},
		},
	}
	if err := writeJSON(path, legacyDoc); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	if _, err := InstallAll(home); err != nil {
		t.Fatalf("InstallAll() error = %v", err)
	}
	assertJSONHook(t, path, "PermissionRequest", agentEventCommand("codex", "permission-request", statePermission, true))
	if hasJSONCommand(t, path, "PermissionRequest", legacyStateBellCommand(stateInput)) {
		t.Fatalf("legacy bell command was not upgraded")
	}
}

func TestInstallAllUpgradesLegacyWorkingHooks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".factory", "hooks.json")
	legacyDoc := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": legacyStateCommand(stateWorking),
						},
					},
				},
			},
		},
	}
	if err := writeJSON(path, legacyDoc); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	if _, err := InstallAll(home); err != nil {
		t.Fatalf("InstallAll() error = %v", err)
	}
	assertJSONHook(t, path, "UserPromptSubmit", agentEventCommand("droid", "user-prompt-submit", stateWorking, false))
	if hasJSONCommand(t, path, "UserPromptSubmit", legacyStateCommand(stateWorking)) {
		t.Fatalf("legacy working command was not upgraded")
	}
}

func TestInstallAllUpgradesUnwrappedAgentEventHooks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".factory", "hooks.json")
	legacyDoc := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "*",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": rawAgentEventCommand("droid", "pre-tool-use", stateWorking, false, true),
						},
					},
				},
			},
		},
	}
	if err := writeJSON(path, legacyDoc); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	if _, err := InstallAll(home); err != nil {
		t.Fatalf("InstallAll() error = %v", err)
	}
	command := agentEventCommand("droid", "pre-tool-use", stateWorking, false)
	assertJSONHook(t, path, "PreToolUse", command)
	if hasJSONCommand(t, path, "PreToolUse", rawAgentEventCommand("droid", "pre-tool-use", stateWorking, false, true)) {
		t.Fatalf("unwrapped agent-event command was not upgraded")
	}
	if countJSONCommand(t, path, "PreToolUse", command) != 1 {
		t.Fatalf("expected upgraded command once, got config %#v", readJSONFile(t, path))
	}
}

func TestInstallAllUpgradesAmbientTmuxAgentEventHooks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".factory", "hooks.json")
	oldCommand := legacyAmbientAgentEventCommand("droid", "pre-tool-use", stateWorking, false, true)
	oldRawCommand := rawAgentEventCommand("droid", "pre-tool-use", stateWorking, false, true)
	command := agentEventCommand("droid", "pre-tool-use", stateWorking, false)
	customCommand := "echo keep-custom-hook"
	legacyDoc := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "*",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": oldCommand,
						},
						map[string]any{
							"type":    "command",
							"command": oldRawCommand,
						},
						map[string]any{
							"type":    "command",
							"command": command,
						},
						map[string]any{
							"type":    "command",
							"command": customCommand,
						},
					},
				},
			},
		},
	}
	if err := writeJSON(path, legacyDoc); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	if _, err := InstallAll(home); err != nil {
		t.Fatalf("InstallAll() error = %v", err)
	}
	assertJSONHook(t, path, "PreToolUse", command)
	if hasJSONCommand(t, path, "PreToolUse", oldCommand) {
		t.Fatalf("ambient tmux agent-event command was not upgraded")
	}
	if hasJSONCommand(t, path, "PreToolUse", oldRawCommand) {
		t.Fatalf("raw agent-event command was not upgraded")
	}
	if countJSONCommand(t, path, "PreToolUse", command) != 1 {
		t.Fatalf("expected upgraded command once, got config %#v", readJSONFile(t, path))
	}
	if countJSONCommand(t, path, "PreToolUse", customCommand) != 1 {
		t.Fatalf("custom command was removed, got config %#v", readJSONFile(t, path))
	}
}

func TestInstallDroidEnablesAndWritesSettingsHooks(t *testing.T) {
	home := t.TempDir()
	if _, err := Install("droid", home); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	settingsPath := filepath.Join(home, ".factory", "settings.json")
	settings := readJSONFile(t, settingsPath)
	if settings["enableHooks"] != true {
		t.Fatalf("enableHooks = %#v", settings["enableHooks"])
	}
	assertJSONHook(t, settingsPath, "UserPromptSubmit", agentEventCommand("droid", "user-prompt-submit", stateWorking, false))
	assertJSONHook(t, settingsPath, "Stop", agentEventCommand("droid", "stop", stateIdle, true))
	assertJSONHook(t, settingsPath, "Notification", agentEventCommand("droid", "notification", stateInput, true))

	// Idempotent: existing settings keys are preserved and no spurious change.
	settings["customKey"] = "keep-me"
	if err := writeJSON(settingsPath, settings); err != nil {
		t.Fatalf("rewrite settings: %v", err)
	}
	result, err := Install("droid", home)
	if err != nil {
		t.Fatalf("second Install() error = %v", err)
	}
	if result.Changed {
		t.Fatalf("droid install changed on second run")
	}
	if readJSONFile(t, settingsPath)["customKey"] != "keep-me" {
		t.Fatalf("existing settings key was dropped")
	}
}

func TestInstallWritesOnlyRequestedAgentHooks(t *testing.T) {
	home := t.TempDir()

	result, err := Install("droid", home)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.AgentID != "droid" || !result.Changed {
		t.Fatalf("Install() result = %#v", result)
	}
	assertJSONHook(t, filepath.Join(home, ".factory", "hooks.json"), "Notification", agentEventCommand("droid", "notification", stateInput, true))
	assertMissing(t, filepath.Join(home, ".claude", "settings.json"))
	assertMissing(t, filepath.Join(home, ".codex", "hooks.json"))
}

func countJSONCommand(t *testing.T, path, event, command string) int {
	t.Helper()
	doc := readJSONFile(t, path)
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		return 0
	}
	groups, ok := hooks[event].([]any)
	if !ok {
		return 0
	}
	var count int
	for _, group := range groups {
		groupMap, ok := group.(map[string]any)
		if !ok {
			continue
		}
		hooks, _ := groupMap["hooks"].([]any)
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if ok && hookMap["type"] == "command" && hookMap["command"] == command {
				count++
			}
		}
	}
	return count
}

func TestInstallUsesCodexHooksForCodexCloud(t *testing.T) {
	home := t.TempDir()

	result, err := Install("codex-cloud", home)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.AgentID != "codex" {
		t.Fatalf("Install() result = %#v", result)
	}
	assertJSONHook(t, filepath.Join(home, ".codex", "hooks.json"), "Stop", agentEventCommand("codex", "stop", stateIdle, true))
}

func TestInstallUnsupportedAgent(t *testing.T) {
	_, err := Install("gemini", t.TempDir())
	if !errors.Is(err, ErrUnsupportedAgent) {
		t.Fatalf("expected ErrUnsupportedAgent, got %v", err)
	}
}

func assertJSONHook(t *testing.T, path, event, command string) {
	t.Helper()
	doc := readJSONFile(t, path)
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("%s missing hooks", path)
	}
	groups, ok := hooks[event].([]any)
	if !ok || !hasCommand(groups, command) {
		t.Fatalf("%s missing %s hook %q: %#v", path, event, command, hooks[event])
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be missing, stat err = %v", path, err)
	}
}

func hasJSONCommand(t *testing.T, path, event, command string) bool {
	t.Helper()
	doc := readJSONFile(t, path)
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		return false
	}
	groups, ok := hooks[event].([]any)
	return ok && hasCommand(groups, command)
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	// #nosec G304 -- test path is under t.TempDir.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}
