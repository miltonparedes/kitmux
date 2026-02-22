package openlocal

import "testing"

func TestResolveEditor(t *testing.T) {
	t.Setenv("KITMUX_EDITOR", "")
	if got := ResolveEditor(); got != defaultEditor {
		t.Fatalf("default: got %q, want %q", got, defaultEditor)
	}

	t.Setenv("KITMUX_EDITOR", "vscode")
	if got := ResolveEditor(); got != EditorVSCode {
		t.Fatalf("env=vscode: got %q", got)
	}

	t.Setenv("KITMUX_EDITOR", "zed")
	if got := ResolveEditor(); got != EditorZed {
		t.Fatalf("env=zed: got %q", got)
	}

	t.Setenv("KITMUX_EDITOR", "invalid")
	if got := ResolveEditor(); got != defaultEditor {
		t.Fatalf("invalid: got %q, want default %q", got, defaultEditor)
	}
}

func TestResolveSSHHost_EnvOverride(t *testing.T) {
	t.Setenv("KITMUX_SSH_HOST", "remotehost")
	if got := ResolveSSHHost(); got != "remotehost" {
		t.Fatalf("got %q, want remotehost", got)
	}
}

func TestResolveSSHHost_Empty(t *testing.T) {
	t.Setenv("KITMUX_SSH_HOST", "")
	t.Setenv("HOME", t.TempDir())
	if got := ResolveSSHHost(); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestResolveSocketPath(t *testing.T) {
	t.Setenv("KITMUX_OPEN_EDITOR_SOCK", "")
	if got := ResolveSocketPath(); got != defaultSocket {
		t.Fatalf("got %q, want %q", got, defaultSocket)
	}

	t.Setenv("KITMUX_OPEN_EDITOR_SOCK", "/custom/path.sock")
	if got := ResolveSocketPath(); got != "/custom/path.sock" {
		t.Fatalf("got %q", got)
	}
}

func TestCacheAndLoadSSHHost(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := CacheSSHHost("test-host"); err != nil {
		t.Fatalf("cache: %v", err)
	}

	t.Setenv("KITMUX_SSH_HOST", "")
	if got := loadCachedHost(); got != "test-host" {
		t.Fatalf("got %q, want test-host", got)
	}
}
