package workbench

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestModelNavigationAndPopupAction(t *testing.T) {
	m := New()

	var cmd tea.Cmd
	for i := 0; i < 2; i++ {
		m, cmd = m.Update(key("j"))
		if cmd != nil {
			t.Fatal("expected navigation without command")
		}
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter to produce a command")
	}
	msg := cmd()
	run, ok := msg.(messages.RunPopupMsg)
	if !ok {
		t.Fatalf("expected RunPopupMsg, got %T", msg)
	}
	if run.Command != "lazygit" {
		t.Fatalf("expected lazygit command, got %q", run.Command)
	}
	if !run.Stay {
		t.Fatal("expected popup to keep workbench alive")
	}
}

func TestLaunchAgentToolOpensDirectoryPicker(t *testing.T) {
	stubDirSources(t)
	m := New()
	m.project = projectStats{Path: "/tmp/current"}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected blink command")
	}
	if updated.mode != modeDirPicker {
		t.Fatalf("expected dir picker, got %v", updated.mode)
	}
	if len(updated.filteredDirs) == 0 || updated.filteredDirs[0].Path != "/tmp/current" {
		t.Fatalf("expected current directory first, got %+v", updated.filteredDirs)
	}
}

func TestDirectoryPickerKeepsInputFixedAndLimitsRows(t *testing.T) {
	m := New()
	m.SetSize(60, 24)
	m.mode = modeDirPicker
	m.dirInput.Focus()
	m.dirs = []dirEntry{
		{Name: "current", Path: "/repo/current"},
		{Name: "one", Path: "/repo/one"},
		{Name: "two", Path: "/repo/two"},
		{Name: "three", Path: "/repo/three"},
		{Name: "four", Path: "/repo/four"},
		{Name: "five", Path: "/repo/five"},
		{Name: "six", Path: "/repo/six"},
	}
	m.filteredDirs = m.dirs

	out := m.View()
	if strings.Count(out, "/repo/") != 5 {
		t.Fatalf("expected five visible directories:\n%s", out)
	}
	if !strings.Contains(out, "+2 more") {
		t.Fatalf("expected hidden count:\n%s", out)
	}

	for range 6 {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		_ = cmd
	}
	if m.dirScroll == 0 {
		t.Fatal("expected directory list to scroll while input stays at top")
	}
}

func TestModelFiltersAgentActivity(t *testing.T) {
	m := New()
	m, _ = m.Update(panesLoadedMsg{panes: []tmux.Pane{
		{SessionName: "repo", WindowIndex: 1, PaneIndex: 0, Command: "zsh"},
		{SessionName: "repo", WindowIndex: 1, PaneIndex: 1, Command: "codex"},
	}})

	if len(m.panes) != 1 {
		t.Fatalf("expected one agent pane, got %d", len(m.panes))
	}
	if m.panes[0].Command != "codex" {
		t.Fatalf("expected codex pane, got %q", m.panes[0].Command)
	}
}

func TestViewRendersBasicWorkbench(t *testing.T) {
	m := New()
	m.SetSize(80, 20)
	m.project = projectStats{Name: "kitmux", Branch: "main", Files: 10, Lines: 200}
	out := m.View()

	for _, want := range []string{"Summary", "Progress", "Branch details", "Artifacts", "Sources", "Tools", "Launch Agent", "Activity"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected view to contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Quick agents") {
		t.Fatalf("expected quick agents section to be removed:\n%s", out)
	}
}

func TestMouseClickToolRunsPaneCommand(t *testing.T) {
	m := New()
	row := m.firstActionRow() + 2*actionRowHeight

	_, cmd := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
		Y:      row,
	})
	if cmd == nil {
		t.Fatal("expected mouse click to produce command")
	}
	msg := cmd()
	run, ok := msg.(messages.RunPopupMsg)
	if !ok {
		t.Fatalf("expected RunPopupMsg, got %T", msg)
	}
	if run.Command != "lazygit" {
		t.Fatalf("expected lazygit, got %q", run.Command)
	}
}

func TestMouseClickActionDescriptionIsClickable(t *testing.T) {
	stubDirSources(t)
	m := New()
	m.project = projectStats{Path: "/tmp/current"}
	row := m.firstActionRow() + 1

	updated, cmd := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
		Y:      row,
	})
	if cmd == nil {
		t.Fatal("expected description row click to produce command")
	}
	if updated.mode != modeDirPicker {
		t.Fatalf("expected dir picker, got %v", updated.mode)
	}
}

func TestViewRendersSummaryDetails(t *testing.T) {
	m := New()
	m.SetSize(90, 28)
	m.project = projectStats{
		Name:         "kitmux",
		Branch:       "agent-workbench",
		Files:        42,
		Lines:        1200,
		Added:        10,
		Deleted:      2,
		Unstaged:     7,
		Untracked:    3,
		ChangedFiles: []string{"README.md", "internal/app/app.go", "internal/views/workbench/render.go"},
	}

	out := m.View()
	for _, want := range []string{"agent-workbench", "7 unstaged", "3 untracked", "README.md", "+1 more", "1,200 lines"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected view to contain %q:\n%s", want, out)
		}
	}
}

func TestLaunchAgentFlowSelectsAgentAfterDirectory(t *testing.T) {
	stubDirSources(t)
	m := New()
	m.project = projectStats{Path: "/tmp/current"}

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected launch tool command")
	}
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected directory selection without command")
	}
	if m.mode != modeAgentPicker {
		t.Fatalf("expected agent picker, got %v", m.mode)
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected launch message")
	}
	msg := cmd()
	launch, ok := msg.(messages.LaunchWorkbenchAgentMsg)
	if !ok {
		t.Fatalf("expected LaunchWorkbenchAgentMsg, got %T", msg)
	}
	if launch.AgentID != "claude" || launch.ModeID != "default" || launch.Dir != "/tmp/current" {
		t.Fatalf("unexpected launch: %+v", launch)
	}
}

func TestDirectoryPickerIncludesZoxideResults(t *testing.T) {
	stubDirSources(t)
	dirs := buildDirEntries("/tmp/current")
	found := false
	for _, dir := range dirs {
		if dir.Path == "/tmp/zoxide-repo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected zoxide result in dirs: %+v", dirs)
	}
}

func stubDirSources(t *testing.T) {
	t.Helper()
	original := execCommand
	execCommand = func(name string, args ...string) (string, error) {
		if name == "zoxide" {
			return "10 /tmp/zoxide-repo\n", nil
		}
		return original(name, args...)
	}
	t.Cleanup(func() {
		execCommand = original
	})
}

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
