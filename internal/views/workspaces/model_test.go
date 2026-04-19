package workspaces

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/tmux"
	wsreg "github.com/miltonparedes/kitmux/internal/workspaces"
	"github.com/miltonparedes/kitmux/internal/worktree"
)

// --- helpers ---

func prependPath(t *testing.T, dir string) {
	t.Helper()
	path := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+path)
}

func writeExecutable(t *testing.T, dir, name, contents string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(contents), 0o755); err != nil { //nolint:gosec // test helper
		t.Fatalf("write %s: %v", p, err)
	}
}

func keyMsg(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func seedModel(projects []workspaceEntry, sessions []tmux.Session, repoRoots map[string]string, wtByPath map[string][]worktree.Worktree, panes []tmux.Pane) Model {
	m := New()
	m.width = 120
	m.height = 30
	m.workspaces = projects
	m.sessions = sessions
	m.repoRoots = repoRoots
	m.wtByPath = wtByPath
	m.panes = panes
	m.clampWorkspaceCursor()
	m.rebuildDetail()
	return m
}

func testWorkspaces() []workspaceEntry {
	return []workspaceEntry{
		{Name: "kitmux", Path: "/home/user/kitmux", Active: true, Activity: 200},
		{Name: "api", Path: "/home/user/api", Active: true, Activity: 100},
		{Name: "dotfiles", Path: "/home/user/dotfiles", Active: false},
	}
}

func testSessions() []tmux.Session {
	return []tmux.Session{
		{Name: "kitmux-main", Path: "/home/user/kitmux", Windows: 3, Attached: true, Activity: 200},
		{Name: "kitmux-feature", Path: "/home/user/kitmux-feature", Windows: 2, Activity: 150},
		{Name: "api-main", Path: "/home/user/api", Windows: 1, Activity: 100},
	}
}

func testRepoRoots() map[string]string {
	return map[string]string{
		"kitmux-main":    "/home/user/kitmux",
		"kitmux-feature": "/home/user/kitmux",
		"api-main":       "/home/user/api",
	}
}

func testWtByPath() map[string][]worktree.Worktree {
	return map[string][]worktree.Worktree{
		"/home/user/kitmux": {
			{Branch: "main", Path: "/home/user/kitmux", IsMain: true},
			{Branch: "feature", Path: "/home/user/kitmux-feature"},
			{Branch: "experiment", Path: "/home/user/kitmux-experiment"},
		},
		"/home/user/api": {
			{Branch: "main", Path: "/home/user/api", IsMain: true},
		},
	}
}

func testPanes() []tmux.Pane {
	return []tmux.Pane{
		{SessionName: "kitmux-main", WindowIndex: 0, PaneIndex: 0, Command: "zsh", Path: "/home/user/kitmux"},
		{SessionName: "kitmux-main", WindowIndex: 1, PaneIndex: 0, Command: "claude", Path: "/home/user/kitmux"},
		{SessionName: "api-main", WindowIndex: 0, PaneIndex: 0, Command: "zsh", Path: "/home/user/api"},
	}
}

func newSeededModel() Model {
	return seedModel(testWorkspaces(), testSessions(), testRepoRoots(), testWtByPath(), testPanes())
}

// --- Column focus tests ---

func TestInitialFocusIsProjects(t *testing.T) {
	m := newSeededModel()
	if m.focus != colWorkspaces {
		t.Errorf("expected initial focus on colWorkspaces, got %d", m.focus)
	}
}

func TestFocusSwitchWithL(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	if m.focus != colDetail {
		t.Errorf("expected focus on colDetail after 'l', got %d", m.focus)
	}
}

func TestFocusSwitchWithH(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("h"))
	m = updated.(Model)
	if m.focus != colWorkspaces {
		t.Errorf("expected focus on colWorkspaces after 'h', got %d", m.focus)
	}
}

func TestFocusSwitchWithArrows(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("right"))
	m = updated.(Model)
	if m.focus != colDetail {
		t.Errorf("expected focus on colDetail after right arrow, got %d", m.focus)
	}
	updated, _ = m.Update(keyMsg("left"))
	m = updated.(Model)
	if m.focus != colWorkspaces {
		t.Errorf("expected focus on colWorkspaces after left arrow, got %d", m.focus)
	}
}

func TestEnterOnProjectFocusesDetail(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.focus != colDetail {
		t.Errorf("expected focus on colDetail after enter on project, got %d", m.focus)
	}
}

