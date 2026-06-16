package agenthooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miltonparedes/kitmux/internal/agentenv"
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
	assertAgentEventShim(t, home)

	factoryPath := filepath.Join(home, ".factory", "hooks.json")
	assertJSONHook(t, factoryPath, "UserPromptSubmit", shimAgentEventCommand(home, "droid", "user-prompt-submit"))
	assertJSONHook(t, factoryPath, "PreToolUse", shimAgentEventCommand(home, "droid", "pre-tool-use"))
	assertJSONHook(t, factoryPath, "PostToolUse", shimAgentEventCommand(home, "droid", "post-tool-use"))
	assertJSONHook(t, factoryPath, "Notification", shimAgentEventCommand(home, "droid", "notification"))
	assertJSONHook(t, factoryPath, "Stop", shimAgentEventCommand(home, "droid", "stop"))
	assertJSONHook(t, factoryPath, "SessionEnd", shimAgentEventCommand(home, "droid", "session-end"))
	assertNoInlineHookState(t, factoryPath)
	assertMissing(t, filepath.Join(home, ".factory", "settings.json"))

	claudePath := filepath.Join(home, ".claude", "settings.json")
	assertJSONHook(t, claudePath, "UserPromptSubmit", shimAgentEventCommand(home, "claude", "user-prompt-submit"))
	assertJSONHook(t, claudePath, "PreToolUse", shimAgentEventCommand(home, "claude", "pre-tool-use"))
	assertJSONHook(t, claudePath, "PermissionRequest", shimAgentEventCommand(home, "claude", "permission-request"))
	assertJSONHook(t, claudePath, "PermissionDenied", shimAgentEventCommand(home, "claude", "permission-denied"))
	assertJSONHook(t, claudePath, "Elicitation", shimAgentEventCommand(home, "claude", "elicitation"))
	assertJSONHook(t, claudePath, "Notification", shimAgentEventCommand(home, "claude", "notification"))
	assertJSONHook(t, claudePath, "Stop", shimAgentEventCommand(home, "claude", "stop"))
	claude := readJSONFile(t, claudePath)
	if claude["preferredNotifChannel"] != "terminal_bell" {
		t.Fatalf("preferredNotifChannel = %#v", claude["preferredNotifChannel"])
	}
	assertNoInlineHookState(t, claudePath)

	codexPath := filepath.Join(home, ".codex", "hooks.json")
	assertJSONHook(t, codexPath, "SessionStart", shimAgentEventCommand(home, "codex", "session-start"))
	assertJSONHook(t, codexPath, "UserPromptSubmit", shimAgentEventCommand(home, "codex", "user-prompt-submit"))
	assertJSONHook(t, codexPath, "PreToolUse", shimAgentEventCommand(home, "codex", "pre-tool-use"))
	assertJSONHook(t, codexPath, "PermissionRequest", shimAgentEventCommand(home, "codex", "permission-request"))
	assertJSONHook(t, codexPath, "PostToolUse", shimAgentEventCommand(home, "codex", "post-tool-use"))
	assertJSONHook(t, codexPath, "Stop", shimAgentEventCommand(home, "codex", "stop"))
	assertNoInlineHookState(t, codexPath)

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
	assertJSONHook(t, path, "PermissionRequest", shimAgentEventCommand(home, "codex", "permission-request"))
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
	assertJSONHook(t, path, "UserPromptSubmit", shimAgentEventCommand(home, "droid", "user-prompt-submit"))
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
	command := shimAgentEventCommand(home, "droid", "pre-tool-use")
	assertJSONHook(t, path, "PreToolUse", command)
	if hasJSONCommand(t, path, "PreToolUse", rawAgentEventCommand("droid", "pre-tool-use", stateWorking, false, true)) {
		t.Fatalf("unwrapped agent-event command was not upgraded")
	}
	if countPreToolUseJSONCommand(t, path, command) != 1 {
		t.Fatalf("expected upgraded command once, got config %#v", readJSONFile(t, path))
	}
}

