package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/workspaces"
)

func TestLoadTree_DoesNotReaddHiddenWorkspaceFromActiveSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	repo := filepath.Join(t.TempDir(), "api")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	fakeBin := t.TempDir()
	writeExecutable(t, fakeBin, "tmux", fmt.Sprintf(`#!/bin/sh
set -eu
if [ "$1" = "list-sessions" ]; then
	printf 'api-main\t1\t0\t%s\t100\n'
	exit 0
fi
exit 1
`, repo))
	writeExecutable(t, fakeBin, "git", `#!/bin/sh
set -eu
if [ "$1" = "-C" ] && [ "$3" = "rev-parse" ] && [ "$4" = "--git-common-dir" ]; then
	printf '%s/.git\n' "$2"
	exit 0
fi
if [ "$1" = "-C" ] && [ "$3" = "branch" ] && [ "$4" = "--show-current" ]; then
	printf 'main\n'
	exit 0
fi
exit 1
`)
	prependPath(t, fakeBin)

	if err := workspaces.SaveRegistry(nil); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	msg, ok := loadTree().(treeLoadedMsg)
	if !ok {
		t.Fatalf("expected treeLoadedMsg, got %T", msg)
	}
	if len(msg.roots) != 0 {
		t.Fatalf("expected hidden workspace to stay hidden, got %d roots", len(msg.roots))
	}

	loaded := workspaces.LoadRegistry()
	if len(loaded) != 0 {
		t.Fatalf("expected hidden workspace not to be re-added, got %+v", loaded)
	}
}

func TestHandleConfirm_RemovesSelectedWorkspaceByPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	first := filepath.Join(t.TempDir(), "acme", "api")
	second := filepath.Join(t.TempDir(), "internal", "api")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	if err := workspaces.SaveRegistry([]workspaces.Workspace{
		{Name: "api", Path: first, AddedAt: 1, LastSeenAt: 1},
		{Name: "api", Path: second, AddedAt: 2, LastSeenAt: 2},
	}); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	m := New()
	m.roots = buildProjectTree([]workspaces.Workspace{
		{Name: "api", Path: first, AddedAt: 1, LastSeenAt: 1},
		{Name: "api", Path: second, AddedAt: 2, LastSeenAt: 2},
	}, nil, nil)
	m.rebuildVisible()
	m.cursor = 1

	if _, cmd := m.handleConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}); cmd == nil {
		t.Fatal("expected reload command after confirmation")
	}

	loaded := workspaces.LoadRegistry()
	if len(loaded) != 1 || loaded[0].Path != first {
		t.Fatalf("expected only selected workspace to be removed, got %+v", loaded)
	}
}

func TestEnterWorktreePicker_UsesWorkspacePath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	first := filepath.Join(t.TempDir(), "acme", "api")
	second := filepath.Join(t.TempDir(), "internal", "api")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	if err := workspaces.SaveRegistry([]workspaces.Workspace{
		{Name: "api", Path: first, AddedAt: 1, LastSeenAt: 1},
		{Name: "api", Path: second, AddedAt: 2, LastSeenAt: 2},
	}); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	fakeBin := t.TempDir()
	writeExecutable(t, fakeBin, "tmux", `#!/bin/sh
exit 0
`)
	writeExecutable(t, fakeBin, "wt", fmt.Sprintf(`#!/bin/sh
set -eu
if [ "$1" = "list" ] && [ "$2" = "--format=json" ]; then
	case "$PWD" in
	%s)
		printf '[{"branch":"main","path":"%s","is_main":true,"working_tree":{"diff":{"added":0,"deleted":0}}}]'
		;;
	%s)
		printf '[{"branch":"feature","path":"%s","is_main":false,"working_tree":{"diff":{"added":1,"deleted":2}}}]'
		;;
	*)
		printf '[]'
		;;
	esac
	exit 0
fi
exit 1
`, first, first, second, second))
	prependPath(t, fakeBin)

	cmd := New().enterWorktreePicker(second)
	if cmd == nil {
		t.Fatal("expected worktree picker command")
	}

	msg := cmd()
	loaded, ok := msg.(wtLoadedMsg)
	if !ok {
		t.Fatalf("expected wtLoadedMsg, got %T", msg)
	}
	if loaded.projPath != second {
		t.Fatalf("expected worktrees for %s, got %s", second, loaded.projPath)
	}
	if loaded.project != "api" {
		t.Fatalf("expected workspace name api, got %q", loaded.project)
	}
	if len(loaded.entries) != 1 || loaded.entries[0].Path != second {
		t.Fatalf("expected worktrees from selected path, got %+v", loaded.entries)
	}
}

func prependPath(t *testing.T, dir string) {
	t.Helper()
	path := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+path)
}

func writeExecutable(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil { //nolint:gosec // test helper needs executable scripts
		t.Fatalf("write %s: %v", path, err)
	}
}