func TestHOnProjectsDoesNothing(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("h"))
	m = updated.(Model)
	if m.focus != colWorkspaces {
		t.Errorf("expected focus to remain on colWorkspaces, got %d", m.focus)
	}
}

func TestLOnEmptyDetailDoesNotSwitch(t *testing.T) {
	m := seedModel(nil, nil, nil, nil, nil)
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	if m.focus != colWorkspaces {
		t.Errorf("expected focus to remain on colWorkspaces with no detail, got %d", m.focus)
	}
}

// --- Project navigation tests ---

func TestJKNavigateProjects(t *testing.T) {
	m := newSeededModel()
	if m.wsCursor != 0 {
		t.Fatalf("expected initial cursor at 0, got %d", m.wsCursor)
	}

	updated, _ := m.Update(keyMsg("j"))
	m = updated.(Model)
	if m.wsCursor != 1 {
		t.Errorf("expected cursor at 1 after 'j', got %d", m.wsCursor)
	}

	updated, _ = m.Update(keyMsg("k"))
	m = updated.(Model)
	if m.wsCursor != 0 {
		t.Errorf("expected cursor at 0 after 'k', got %d", m.wsCursor)
	}
}

func TestProjectCursorClamps(t *testing.T) {
	m := newSeededModel()
	// Go past the end
	for i := 0; i < 10; i++ {
		updated, _ := m.Update(keyMsg("j"))
		m = updated.(Model)
	}
	if m.wsCursor != len(m.workspaces)-1 {
		t.Errorf("expected cursor clamped to %d, got %d", len(m.workspaces)-1, m.wsCursor)
	}

	// Go past the beginning
	for i := 0; i < 10; i++ {
		updated, _ := m.Update(keyMsg("k"))
		m = updated.(Model)
	}
	if m.wsCursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", m.wsCursor)
	}
}

func TestGAndGJumpToEnds(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("G"))
	m = updated.(Model)
	if m.wsCursor != len(m.workspaces)-1 {
		t.Errorf("expected cursor at last project, got %d", m.wsCursor)
	}
	updated, _ = m.Update(keyMsg("g"))
	m = updated.(Model)
	if m.wsCursor != 0 {
		t.Errorf("expected cursor at 0 after 'g', got %d", m.wsCursor)
	}
}

func TestProjectCursorChangeUpdatesDetail(t *testing.T) {
	m := newSeededModel()
	initialBranches := len(m.branches)

	updated, _ := m.Update(keyMsg("j"))
	m = updated.(Model)
	if len(m.branches) == initialBranches && m.workspaces[0].Path != m.workspaces[1].Path {
		t.Log("branches may have changed")
	}
	// Verify detail corresponds to the second project (api)
	for _, b := range m.branches {
		if b.IsSession && b.SessionName != "" {
			root := m.repoRoots[b.SessionName]
			if root != m.workspaces[m.wsCursor].Path {
				t.Errorf("branch %q belongs to %q, not selected project %q", b.Name, root, m.workspaces[m.wsCursor].Path)
			}
		}
	}
}

// --- Detail navigation tests ---

func TestDetailJKNavigation(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	if m.detCursor != 0 {
		t.Fatalf("expected detail cursor at 0, got %d", m.detCursor)
	}
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(Model)
	if m.detCursor != 1 {
		t.Errorf("expected detail cursor at 1 after 'j', got %d", m.detCursor)
	}
	updated, _ = m.Update(keyMsg("k"))
	m = updated.(Model)
	if m.detCursor != 0 {
		t.Errorf("expected detail cursor at 0 after 'k', got %d", m.detCursor)
	}
}

func TestDetailCursorClamps(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	for i := 0; i < 50; i++ {
		updated, _ = m.Update(keyMsg("j"))
		m = updated.(Model)
	}
	if m.detCursor != m.detailItems-1 {
		t.Errorf("expected detail cursor clamped to %d, got %d", m.detailItems-1, m.detCursor)
	}
}

func TestDetailGAndG(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("G"))
	m = updated.(Model)
	if m.detCursor != m.detailItems-1 {
		t.Errorf("expected detail cursor at last item, got %d", m.detCursor)
	}
	updated, _ = m.Update(keyMsg("g"))
	m = updated.(Model)
	if m.detCursor != 0 {
		t.Errorf("expected detail cursor at 0, got %d", m.detCursor)
	}
}

// --- Data loading and rebuild tests ---

