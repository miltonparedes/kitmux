package workspaces

import (
	"testing"

	wsdata "github.com/miltonparedes/kitmux/internal/workspaces/data"
)

func TestAFromProjectsOpensAttachChoice(t *testing.T) {
	// From the workspaces column we don't yet have a branch target, so `a`
	// opens the "existing branch / new worktree" chooser rather than jumping
	// straight into the agent picker.
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("a"))
	m = updated.(Model)
	if m.mode != modeAgentAttachChoice {
		t.Errorf("expected modeAgentAttachChoice from workspaces column, got %d", m.mode)
	}
	if m.agentPickerTarget != agentTargetWindow {
		t.Errorf("expected target window, got %d", m.agentPickerTarget)
	}
}

func TestAFromDetailOpensAgentPickerForBranch(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("a"))
	m = updated.(Model)
	if m.mode != modeAgentPicker {
		t.Errorf("expected modeAgentPicker from detail, got %d", m.mode)
	}
	if m.agentPickerIntent != agentIntentAttachBranch {
		t.Errorf("expected intent attachBranch, got %d", m.agentPickerIntent)
	}
	if m.attachBranch.Name == "" {
		t.Error("expected attachBranch populated from detail cursor")
	}
}

func TestShiftAFromDetailUsesSplitTarget(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("A"))
	m = updated.(Model)
	if m.mode != modeAgentPicker {
		t.Errorf("expected modeAgentPicker, got %d", m.mode)
	}
	if m.agentPickerTarget != agentTargetSplit {
		t.Errorf("expected target split, got %d", m.agentPickerTarget)
	}
}

func TestAttachChoiceExistingBranchOpensPicker(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("a"))
	m = updated.(Model)
	// cursor 0 = "In existing branch..."
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.mode != modeAttachBranchPicker {
		t.Errorf("expected modeAttachBranchPicker, got %d", m.mode)
	}
}

func TestAttachChoiceNewWorktreeOpensInput(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("a"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("j")) // move to "In new worktree..."
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.mode != modeNewBranch {
		t.Errorf("expected modeNewBranch, got %d", m.mode)
	}
}

func TestNewBranchTabOpensAgentPicker(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("c"))
	m = updated.(Model)
	// Need a branch name before Tab is meaningful.
	m.newBranch.SetValue("feat/x")
	updated, _ = m.Update(keyMsg("tab"))
	m = updated.(Model)
	if m.mode != modeNewBranchAgent {
		t.Errorf("expected modeNewBranchAgent after Tab, got %d", m.mode)
	}
	if m.agentPickerIntent != agentIntentNewWorktreeAgent {
		t.Errorf("expected intent newWorktreeAgent, got %d", m.agentPickerIntent)
	}
}

func TestNewBranchTabIgnoredWhenEmpty(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("c"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("tab"))
	m = updated.(Model)
	if m.mode != modeNewBranch {
		t.Errorf("expected to stay in modeNewBranch with empty input, got %d", m.mode)
	}
}

func TestFOpensZoxideFromEitherColumn(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("f"))
	m = updated.(Model)
	if m.mode != modeWorkspaceSearch {
		t.Errorf("expected modeWorkspaceSearch from workspaces column, got %d", m.mode)
	}

	m = newSeededModel()
	updated, _ = m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("f"))
	m = updated.(Model)
	if m.mode != modeWorkspaceSearch {
		t.Errorf("expected modeWorkspaceSearch from detail, got %d", m.mode)
	}
}

func TestIsEditingReportsInputModes(t *testing.T) {
	m := newSeededModel()
	if m.IsEditing() {
		t.Fatal("expected not editing in modeNormal")
	}
	m.mode = modeFiltering
	if !m.IsEditing() {
		t.Error("expected editing in modeFiltering")
	}
	m.mode = modeWorkspaceSearch
	if !m.IsEditing() {
		t.Error("expected editing in modeWorkspaceSearch")
	}
	m.mode = modeAgentPicker
	if !m.IsEditing() {
		t.Error("expected editing in modeAgentPicker")
	}
	m.mode = modeNewBranchAgent
	if !m.IsEditing() {
		t.Error("expected editing in modeNewBranchAgent")
	}
	m.mode = modeAgentAttachChoice
	if !m.IsEditing() {
		t.Error("expected editing in modeAgentAttachChoice")
	}
	m.mode = modeAttachBranchPicker
	if !m.IsEditing() {
		t.Error("expected editing in modeAttachBranchPicker")
	}
}

func TestApplyWorkspaceSummary(t *testing.T) {
	m := newSeededModel()
	m.wsStats = map[string]wsdata.WorkspaceStats{
		"/home/user/kitmux": {
			Path: "/home/user/kitmux",
			Worktrees: []wsdata.WorktreeStat{
				{Branch: "main", WorktreePath: "/home/user/kitmux", Added: 0, Deleted: 0},
				{Branch: "feature", WorktreePath: "/home/user/kitmux-feature", Added: 7, Deleted: 2, Modified: true},
			},
		},
	}
	m.applyWorkspaceSummary()

	if m.workspaces[0].Added != 7 {
		t.Errorf("expected kitmux added=7, got %d", m.workspaces[0].Added)
	}
	if m.workspaces[0].Deleted != 2 {
		t.Errorf("expected kitmux deleted=2, got %d", m.workspaces[0].Deleted)
	}
	if m.workspaces[0].Worktrees != 2 {
		t.Errorf("expected 2 worktrees, got %d", m.workspaces[0].Worktrees)
	}
	if m.workspaces[0].DirtyCount != 1 {
		t.Errorf("expected 1 dirty, got %d", m.workspaces[0].DirtyCount)
	}
	// api has no stats entry
	if m.workspaces[1].Added != 0 || m.workspaces[1].Worktrees != 0 {
		t.Errorf("expected api summary zeroed, got +%d worktrees=%d", m.workspaces[1].Added, m.workspaces[1].Worktrees)
	}
}

func TestStatsLoadedMsgMergesWorkspaceStats(t *testing.T) {
	m := newSeededModel()
	stats := map[string]wsdata.WorkspaceStats{
		"/home/user/api": {
			Path: "/home/user/api",
			Worktrees: []wsdata.WorktreeStat{
				{Branch: "main", WorktreePath: "/home/user/api", Added: 3, Deleted: 1, Modified: true},
			},
		},
	}
	updated, _ := m.Update(statsLoadedMsg{wsStats: stats})
	m = updated.(Model)
	if m.wsStats["/home/user/api"].Worktrees[0].Added != 3 {
		t.Errorf("expected api added=3, got %+v", m.wsStats["/home/user/api"])
	}
	// Summary reflects new stats
	found := false
	for _, p := range m.workspaces {
		if p.Path == "/home/user/api" {
			if p.Added != 3 || p.Deleted != 1 {
				t.Errorf("expected summary +3/-1, got +%d/-%d", p.Added, p.Deleted)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("api project not in list")
	}
}
