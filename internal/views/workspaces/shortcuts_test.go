package workspaces

import (
	"testing"

	wsdata "github.com/miltonparedes/kitmux/internal/workspaces/data"
)

func TestAOpensAgentPickerFromProjects(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("a"))
	m = updated.(Model)
	if m.mode != modeAgentPicker {
		t.Errorf("expected modeAgentPicker from projects, got %d", m.mode)
	}
}

func TestAOpensAgentPickerFromDetail(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("a"))
	m = updated.(Model)
	if m.mode != modeAgentPicker {
		t.Errorf("expected modeAgentPicker from detail, got %d", m.mode)
	}
}

func TestFOpensZoxideFromEitherColumn(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(keyMsg("f"))
	m = updated.(Model)
	if m.mode != modeProjectSearch {
		t.Errorf("expected modeProjectSearch from projects, got %d", m.mode)
	}

	m = newSeededModel()
	updated, _ = m.Update(keyMsg("l"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("f"))
	m = updated.(Model)
	if m.mode != modeProjectSearch {
		t.Errorf("expected modeProjectSearch from detail, got %d", m.mode)
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
	m.mode = modeProjectSearch
	if !m.IsEditing() {
		t.Error("expected editing in modeProjectSearch")
	}
	m.mode = modeAgentPicker
	if !m.IsEditing() {
		t.Error("expected editing in modeAgentPicker")
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

	if m.projects[0].Added != 7 {
		t.Errorf("expected kitmux added=7, got %d", m.projects[0].Added)
	}
	if m.projects[0].Deleted != 2 {
		t.Errorf("expected kitmux deleted=2, got %d", m.projects[0].Deleted)
	}
	if m.projects[0].Worktrees != 2 {
		t.Errorf("expected 2 worktrees, got %d", m.projects[0].Worktrees)
	}
	if m.projects[0].DirtyCount != 1 {
		t.Errorf("expected 1 dirty, got %d", m.projects[0].DirtyCount)
	}
	// api has no stats entry
	if m.projects[1].Added != 0 || m.projects[1].Worktrees != 0 {
		t.Errorf("expected api summary zeroed, got +%d worktrees=%d", m.projects[1].Added, m.projects[1].Worktrees)
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
	for _, p := range m.projects {
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