func TestDataLoadedMsgPopulatesModel(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 30

	msg := dataLoadedMsg{
		workspaces: testWorkspaces(),
		sessions:   testSessions(),
		repoRoots:  testRepoRoots(),
		wtByPath:   testWtByPath(),
		panes:      testPanes(),
	}

	updated, _ := m.Update(msg)
	m = updated.(Model)

	if len(m.workspaces) != 3 {
		t.Errorf("expected 3 workspaces, got %d", len(m.workspaces))
	}
	if len(m.sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(m.sessions))
	}
	if len(m.branches) == 0 {
		t.Error("expected branches to be populated for first project")
	}
}

func TestRebuildDetail_FirstProjectHasBranchesAndAgents(t *testing.T) {
	m := newSeededModel()
	// First project is kitmux with 2 sessions and 1 inactive worktree
	if len(m.branches) != 3 {
		t.Errorf("expected 3 branches (2 sessions + 1 inactive wt), got %d", len(m.branches))
	}
	// Should have 1 detected agent (claude) + 1 launcher
	if len(m.agentEntries) != 2 {
		t.Errorf("expected 2 agent entries (1 detected + 1 launcher), got %d", len(m.agentEntries))
	}
	if !m.agentEntries[len(m.agentEntries)-1].IsLauncher {
		t.Error("expected last agent entry to be launcher")
	}
}

func TestRebuildDetail_InactiveProjectHasNoSessions(t *testing.T) {
	m := newSeededModel()
	// Navigate to dotfiles (3rd project, inactive)
	updated, _ := m.Update(keyMsg("j"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(Model)

	if m.wsCursor != 2 {
		t.Fatalf("expected cursor at 2, got %d", m.wsCursor)
	}

	sessionCount := 0
	for _, b := range m.branches {
		if b.IsSession {
			sessionCount++
		}
	}
	if sessionCount != 0 {
		t.Errorf("expected 0 sessions for inactive project, got %d", sessionCount)
	}
}

func TestRebuildDetail_MainBranchSortedFirst(t *testing.T) {
	m := newSeededModel()
	if len(m.branches) == 0 {
		t.Fatal("no branches")
	}
	// First branch should be main (active session)
	if !isMainBranch(m.branches[0].Name) {
		t.Errorf("expected first branch to be main, got %q", m.branches[0].Name)
	}
}

func TestRebuildDetail_ActiveSessionNotDuplicatedWithWorktree(t *testing.T) {
	m := newSeededModel()
	seen := make(map[string]int)
	for _, b := range m.branches {
		seen[b.Name]++
	}
	for name, count := range seen {
		if count > 1 {
			t.Errorf("branch %q appears %d times, expected unique", name, count)
		}
	}
}

// --- Agent detection tests ---

func TestDetectAgents_FindsClaudePane(t *testing.T) {
	detected := detectAgents(testPanes(), "/home/user/kitmux", testRepoRoots(), testSessions())
	agentCount := 0
	for _, ae := range detected {
		if !ae.IsLauncher {
			agentCount++
		}
	}
	if agentCount != 1 {
		t.Errorf("expected 1 detected agent (claude), got %d", agentCount)
	}
	if detected[0].Name != "Claude Code" {
		t.Errorf("expected agent name 'Claude Code', got %q", detected[0].Name)
	}
}

func TestDetectAgents_IgnoresNonAgentPanes(t *testing.T) {
	panes := []tmux.Pane{
		{SessionName: "kitmux-main", WindowIndex: 0, PaneIndex: 0, Command: "zsh", Path: "/home/user/kitmux"},
		{SessionName: "kitmux-main", WindowIndex: 0, PaneIndex: 1, Command: "vim", Path: "/home/user/kitmux"},
	}
	detected := detectAgents(panes, "/home/user/kitmux", testRepoRoots(), testSessions())
	agentCount := 0
	for _, ae := range detected {
		if !ae.IsLauncher {
			agentCount++
		}
	}
	if agentCount != 0 {
		t.Errorf("expected 0 detected agents for non-agent panes, got %d", agentCount)
	}
}

func TestDetectAgents_OnlyMatchesProjectPath(t *testing.T) {
	panes := []tmux.Pane{
		{SessionName: "api-main", WindowIndex: 0, PaneIndex: 0, Command: "claude", Path: "/home/user/api"},
	}
	detected := detectAgents(panes, "/home/user/kitmux", testRepoRoots(), testSessions())
	agentCount := 0
	for _, ae := range detected {
		if !ae.IsLauncher {
			agentCount++
		}
	}
	if agentCount != 0 {
		t.Errorf("expected 0 agents for different project path, got %d", agentCount)
	}
}

func TestDetectAgents_AlwaysHasLauncher(t *testing.T) {
	detected := detectAgents(nil, "/nonexistent", nil, nil)
	if len(detected) != 1 || !detected[0].IsLauncher {
		t.Error("expected launcher entry even with no panes")
	}
}

// --- Mode transition tests ---

func TestEscOnDetailGoesBackToProjects(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	if m.focus != colDetail {
		t.Fatal("should be in detail")
	}
	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)
	if m.focus != colWorkspaces {
		t.Errorf("expected focus back to projects after esc, got %d", m.focus)
	}
}

func TestQOnDetailGoesBackToProjects(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, cmd := m.Update(keyMsg("q"))
	m = updated.(Model)
	if m.focus != colWorkspaces {
		t.Errorf("expected focus back to projects after q, got %d", m.focus)
	}
	if cmd != nil {
		t.Error("expected no quit command when going back from detail")
	}
}

func TestQOnProjectsQuits(t *testing.T) {
	m := newSeededModel()
	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Error("expected quit command on q from projects column")
	}
}

