package threads

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agentrename"
	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/agentthread"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestBuildRowsKeepsHeadlessDetailedAndSkipsDuplicatePane(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "droid-app", Path: "/repo/app", Activity: 10, Thread: true, AgentID: "droid", AgentState: "idle", ThreadTitle: "custom thread title", AgentSessionID: "11111111-1111-4111-8111-111111111111"},
		{Name: "work", Path: "/repo/app"},
	}
	panes := []tmux.Pane{
		{SessionName: "droid-app", WindowIndex: 0, PaneIndex: 0, ID: "%1", Command: "droid", Path: "/repo/app", Title: "feat/threads", AgentState: "working", AgentUpdated: time.Now().UnixMilli()},
		{SessionName: "work", WindowIndex: 1, PaneIndex: 2, ID: "%2", Command: "codex", Path: "/repo/app", Title: "codex review", AgentState: "input"},
	}

	rows := buildRows(sessions, panes)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, rows = %#v", len(rows), rows)
	}
	if rows[0].Kind != RowHeadless || rows[0].AgentID != "droid" {
		t.Fatalf("headless row = %#v", rows[0])
	}
	if rows[0].Title != "custom thread title" || rows[0].AgentState != "working" || rows[0].AgentSymbol != "⛬" {
		t.Fatalf("headless title/state/symbol = %#v", rows[0])
	}
	if rows[0].PaneID != "%1" || rows[0].AgentSessionID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("headless pane/session id = %#v", rows[0])
	}
	if rows[1].Kind != RowEphemeral || rows[1].AgentID != "codex" {
		t.Fatalf("ephemeral row = %#v", rows[1])
	}
	if rows[1].AgentState != "input" {
		t.Fatalf("ephemeral state = %q", rows[1].AgentState)
	}
}

func TestEnrichAgentTitlesUsesCodexThreadTitleAndSyncsTmux(t *testing.T) {
	originalLookup := lookupAgentTitle
	originalSync := syncThreadTitle
	originalPrefix := syncThreadPrefix
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		lookupAgentTitle = originalLookup
		syncThreadTitle = originalSync
		syncThreadPrefix = originalPrefix
		refreshThreadClient = originalRefresh
	})

	var syncedSession, syncedTitle, syncedPrefix, refreshedSession string
	lookupAgentTitle = func(target agentrename.Target) (string, error) {
		if target.AgentID != "codex" || target.PanePID != 67488 {
			t.Fatalf("target = %#v", target)
		}
		return "improve rename", nil
	}
	syncThreadTitle = func(sessionName, title string) error {
		syncedSession = sessionName
		syncedTitle = title
		return nil
	}
	syncThreadPrefix = func(sessionName, prefix string) error {
		if sessionName != "codex-kitmux" {
			t.Fatalf("prefix session = %q", sessionName)
		}
		syncedPrefix = prefix
		return nil
	}
	refreshThreadClient = func(sessionName string) error {
		refreshedSession = sessionName
		return nil
	}

	rows := enrichAgentTitles([]Row{{
		Kind:        RowHeadless,
		AgentID:     "codex",
		AgentSymbol: "⌾",
		AgentState:  "idle",
		Title:       "⠋ kitmux",
		SessionName: "codex-kitmux",
		PanePID:     67488,
	}})

	if rows[0].Title != "improve rename" {
		t.Fatalf("title = %q", rows[0].Title)
	}
	if syncedSession != "codex-kitmux" || syncedTitle != "improve rename" {
		t.Fatalf("synced session/title = %q/%q", syncedSession, syncedTitle)
	}
	if syncedPrefix != "⌾" {
		t.Fatalf("synced prefix = %q", syncedPrefix)
	}
	if refreshedSession != "codex-kitmux" {
		t.Fatalf("refreshed session = %q", refreshedSession)
	}
}

func TestReconcilePaneTitleRenamesSyncsLivePaneTitleOverride(t *testing.T) {
	originalSync := syncThreadTitle
	originalPrefix := syncThreadPrefix
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		syncThreadTitle = originalSync
		syncThreadPrefix = originalPrefix
		refreshThreadClient = originalRefresh
	})

	var syncedSession, syncedTitle, syncedPrefix, refreshedSession string
	syncThreadTitle = func(sessionName, title string) error {
		syncedSession = sessionName
		syncedTitle = title
		return nil
	}
	syncThreadPrefix = func(sessionName, prefix string) error {
		if sessionName != "droid-kitmux" {
			t.Fatalf("prefix session = %q", sessionName)
		}
		syncedPrefix = prefix
		return nil
	}
	refreshThreadClient = func(sessionName string) error {
		refreshedSession = sessionName
		return nil
	}

	rows := reconcilePaneTitleRenames([]Row{{
		Kind:        RowHeadless,
		AgentID:     "droid",
		AgentName:   "Droid",
		AgentSymbol: "⛬",
		AgentState:  "idle",
		Title:       "hello test",
		ThreadTitle: "hello test",
		PaneTitle:   "⛬ test name",
		SessionName: "droid-kitmux",
		Path:        "/repo/app",
	}})

	if rows[0].Title != "test name" || rows[0].ThreadTitle != "test name" {
		t.Fatalf("titles = %q/%q", rows[0].Title, rows[0].ThreadTitle)
	}
	if !rows[0].TitleOverride {
		t.Fatal("expected title override")
	}
	if syncedSession != "droid-kitmux" || syncedTitle != "test name" {
		t.Fatalf("synced session/title = %q/%q", syncedSession, syncedTitle)
	}
	if syncedPrefix != "⛬" {
		t.Fatalf("prefix = %q", syncedPrefix)
	}
	if refreshedSession != "droid-kitmux" {
		t.Fatalf("refreshed session = %q", refreshedSession)
	}
}

