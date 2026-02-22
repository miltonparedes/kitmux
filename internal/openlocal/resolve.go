package openlocal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

const (
	EditorZed    = "zed"
	EditorVSCode = "vscode"

	defaultEditor = EditorZed
	defaultSocket = "/tmp/kitmux-bridge.sock"
	cacheDir      = ".config/kitmux/open-local-editor"
	hostsFile     = "hosts.json"
)

// hostsCache maps hostname/IP to SSH alias used by editors.
type hostsCache struct {
	Hosts map[string]string `json:"hosts"`
}

// ResolveEditor returns the configured editor or the default.
func ResolveEditor() string {
	if e := os.Getenv("KITMUX_EDITOR"); e == EditorZed || e == EditorVSCode {
		return e
	}
	return defaultEditor
}

// ResolveSocketPath returns the bridge socket path.
func ResolveSocketPath() string {
	if s := os.Getenv("KITMUX_OPEN_EDITOR_SOCK"); s != "" {
		return s
	}
	return defaultSocket
}

// ResolveCurrentSessionPath returns the working directory of the current tmux session.
func ResolveCurrentSessionPath() (string, error) {
	name, err := tmux.CurrentSession()
	if err != nil {
		return "", fmt.Errorf("current session: %w", err)
	}
	sessions, err := tmux.ListSessions()
	if err != nil {
		return "", fmt.Errorf("list sessions: %w", err)
	}
	for _, s := range sessions {
		if s.Name == name {
			if s.Path == "" {
				return "", fmt.Errorf("session %q has no path", name)
			}
			return s.Path, nil
		}
	}
	return "", fmt.Errorf("session %q not found", name)
}

// ResolveSSHHost returns the SSH host alias with priority:
// 1. KITMUX_SSH_HOST env var
// 2. Cached value from hosts.json
// 3. Empty string (caller should prompt or fallback)
func ResolveSSHHost() string {
	if h := os.Getenv("KITMUX_SSH_HOST"); h != "" {
		return h
	}
	if h := loadCachedHost(); h != "" {
		return h
	}
	return ""
}

// CacheSSHHost persists the SSH host alias for future use.
func CacheSSHHost(host string) error {
	dir := cacheFilePath()
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return err
	}

	cache := hostsCache{Hosts: map[string]string{"default": host}}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dir, data, 0o600)
}

func loadCachedHost() string {
	data, err := os.ReadFile(cacheFilePath())
	if err != nil {
		return ""
	}
	var cache hostsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return ""
	}
	return cache.Hosts["default"]
}

func cacheFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, cacheDir, hostsFile)
}