func TestDEntersConfirmMode(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("d"))
	m = updated.(Model)
	if m.mode != modeConfirm {
		t.Errorf("expected modeConfirm, got %d", m.mode)
	}
	if m.confirmName != "kitmux" {
		t.Errorf("expected confirm name 'kitmux', got %q", m.confirmName)
	}
}

func TestConfirmNoCancels(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("d"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("n"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after cancel, got %d", m.mode)
	}
}

func TestConfirmYesRemoves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := wsreg.SaveRegistry([]wsreg.Workspace{
		{Name: "alpha", Path: "/tmp/alpha", AddedAt: 1, LastSeenAt: 1},
		{Name: "beta", Path: "/tmp/beta", AddedAt: 2, LastSeenAt: 2},
	}); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	m := seedModel(
		[]workspaceEntry{
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		},
		nil, nil, nil, nil,
	)
	m.wsCursor = 0

	updated, _ := m.Update(keyMsg("d"))
	m = updated.(Model)
	updated, cmd := m.Update(keyMsg("y"))
	m = updated.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after confirm, got %d", m.mode)
	}
	if cmd == nil {
		t.Error("expected reload command after confirm delete")
	}

	loaded := wsreg.LoadRegistry()
	if len(loaded) != 1 || loaded[0].Path != "/tmp/beta" {
		t.Errorf("expected only beta to remain, got %+v", loaded)
	}
}

func TestNEntersProjectSearchMode(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("n"))
	m = updated.(Model)
	if m.mode != modeWorkspaceSearch {
		t.Errorf("expected modeWorkspaceSearch, got %d", m.mode)
	}
}

func TestNInDetailDoesNothing(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("n"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal, got %d", m.mode)
	}
}

func TestCEntersNewBranchMode(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("c"))
	m = updated.(Model)
	if m.mode != modeNewBranch {
		t.Errorf("expected modeNewBranch, got %d", m.mode)
	}
}

func TestCInProjectsDoesNothing(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("c"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal, got %d", m.mode)
	}
}

func TestNewBranchEscCancels(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("c"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after esc, got %d", m.mode)
	}
}

func TestProjectSearchEscCancels(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("n"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after esc, got %d", m.mode)
	}
}

// --- Agent picker tests ---

func TestAgentPickerEntersAndExits(t *testing.T) {
	m := newSeededModel()
	// Navigate to detail, then to the launcher item (last in detail)
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("G"))
	m = updated.(Model)

	// Last item should be the launcher
	lastIdx := m.detailItems - 1
	agentIdx := lastIdx - len(m.branches)
	if agentIdx < 0 || agentIdx >= len(m.agentEntries) {
		t.Fatalf("agent index out of range: %d", agentIdx)
	}
	if !m.agentEntries[agentIdx].IsLauncher {
		t.Fatal("expected last detail item to be agent launcher")
	}

	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.mode != modeAgentPicker {
		t.Errorf("expected modeAgentPicker, got %d", m.mode)
	}

	// Navigate the picker
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(Model)
	if m.agentPicker.cursor != 1 {
		t.Errorf("expected agent picker cursor at 1, got %d", m.agentPicker.cursor)
	}

	// Cycle mode with tab
	updated, _ = m.Update(keyMsg("tab"))
	m = updated.(Model)

	// Esc exits picker
	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after esc, got %d", m.mode)
	}
}

