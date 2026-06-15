package agenthooks

import (
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
	assertJSONHook(t, factoryPath, "UserPromptSubmit", stateCommand("working"))
	assertJSONHook(t, factoryPath, "PreToolUse", stateCommand("working"))
	assertJSONHook(t, factoryPath, "Notification", bellCommand("input"))
	assertJSONHook(t, factoryPath, "Stop", bellCommand("idle"))

	claudePath := filepath.Join(home, ".claude", "settings.json")
	assertJSONHook(t, claudePath, "UserPromptSubmit", stateCommand("working"))
	assertJSONHook(t, claudePath, "PreToolUse", stateCommand("working"))
	assertJSONHook(t, claudePath, "PermissionRequest", bellCommand("input"))
	assertJSONHook(t, claudePath, "Notification", bellCommand("input"))
	assertJSONHook(t, claudePath, "Stop", bellCommand("idle"))
	claude := readJSONFile(t, claudePath)
	if claude["preferredNotifChannel"] != "terminal_bell" {
		t.Fatalf("preferredNotifChannel = %#v", claude["preferredNotifChannel"])
	}

	codexPath := filepath.Join(home, ".codex", "hooks.json")
	assertJSONHook(t, codexPath, "UserPromptSubmit", stateCommand("working"))
	assertJSONHook(t, codexPath, "PreToolUse", stateCommand("working"))
	assertJSONHook(t, codexPath, "PermissionRequest", bellCommand("input"))
	assertJSONHook(t, codexPath, "Stop", bellCommand("idle"))

	pluginPath := filepath.Join(home, ".config", "opencode", "plugins", "kitmux-zed-bell.js")
	// #nosec G304 -- test path is under t.TempDir.
	plugin, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("read opencode plugin: %v", err)
	}
	if string(plugin) != openCodePlugin() {
		t.Fatalf("unexpected opencode plugin:\n%s", string(plugin))
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
							"command": legacyStateBellCommand("input"),
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
	assertJSONHook(t, path, "PermissionRequest", bellCommand("input"))
	if hasJSONCommand(t, path, "PermissionRequest", legacyStateBellCommand("input")) {
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
							"command": legacyStateCommand("working"),
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
	assertJSONHook(t, path, "UserPromptSubmit", stateCommand("working"))
	if hasJSONCommand(t, path, "UserPromptSubmit", legacyStateCommand("working")) {
		t.Fatalf("legacy working command was not upgraded")
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
	assertJSONHook(t, filepath.Join(home, ".factory", "hooks.json"), "Notification", bellCommand("input"))
	assertMissing(t, filepath.Join(home, ".claude", "settings.json"))
	assertMissing(t, filepath.Join(home, ".codex", "hooks.json"))
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
	assertJSONHook(t, filepath.Join(home, ".codex", "hooks.json"), "Stop", bellCommand("idle"))
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
