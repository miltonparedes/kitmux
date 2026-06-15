package agenthooks

import (
	"encoding/json"
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

	assertJSONHook(t, filepath.Join(home, ".factory", "hooks.json"), "Notification")
	assertJSONHook(t, filepath.Join(home, ".factory", "hooks.json"), "Stop")

	claudePath := filepath.Join(home, ".claude", "settings.json")
	assertJSONHook(t, claudePath, "Notification")
	assertJSONHook(t, claudePath, "Stop")
	claude := readJSONFile(t, claudePath)
	if claude["preferredNotifChannel"] != "terminal_bell" {
		t.Fatalf("preferredNotifChannel = %#v", claude["preferredNotifChannel"])
	}

	assertJSONHook(t, filepath.Join(home, ".codex", "hooks.json"), "PermissionRequest")
	assertJSONHook(t, filepath.Join(home, ".codex", "hooks.json"), "Stop")

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

func assertJSONHook(t *testing.T, path, event string) {
	t.Helper()
	doc := readJSONFile(t, path)
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("%s missing hooks", path)
	}
	groups, ok := hooks[event].([]any)
	if !ok || !hasCommand(groups, bellCommand) {
		t.Fatalf("%s missing %s bell hook: %#v", path, event, hooks[event])
	}
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