func TestAgentPickerCursorClamps(t *testing.T) {
	m := newSeededModel()
	m.mode = modeAgentPicker
	m.agentPicker.cursor = 0

	// Go up past start
	updated, _ := m.Update(keyMsg("k"))
	m = updated.(Model)
	if m.agentPicker.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.agentPicker.cursor)
	}

	// Go down past end
	for i := 0; i < 100; i++ {
		updated, _ = m.Update(keyMsg("j"))
		m = updated.(Model)
	}
	if m.agentPicker.cursor != len(m.agentPicker.agents)-1 {
		t.Errorf("expected cursor clamped at %d, got %d", len(m.agentPicker.agents)-1, m.agentPicker.cursor)
	}
}

func TestAgentPickerTabCyclesMode(t *testing.T) {
	m := newSeededModel()
	m.mode = modeAgentPicker
	m.agentPicker.cursor = 0

	a := m.agentPicker.agents[0]
	initialMode := m.agentPicker.modeIndex[0]

	updated, _ := m.Update(keyMsg("tab"))
	m = updated.(Model)

	expected := (initialMode + 1) % len(a.Modes)
	if m.agentPicker.modeIndex[0] != expected {
		t.Errorf("expected mode index %d after tab, got %d", expected, m.agentPicker.modeIndex[0])
	}
}

// --- Stats tests ---

func TestStatsAppliedToBranches(t *testing.T) {
	m := newSeededModel()
	m.stats = map[string]sessionStats{
		"kitmux-main": {Added: 12, Deleted: 3},
	}
	m.rebuildDetail()

	for _, b := range m.branches {
		if b.SessionName == "kitmux-main" {
			if b.DiffAdded != 12 || b.DiffDel != 3 {
				t.Errorf("expected stats +12 -3, got +%d -%d", b.DiffAdded, b.DiffDel)
			}
			return
		}
	}
	t.Error("kitmux-main branch not found")
}

func TestStatsLoadedMsgUpdatesModel(t *testing.T) {
	m := newSeededModel()
	msg := statsLoadedMsg{stats: map[string]sessionStats{
		"api-main": {Added: 5, Deleted: 2},
	}}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.stats["api-main"].Added != 5 {
		t.Errorf("expected stats added=5, got %d", m.stats["api-main"].Added)
	}
}

// --- Scrolling tests ---

func TestProjectScrollingWithSmallHeight(t *testing.T) {
	m := newSeededModel()
	m.height = 8 // very small

	// Navigate to last project
	for i := 0; i < 5; i++ {
		updated, _ := m.Update(keyMsg("j"))
		m = updated.(Model)
	}

	// Scroll should have adjusted
	if m.wsCursor > len(m.workspaces)-1 {
		t.Error("cursor out of bounds")
	}
}

func TestDetailScrollingWithSmallHeight(t *testing.T) {
	m := newSeededModel()
	m.height = 8
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)

	for i := 0; i < 20; i++ {
		updated, _ = m.Update(keyMsg("j"))
		m = updated.(Model)
	}

	if m.detCursor > m.detailItems-1 {
		t.Error("detail cursor out of bounds")
	}
}

// --- Render tests ---

func TestViewColumns_BasicRender(t *testing.T) {
	m := newSeededModel()
	output := m.View()

	if !strings.Contains(output, "kitmux") {
		t.Error("expected 'kitmux' in rendered output")
	}
	if !strings.Contains(output, "api") {
		t.Error("expected 'api' in rendered output")
	}
	if !strings.Contains(output, "Branches") {
		t.Error("expected 'Branches' section header in rendered output")
	}
	if !strings.Contains(output, "Agents") {
		t.Error("expected 'Agents' section header in rendered output")
	}
}

func TestViewColumns_ShowsAgentLauncher(t *testing.T) {
	m := newSeededModel()
	output := m.View()

	if !strings.Contains(output, "launch agent") {
		t.Error("expected '+ launch agent...' in rendered output")
	}
}

func TestViewColumns_ShowsDetectedAgent(t *testing.T) {
	m := newSeededModel()
	output := m.View()

	if !strings.Contains(output, "Claude Code") {
		t.Error("expected 'Claude Code' detected agent in rendered output")
	}
}

func TestViewColumns_ActiveProjectShowsBadge(t *testing.T) {
	m := newSeededModel()
	output := m.View()

	if !strings.Contains(output, "●") {
		t.Error("expected active badge in rendered output")
	}
}

