package workspaces

import (
	"strings"
	"testing"
)

func TestFilteredIndicesFuzzy(t *testing.T) {
	wss := []workspaceEntry{
		{Name: "kitmux", Path: "/a"},
		{Name: "api", Path: "/b"},
		{Name: "dotfiles", Path: "/c"},
	}
	got := filteredWorkspaceIndices(wss, "api")
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("expected [1], got %v", got)
	}

	got = filteredWorkspaceIndices(wss, "")
	if len(got) != 3 {
		t.Fatalf("expected all indices, got %v", got)
	}

	got = filteredWorkspaceIndices(wss, "zzzzz")
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", got)
	}
}

func TestRenderFilteredLeftOnlyShowsMatches(t *testing.T) {
	m := newSeededModel()
	m.width = 80
	m.height = 20
	m.mode = modeFiltering
	m.filter.SetValue("api")

	out := m.renderLeftColumn(m.leftWidth(), m.contentHeight())

	if strings.Contains(out, "kitmux") {
		t.Errorf("filtered view should hide non-matching 'kitmux', got:\n%s", out)
	}
	if strings.Contains(out, "dotfiles") {
		t.Errorf("filtered view should hide non-matching 'dotfiles', got:\n%s", out)
	}
	if !strings.Contains(out, "api") {
		t.Errorf("filtered view should include matching 'api', got:\n%s", out)
	}
}

func TestRenderFilteredLeftShowsNoMatchHint(t *testing.T) {
	m := newSeededModel()
	m.width = 80
	m.height = 20
	m.mode = modeFiltering
	m.filter.SetValue("zzzzz")

	out := m.renderLeftColumn(m.leftWidth(), m.contentHeight())
	if !strings.Contains(out, "no matches") {
		t.Errorf("expected no-match hint, got:\n%s", out)
	}
}

func TestFilterEnterSnapsCursorToFirstMatch(t *testing.T) {
	m := newSeededModel()
	m.wsCursor = 0
	updated, _ := m.Update(keyMsg("/"))
	m = updated.(Model)

	// Type "dot" to match "dotfiles"
	for _, r := range "dot" {
		updated, _ = m.Update(keyMsg(string(r)))
		m = updated.(Model)
	}

	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)

	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after enter, got %d", m.mode)
	}
	if m.workspaces[m.wsCursor].Name != "dotfiles" {
		t.Errorf("expected cursor on dotfiles, got %q", m.workspaces[m.wsCursor].Name)
	}
}