func TestReconcilePaneTitleRenamesIgnoresDefaultAgentTitle(t *testing.T) {
	originalSync := syncThreadTitle
	t.Cleanup(func() {
		syncThreadTitle = originalSync
	})
	syncThreadTitle = func(_, _ string) error {
		t.Fatal("syncThreadTitle should not run for default pane title")
		return nil
	}

	rows := reconcilePaneTitleRenames([]Row{{
		Kind:        RowHeadless,
		AgentID:     "droid",
		AgentName:   "Droid",
		Title:       "custom title",
		ThreadTitle: "custom title",
		PaneTitle:   "Droid",
		SessionName: "droid-kitmux",
	}})

	if rows[0].Title != "custom title" || rows[0].ThreadTitle != "custom title" {
		t.Fatalf("titles changed = %q/%q", rows[0].Title, rows[0].ThreadTitle)
	}
}

func TestReconcilePaneTitleRenamesPersistsFirstLiveRename(t *testing.T) {
	originalSync := syncThreadTitle
	originalPrefix := syncThreadPrefix
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		syncThreadTitle = originalSync
		syncThreadPrefix = originalPrefix
		refreshThreadClient = originalRefresh
	})

	var syncedTitle string
	syncThreadTitle = func(_, title string) error {
		syncedTitle = title
		return nil
	}
	syncThreadPrefix = func(_, _ string) error { return nil }
	refreshThreadClient = func(string) error { return nil }

	rows := reconcilePaneTitleRenames([]Row{{
		Kind:        RowHeadless,
		AgentID:     "droid",
		AgentName:   "Droid",
		AgentSymbol: "⛬",
		AgentState:  "idle",
		Title:       "⛬ issue triage",
		PaneTitle:   "⛬ issue triage",
		SessionName: "droid-kitmux",
		Path:        "/repo/app",
	}})

	if rows[0].Title != "issue triage" || rows[0].ThreadTitle != "issue triage" {
		t.Fatalf("titles = %q/%q", rows[0].Title, rows[0].ThreadTitle)
	}
	if syncedTitle != "issue triage" {
		t.Fatalf("synced title = %q", syncedTitle)
	}
}

func TestRepairThreadTitlePrefixesKeepsAgentIconForIdleRenamedThreads(t *testing.T) {
	originalPrefix := syncThreadPrefix
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		syncThreadPrefix = originalPrefix
		refreshThreadClient = originalRefresh
	})

	var synced []string
	syncThreadPrefix = func(sessionName, prefix string) error {
		synced = append(synced, sessionName+"="+prefix)
		return nil
	}
	refreshThreadClient = func(string) error { return nil }

	rows := repairThreadTitlePrefixes([]Row{
		{Kind: RowHeadless, AgentID: "droid", AgentSymbol: "⛬", AgentState: "idle", Title: "Greeting the assistant", TitleOverride: true, SessionName: "droid-kitmux"},
		{Kind: RowHeadless, AgentID: "claude", AgentSymbol: "✳", AgentState: "working", Title: "Still working", TitleOverride: true, SessionName: "claude-kitmux"},
	})

	if rows[0].Title != "Greeting the assistant" {
		t.Fatalf("title changed = %q", rows[0].Title)
	}
	if len(synced) != 1 || synced[0] != "droid-kitmux=⛬" {
		t.Fatalf("synced prefixes = %#v", synced)
	}
}