func TestViewColumns_FooterChangesWithFocus(t *testing.T) {
	m := newSeededModel()
	output1 := m.View()
	if !strings.Contains(output1, "n add") {
		t.Error("expected project-focused footer hints")
	}

	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	output2 := m.View()
	if !strings.Contains(output2, "h back") {
		t.Error("expected detail-focused footer hints")
	}
}

func TestViewProjectSearch_Renders(t *testing.T) {
	m := newSeededModel()
	m.mode = modeWorkspaceSearch
	m.zoxide.all = []zoxideEntry{
		{Score: 100, Path: "/home/user/test", Short: "~/test"},
	}
	m.zoxide.filtered = m.zoxide.all

	output := m.View()
	if !strings.Contains(output, "~/test") {
		t.Error("expected zoxide entry in search view")
	}
}

func TestViewAgentPicker_Renders(t *testing.T) {
	m := newSeededModel()
	m.mode = modeAgentPicker

	output := m.View()
	if !strings.Contains(output, "Launch Agent") {
		t.Error("expected 'Launch Agent' header")
	}
	for _, a := range agents.DefaultAgents() {
		if !strings.Contains(output, a.Name) {
			t.Errorf("expected agent %q in picker", a.Name)
		}
	}
}

func TestViewConfirmMode_ShowsPrompt(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("d"))
	m = updated.(Model)
	output := m.View()
	if !strings.Contains(output, "remove 'kitmux'?") {
		t.Error("expected confirm prompt in footer")
	}
}

func TestViewNewBranch_ShowsInput(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("c"))
	m = updated.(Model)
	output := m.View()
	if !strings.Contains(output, "New branch") {
		t.Error("expected 'New branch' in output")
	}
}

func TestViewColumns_InactiveWorktreeDimmed(t *testing.T) {
	m := newSeededModel()
	// The "experiment" worktree is inactive for kitmux
	output := m.View()
	if !strings.Contains(output, "experiment") {
		t.Error("expected inactive worktree 'experiment' in output")
	}
}

func TestViewColumns_HasColumnHeaders(t *testing.T) {
	m := newSeededModel()
	output := m.View()
	if !strings.Contains(output, "Workspaces") {
		t.Error("expected 'Workspaces' header in output")
	}
	if !strings.Contains(output, "Branches") {
		t.Error("expected 'Branches' section header in output")
	}
}

// --- Diff stats rendering ---

func TestBranchDiffStats(t *testing.T) {
	tests := []struct {
		name     string
		br       branchEntry
		wantAdd  bool
		wantDel  bool
		wantNone bool
	}{
		{"both", branchEntry{DiffAdded: 10, DiffDel: 5}, true, true, false},
		{"add only", branchEntry{DiffAdded: 3}, true, false, false},
		{"del only", branchEntry{DiffDel: 7}, false, true, false},
		{"none", branchEntry{}, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := branchDiffStats(tt.br)
			if tt.wantNone && result != "" {
				t.Errorf("expected empty, got %q", result)
			}
			if tt.wantAdd && !strings.Contains(result, fmt.Sprintf("+%d", tt.br.DiffAdded)) {
				t.Errorf("expected +%d in result", tt.br.DiffAdded)
			}
			if tt.wantDel && !strings.Contains(result, fmt.Sprintf("-%d", tt.br.DiffDel)) {
				t.Errorf("expected -%d in result", tt.br.DiffDel)
			}
		})
	}
}

// --- Window size tests ---

func TestWindowSizeMsg(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m = updated.(Model)
	if m.width != 200 || m.height != 50 {
		t.Errorf("expected 200x50, got %dx%d", m.width, m.height)
	}
}

func TestLeftWidthMinimum(t *testing.T) {
	m := newSeededModel()
	m.width = 100
	if got := m.leftWidth(); got < 18 {
		t.Errorf("expected minimum left width of 18 at width=100, got %d", got)
	}
}

func TestRightWidthMinimum(t *testing.T) {
	m := newSeededModel()
	m.width = 100
	if got := m.rightWidth(); got < 10 {
		t.Errorf("expected right width to leave at least 10 cols at width=100, got %d", got)
	}
}

// --- Mouse tests ---

func TestMouseScrollInProjects(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.wsCursor != 1 {
		t.Errorf("expected cursor at 1 after wheel down, got %d", m.wsCursor)
	}
	updated, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m = updated.(Model)
	if m.wsCursor != 0 {
		t.Errorf("expected cursor at 0 after wheel up, got %d", m.wsCursor)
	}
}