func TestInstallAllUpgradesAmbientTmuxAgentEventHooks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".factory", "hooks.json")
	oldCommand := legacyAmbientAgentEventCommand("droid", "pre-tool-use", stateWorking, false, true)
	oldRawCommand := rawAgentEventCommand("droid", "pre-tool-use", stateWorking, false, true)
	oldWrappedCommand := agentenv.WrapHookCommand("droid", oldRawCommand)
	command := shimAgentEventCommand(home, "droid", "pre-tool-use")
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
							"command": oldWrappedCommand,
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
	if hasJSONCommand(t, path, "PreToolUse", oldWrappedCommand) {
		t.Fatalf("wrapped agent-event command was not upgraded")
	}
	if countPreToolUseJSONCommand(t, path, command) != 1 {
		t.Fatalf("expected upgraded command once, got config %#v", readJSONFile(t, path))
	}
	if countPreToolUseJSONCommand(t, path, customCommand) != 1 {
		t.Fatalf("custom command was removed, got config %#v", readJSONFile(t, path))
	}
}

func TestInstallDroidUsesHooksJSONAndCleansSettingsHooks(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".factory", "settings.json")
	oldCommand := agentenv.WrapHookCommand("droid", rawAgentEventCommand("droid", "pre-tool-use", stateWorking, false, true))
	customCommand := "echo keep-custom-hook"
	settings := map[string]any{
		"enableHooks": true,
		"customKey":   "keep-me",
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
							"command": customCommand,
						},
					},
				},
			},
		},
	}
	if err := writeJSON(settingsPath, settings); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	result, err := Install("droid", home)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.Path != filepath.Join(home, ".factory", "hooks.json") || !result.Changed {
		t.Fatalf("Install() result = %#v", result)
	}
	assertJSONHook(t, result.Path, "PreToolUse", shimAgentEventCommand(home, "droid", "pre-tool-use"))
	updatedSettings := readJSONFile(t, settingsPath)
	if updatedSettings["enableHooks"] != true || updatedSettings["customKey"] != "keep-me" {
		t.Fatalf("settings keys were not preserved: %#v", updatedSettings)
	}
	if hasJSONCommand(t, settingsPath, "PreToolUse", oldCommand) {
		t.Fatalf("old droid settings hook was not cleaned")
	}
	if hasJSONCommand(t, settingsPath, "PreToolUse", shimAgentEventCommand(home, "droid", "pre-tool-use")) {
		t.Fatalf("droid settings retained canonical hook")
	}
	if countPreToolUseJSONCommand(t, settingsPath, customCommand) != 1 {
		t.Fatalf("custom settings hook was not preserved: %#v", readJSONFile(t, settingsPath))
	}

	result, err = Install("droid", home)
	if err != nil {
		t.Fatalf("second Install() error = %v", err)
	}
	if result.Changed {
		t.Fatalf("droid install changed on second run")
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
	assertJSONHook(t, filepath.Join(home, ".factory", "hooks.json"), "Notification", shimAgentEventCommand(home, "droid", "notification"))
	assertMissing(t, filepath.Join(home, ".factory", "settings.json"))
	assertMissing(t, filepath.Join(home, ".claude", "settings.json"))
	assertMissing(t, filepath.Join(home, ".codex", "hooks.json"))
}

func countPreToolUseJSONCommand(t *testing.T, path, command string) int {
	t.Helper()
	doc := readJSONFile(t, path)
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		return 0
	}
	groups, ok := hooks["PreToolUse"].([]any)
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
	assertJSONHook(t, filepath.Join(home, ".codex", "hooks.json"), "Stop", shimAgentEventCommand(home, "codex", "stop"))
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

func assertAgentEventShim(t *testing.T, home string) {
	t.Helper()
	info, err := os.Stat(agentEventShimPath(home))
	if err != nil {
		t.Fatalf("stat shim: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("shim is not executable: %s", info.Mode().Perm())
	}
}

func assertNoInlineHookState(t *testing.T, path string) {
	t.Helper()
	doc := readJSONFile(t, path)
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		return
	}
	for event, rawGroups := range hooks {
		groups, _ := rawGroups.([]any)
		for _, group := range groups {
			groupMap, _ := group.(map[string]any)
			rawHooks, _ := groupMap["hooks"].([]any)
			for _, hook := range rawHooks {
				hookMap, _ := hook.(map[string]any)
				command, _ := hookMap["command"].(string)
				if strings.Contains(command, "hook agent-event") || strings.Contains(command, "KITMUX_TMUX_") ||
					strings.Contains(command, " --state ") ||
					strings.Contains(command, " --bell") {
					t.Fatalf("%s %s has inline tracking/state command: %q", path, event, command)
				}
			}
		}
	}
}

func shimAgentEventCommand(home, agent, event string) string {
	return agentEventCommand(agentEventShimPath(home), agent, event)
}

func agentEventShimPath(home string) string {
	return filepath.Join(home, ".config", "kitmux", "hooks", "agent-event")
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