func TestRowIconAndTitleComposeStateGlyphWithoutDuplication(t *testing.T) {
	compose := func(row Row) string {
		return rowIcon(row, 0) + " " + rowTitle(row)
	}

	working := Row{AgentSymbol: "⛬", AgentState: "working", Title: "feat/threads"}
	if got := compose(working); got != "⠋ feat/threads" {
		t.Fatalf("working = %q", got)
	}

	input := Row{AgentSymbol: "✳", AgentState: "input", Title: "fix title"}
	if got := compose(input); got != "⮞ fix title" {
		t.Fatalf("input = %q", got)
	}

	permission := Row{AgentSymbol: "✳", AgentState: "permission", Title: "needs approval"}
	if got := compose(permission); got != "! needs approval" {
		t.Fatalf("permission = %q", got)
	}

	codexWorking := Row{AgentID: "codex", AgentSymbol: "›", AgentState: "working", Title: "⠹ kitmux"}
	if got := compose(codexWorking); got != "⠋ kitmux" {
		t.Fatalf("codex working = %q", got)
	}

	droidWorkingWithNativePrefix := Row{AgentID: "droid", AgentSymbol: "⛬", AgentState: "working", Title: "⠂ ⛬ Android app"}
	if got := compose(droidWorkingWithNativePrefix); got != "⠋ Android app" {
		t.Fatalf("droid working = %q", got)
	}

	idle := Row{AgentSymbol: "⛬", AgentState: "idle", Title: "⛬ Droid · app"}
	if got := compose(idle); got != "⛬ Droid · app" {
		t.Fatalf("idle = %q", got)
	}
}

func TestModelClampsScrollAfterEmptyEndKeyThenLoad(t *testing.T) {
	m := New()
	m.SetSize(80, 5)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m, _ = m.Update(loadedMsg{rows: []Row{{Kind: RowHeadless, SessionName: "droid-app", AgentName: "Droid"}}})

	if m.scroll != 0 || m.cursor != 0 {
		t.Fatalf("cursor=%d scroll=%d, want both 0", m.cursor, m.scroll)
	}
	if got := m.View(); got == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestRelaunchKeyReturnsCommandForHeadlessRows(t *testing.T) {
	m := New()
	m, _ = m.Update(loadedMsg{rows: []Row{{
		Kind:           RowHeadless,
		AgentID:        "droid",
		SessionName:    "droid-app",
		PanePID:        123,
		AgentSessionID: "11111111-1111-4111-8111-111111111111",
	}}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	if cmd == nil {
		t.Fatal("expected relaunch command")
	}
}

func TestRenameKeyStillStartsRename(t *testing.T) {
	m := New()
	m, _ = m.Update(loadedMsg{rows: []Row{{Kind: RowHeadless, SessionName: "droid-app", Title: "Droid app"}}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if !m.renaming {
		t.Fatal("expected rename mode")
	}
}

func TestRenameHeadlessSyncsThreadAndPaneTitle(t *testing.T) {
	originalSyncThread := syncThreadTitle
	originalSyncPrefix := syncThreadPrefix
	originalSyncPane := syncPaneTitle
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		syncThreadTitle = originalSyncThread
		syncThreadPrefix = originalSyncPrefix
		syncPaneTitle = originalSyncPane
		refreshThreadClient = originalRefresh
	})

	var threadTitle, paneTarget, paneTitle, prefix, refreshed string
	syncThreadTitle = func(sessionName, title string) error {
		if sessionName != "droid-kitmux" {
			t.Fatalf("thread session = %q", sessionName)
		}
		threadTitle = title
		return nil
	}
	syncPaneTitle = func(target, title string) error {
		paneTarget = target
		paneTitle = title
		return nil
	}
	syncThreadPrefix = func(_, value string) error {
		prefix = value
		return nil
	}
	refreshThreadClient = func(sessionName string) error {
		refreshed = sessionName
		return nil
	}

	err := renameRow(Row{
		Kind:        RowHeadless,
		AgentID:     "droid",
		AgentSymbol: "⛬",
		AgentState:  "idle",
		SessionName: "droid-kitmux",
		PaneID:      "%51",
		PanePID:     6396,
	}, "Hello")
	if err != nil {
		t.Fatalf("renameRow() error = %v", err)
	}
	if threadTitle != "Hello" || paneTarget != "%51" || paneTitle != "Hello" {
		t.Fatalf("thread/pane titles = %q/%q:%q", threadTitle, paneTarget, paneTitle)
	}
	if prefix != "⛬" {
		t.Fatalf("prefix = %q", prefix)
	}
	if refreshed != "droid-kitmux" {
		t.Fatalf("refreshed = %q", refreshed)
	}
}

func TestNewHeadlessUsesLaunchDir(t *testing.T) {
	originalCreateThread := createThread
	t.Cleanup(func() {
		createThread = originalCreateThread
	})

	var gotSpec agentthread.Spec
	createThread = func(spec agentthread.Spec, _ agentthread.Ops) (agentthread.Resolved, error) {
		gotSpec = spec
		return agentthread.Resolved{SessionName: "droid-current"}, nil
	}

	agent, ok := agents.Find("droid")
	if !ok {
		t.Fatal("missing droid agent")
	}

	msg := newHeadlessCmd(agent, "/repo/current")()
	if gotSpec.Dir != "/repo/current" {
		t.Fatalf("dir = %q, want launch dir", gotSpec.Dir)
	}
	if got, ok := msg.(messages.SwitchSessionMsg); !ok || got.Name != "droid-current" {
		t.Fatalf("msg = %#v", msg)
	}
}