func TestMouseScrollInDetail(t *testing.T) {
	m := newSeededModel()
	m.focus = colDetail
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.detCursor != 1 {
		t.Errorf("expected detail cursor at 1, got %d", m.detCursor)
	}
}

// --- Mouse left click ---

func clickAt(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease}
}

func TestMouseClickProject_SelectsAndMovesCursor(t *testing.T) {
	m := newSeededModel()
	m.width = 120
	m.height = 30
	// Header + sep occupy rows 0,1; rows are 2 lines each (item + sep).
	// Click on the second project (row index 1 -> y = 2 + 1*2 = 4).
	updated, _ := m.Update(clickAt(2, 4))
	m = updated.(Model)
	if m.wsCursor != 1 {
		t.Errorf("expected cursor on project 1 after click, got %d", m.wsCursor)
	}
	if m.focus != colWorkspaces {
		t.Errorf("expected focus on projects column, got %d", m.focus)
	}
}

func TestMouseClickProject_TwiceMovesFocusToDetail(t *testing.T) {
	m := newSeededModel()
	m.width = 120
	m.height = 30
	// First click selects project 0 (already selected) and dives into detail.
	updated, _ := m.Update(clickAt(2, 2))
	m = updated.(Model)
	if m.focus != colDetail {
		t.Errorf("expected focus to move to detail on second click of selected project, got %d", m.focus)
	}
}

func TestMouseClickDetail_SelectsBranch(t *testing.T) {
	m := newSeededModel()
	m.width = 120
	m.height = 30
	leftW := m.leftWidth()
	rightStart := leftW + 3
	// y=2 is the "Branches" header; y=4 is the first branch row.
	updated, _ := m.Update(clickAt(rightStart+2, 4))
	m = updated.(Model)
	if m.focus != colDetail {
		t.Errorf("expected focus to detail after right column click, got %d", m.focus)
	}
	if m.detCursor != 0 {
		t.Errorf("expected detCursor=0 (first branch), got %d", m.detCursor)
	}
}

// --- isMainBranch / trimPrefix ---

func TestIsMainBranch(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"main", true},
		{"master", true},
		{"kitmux-main", true},
		{"kitmux-master", true},
		{"feature", false},
		{"main-feature", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMainBranch(tt.name); got != tt.want {
				t.Errorf("isMainBranch(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestTrimPrefix(t *testing.T) {
	tests := []struct {
		sess, proj, want string
	}{
		{"kitmux-main", "kitmux", "main"},
		{"kitmux-feature-x", "kitmux", "feature-x"},
		{"api-v2", "api", "v2"},
		{"unrelated", "kitmux", "unrelated"},
	}
	for _, tt := range tests {
		t.Run(tt.sess, func(t *testing.T) {
			if got := trimPrefix(tt.sess, tt.proj); got != tt.want {
				t.Errorf("trimPrefix(%q, %q) = %q, want %q", tt.sess, tt.proj, got, tt.want)
			}
		})
	}
}

// --- uniqueSessName ---

func TestUniqueSessName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	fakeBin := t.TempDir()
	writeExecutable(t, fakeBin, "tmux", `#!/bin/sh
if [ "$1" = "has-session" ]; then
	exit 1
fi
exit 0
`)
	prependPath(t, fakeBin)

	name := uniqueSessName("test-session")
	if name != "test-session" {
		t.Errorf("expected 'test-session', got %q", name)
	}
}

// --- Filter mode ---

func TestFilterModeEntersAndExits(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("/"))
	m = updated.(Model)
	if m.mode != modeFiltering {
		t.Errorf("expected modeFiltering, got %d", m.mode)
	}

	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after esc, got %d", m.mode)
	}
}

func TestFilterModeEnterAccepts(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("/"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after enter, got %d", m.mode)
	}
}

// --- Refresh ---

func TestRRefreshReloads(t *testing.T) {
	m := newSeededModel()
	_, cmd := m.Update(keyMsg("r"))
	if cmd == nil {
		t.Error("expected reload command on 'r'")
	}
}

// --- Edge cases ---

func TestEmptyModelNoProjects(t *testing.T) {
	m := seedModel(nil, nil, nil, nil, nil)
	output := m.View()
	if !strings.Contains(output, "No workspaces") {
		t.Error("expected 'No workspaces' message")
	}
}

func TestEmptyModelNavigation(t *testing.T) {
	m := seedModel(nil, nil, nil, nil, nil)
	// Should not panic
	updated, _ := m.Update(keyMsg("j"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("k"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("h"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("G"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("g"))
	_ = updated.(Model)
}

func TestSingleProjectModel(t *testing.T) {
	m := seedModel(
		[]workspaceEntry{{Name: "solo", Path: "/tmp/solo"}},
		nil, nil, nil, nil,
	)
	output := m.View()
	if !strings.Contains(output, "solo") {
		t.Error("expected 'solo' in output")
	}
}

// --- contentHeight ---

func TestContentHeight(t *testing.T) {
	m := newSeededModel()
	m.height = 30
	if h := m.contentHeight(); h != 26 {
		t.Errorf("expected contentHeight 26, got %d", h)
	}
	m.height = 4
	if h := m.contentHeight(); h != 1 {
		t.Errorf("expected minimum contentHeight 1, got %d", h)
	}
}

// --- Zoxide picker helpers ---

func TestZoxideFilter(t *testing.T) {
	z := &zoxidePicker{
		all: []zoxideEntry{
			{Score: 100, Path: "/home/user/kitmux", Short: "~/kitmux"},
			{Score: 80, Path: "/home/user/api", Short: "~/api"},
			{Score: 60, Path: "/home/user/dotfiles", Short: "~/dotfiles"},
		},
	}
	z.input = New().zoxide.input
	z.input.SetValue("kit")
	z.filter()

	if len(z.filtered) != 1 {
		t.Errorf("expected 1 match for 'kit', got %d", len(z.filtered))
	}
	if z.filtered[0].Short != "~/kitmux" {
		t.Errorf("expected ~/kitmux, got %q", z.filtered[0].Short)
	}
}

func TestZoxideFilterEmpty(t *testing.T) {
	z := &zoxidePicker{
		all: []zoxideEntry{
			{Score: 100, Path: "/home/user/kitmux", Short: "~/kitmux"},
		},
	}
	z.input = New().zoxide.input
	z.input.SetValue("")
	z.filter()

	if len(z.filtered) != 1 {
		t.Errorf("expected all results for empty query, got %d", len(z.filtered))
	}
}

func TestZoxideSelected(t *testing.T) {
	z := &zoxidePicker{
		filtered: []zoxideEntry{
			{Score: 100, Path: "/home/user/kitmux", Short: "~/kitmux"},
		},
		cursor: 0,
	}
	sel := z.selected()
	if sel == nil || sel.Path != "/home/user/kitmux" {
		t.Error("expected selected zoxide entry")
	}

	z.filtered = nil
	if z.selected() != nil {
		t.Error("expected nil when no entries")
	}
}

// --- selectedWorkspace ---

func TestSelectedProject(t *testing.T) {
	m := newSeededModel()
	p := m.selectedWorkspace()
	if p == nil || p.Name != "kitmux" {
		t.Error("expected selected project to be kitmux")
	}

	m.workspaces = nil
	if m.selectedWorkspace() != nil {
		t.Error("expected nil when no projects")
	}
}

// --- Integration: full navigation flow ---

func TestFullNavigationFlow(t *testing.T) {
	m := newSeededModel()

	// Start at projects column, first project selected
	if m.focus != colWorkspaces || m.wsCursor != 0 {
		t.Fatal("unexpected initial state")
	}

	// Move to second project
	updated, _ := m.Update(keyMsg("j"))
	m = updated.(Model)
	if m.wsCursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", m.wsCursor)
	}
	// Second project is "api" — verify detail shows api branches
	for _, b := range m.branches {
		if b.IsSession && b.SessionName == "kitmux-main" {
			t.Error("detail should not show kitmux sessions when api is selected")
		}
	}

	// Enter detail column
	updated, _ = m.Update(keyMsg("l"))
	m = updated.(Model)
	if m.focus != colDetail {
		t.Fatal("expected detail focus")
	}

	// Navigate detail
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(Model)

	// Go back to projects
	updated, _ = m.Update(keyMsg("h"))
	m = updated.(Model)
	if m.focus != colWorkspaces {
		t.Fatal("expected projects focus after h")
	}

	// Move to last project and check
	updated, _ = m.Update(keyMsg("G"))
	m = updated.(Model)
	if m.wsCursor != 2 {
		t.Fatalf("expected cursor at 2, got %d", m.wsCursor)
	}
	if m.workspaces[m.wsCursor].Name != "dotfiles" {
		t.Error("expected dotfiles at cursor")
	}
}
