package threads

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agentrename"
	"github.com/miltonparedes/kitmux/internal/agentresume"
	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/agentthread"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

const droidKitmuxSession = "droid-kitmux"

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

func TestFilterRowsKeepsOnlyMatchingDirectory(t *testing.T) {
	root := t.TempDir()
	matchDir := filepath.Join(root, "app")
	otherDir := filepath.Join(root, "other")
	if err := os.MkdirAll(matchDir, 0o750); err != nil {
		t.Fatalf("mkdir match dir: %v", err)
	}
	if err := os.MkdirAll(otherDir, 0o750); err != nil {
		t.Fatalf("mkdir other dir: %v", err)
	}
	rows := []Row{
		{Kind: RowHeadless, SessionName: "droid-app", Path: matchDir},
		{Kind: RowEphemeral, SessionName: "work", PaneID: "%1", Path: matchDir},
		{Kind: RowHeadless, SessionName: "droid-pane", Path: otherDir, PanePath: matchDir},
		{Kind: RowHeadless, SessionName: "codex-other", Path: otherDir},
		{Kind: RowEphemeral, SessionName: "empty"},
	}

	got := filterRows(rows, loadOptions{filterDir: matchDir})

	if len(got) != 3 {
		t.Fatalf("len(got) = %d, got = %#v", len(got), got)
	}
	if got[0].Kind != RowHeadless || got[0].SessionName != "droid-app" {
		t.Fatalf("first row = %#v", got[0])
	}
	if got[1].Kind != RowEphemeral || got[1].SessionName != "work" {
		t.Fatalf("second row = %#v", got[1])
	}
	if got[2].Kind != RowHeadless || got[2].SessionName != "droid-pane" {
		t.Fatalf("third row = %#v", got[2])
	}
}

func TestFilterRowsShowAllSkipsDirectoryFilter(t *testing.T) {
	rows := []Row{
		{Kind: RowHeadless, SessionName: "droid-app", Path: "/repo/app"},
		{Kind: RowEphemeral, SessionName: "work", Path: "/repo/app"},
		{Kind: RowHeadless, SessionName: "codex-other", Path: "/repo/other"},
	}

	got := filterRows(rows, loadOptions{filterDir: "/repo/app", showAll: true})

	if len(got) != len(rows) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(rows))
	}
	for i := range rows {
		if got[i].SessionName != rows[i].SessionName {
			t.Fatalf("row %d = %q, want %q", i, got[i].SessionName, rows[i].SessionName)
		}
	}
}

func TestPrepareRowsReconcilesHiddenRowsBeforeFiltering(t *testing.T) {
	originalSync := syncThreadTitle
	originalPrefix := syncThreadPrefix
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		syncThreadTitle = originalSync
		syncThreadPrefix = originalPrefix
		refreshThreadClient = originalRefresh
	})

	var synced []string
	syncThreadTitle = func(sessionName, title string) error {
		synced = append(synced, sessionName+"="+title)
		return nil
	}
	syncThreadPrefix = func(_, _ string) error { return nil }
	refreshThreadClient = func(string) error { return nil }

	root := t.TempDir()
	matchDir := filepath.Join(root, "app")
	otherDir := filepath.Join(root, "other")
	if err := os.MkdirAll(matchDir, 0o750); err != nil {
		t.Fatalf("mkdir match dir: %v", err)
	}
	if err := os.MkdirAll(otherDir, 0o750); err != nil {
		t.Fatalf("mkdir other dir: %v", err)
	}

	rows := prepareRows(
		[]tmux.Session{
			{Name: "droid-app", Path: matchDir, Thread: true, AgentID: "droid", AgentState: "idle"},
			{Name: "droid-other", Path: otherDir, Thread: true, AgentID: "droid", AgentState: "idle", ThreadTitle: "old title"},
		},
		[]tmux.Pane{
			{SessionName: "droid-app", Path: matchDir, Title: "droid"},
			{SessionName: "droid-other", Path: otherDir, Title: "new hidden title"},
		},
		loadOptions{filterDir: matchDir},
	)

	if len(rows) != 1 || rows[0].SessionName != "droid-app" {
		t.Fatalf("filtered rows = %#v", rows)
	}
	if len(synced) != 1 || synced[0] != "droid-other=new hidden title" {
		t.Fatalf("synced titles = %#v", synced)
	}
}

func TestRowAgentMetadataUsesNewestExplicitState(t *testing.T) {
	now := time.Now().UnixMilli()
	state, event, _, updated := rowAgentMetadata(
		agentMetadata{State: "input", Event: "notification", Updated: now - 5000},
		agentMetadata{State: "idle", Event: "stop", Updated: now},
	)
	if state != "idle" || event != "stop" || updated != now {
		t.Fatalf("metadata = %q/%q/%d, want idle/stop/%d", state, event, updated, now)
	}

	state, event, _, updated = rowAgentMetadata(
		agentMetadata{State: "working", Event: "pre-tool-use", Updated: now - 5000},
		agentMetadata{State: "input", Event: "notification", Updated: now},
	)
	if state != "input" || event != "notification" || updated != now {
		t.Fatalf("metadata = %q/%q/%d, want input/notification/%d", state, event, updated, now)
	}
}

func TestRowAgentMetadataIgnoresStaleWorkingState(t *testing.T) {
	now := time.Now().UnixMilli()
	stale := time.Now().Add(-3 * time.Hour).UnixMilli()
	state, event, _, updated := rowAgentMetadata(
		agentMetadata{State: "working", Event: "pre-tool-use", Updated: stale},
		agentMetadata{State: "idle", Event: "stop", Updated: now},
	)
	if state != "idle" || event != "stop" || updated != now {
		t.Fatalf("metadata = %q/%q/%d, want idle/stop/%d", state, event, updated, now)
	}
}

func TestRowAgentMetadataBreaksTimestampTiesTowardAttentionOrIdle(t *testing.T) {
	now := time.Now().UnixMilli()
	tests := []struct {
		name  string
		left  agentMetadata
		right agentMetadata
		want  string
	}{
		{
			name:  "input beats working",
			left:  agentMetadata{State: "working", Event: "pre-tool-use", Updated: now},
			right: agentMetadata{State: "input", Event: "notification", Updated: now},
			want:  "input",
		},
		{
			name:  "idle beats working",
			left:  agentMetadata{State: "working", Event: "pre-tool-use", Updated: now},
			right: agentMetadata{State: "idle", Event: "stop", Updated: now},
			want:  "idle",
		},
	}
	for _, tc := range tests {
		state, _, _, _ := rowAgentMetadata(tc.left, tc.right)
		if state != tc.want {
			t.Fatalf("%s state = %q, want %q", tc.name, state, tc.want)
		}
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
		if sessionName != droidKitmuxSession {
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
		SessionName: droidKitmuxSession,
		Path:        "/repo/app",
	}})

	if rows[0].Title != "test name" || rows[0].ThreadTitle != "test name" {
		t.Fatalf("titles = %q/%q", rows[0].Title, rows[0].ThreadTitle)
	}
	if !rows[0].TitleOverride {
		t.Fatal("expected title override")
	}
	if syncedSession != droidKitmuxSession || syncedTitle != "test name" {
		t.Fatalf("synced session/title = %q/%q", syncedSession, syncedTitle)
	}
	if syncedPrefix != "⛬" {
		t.Fatalf("prefix = %q", syncedPrefix)
	}
	if refreshedSession != droidKitmuxSession {
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
		SessionName: droidKitmuxSession,
	}})

	if rows[0].Title != "custom title" || rows[0].ThreadTitle != "custom title" {
		t.Fatalf("titles changed = %q/%q", rows[0].Title, rows[0].ThreadTitle)
	}
}

func TestReconcilePaneTitleRenamesIgnoresWorkerTitle(t *testing.T) {
	originalSync := syncThreadTitle
	t.Cleanup(func() {
		syncThreadTitle = originalSync
	})
	syncThreadTitle = func(_, _ string) error {
		t.Fatal("syncThreadTitle should not run for transient worker title")
		return nil
	}

	rows := reconcilePaneTitleRenames([]Row{{
		Kind:        RowHeadless,
		AgentID:     "droid",
		AgentName:   "Droid",
		Title:       "hooks",
		ThreadTitle: "hooks",
		PaneTitle:   "⛬ Worker: Audit hook runtime",
		SessionName: droidKitmuxSession,
	}})

	if rows[0].Title != "hooks" || rows[0].ThreadTitle != "hooks" {
		t.Fatalf("titles changed = %q/%q", rows[0].Title, rows[0].ThreadTitle)
	}
}

func TestReconcilePaneTitleRenamesIgnoresAgentDisplayTitle(t *testing.T) {
	originalSync := syncThreadTitle
	t.Cleanup(func() {
		syncThreadTitle = originalSync
	})
	syncThreadTitle = func(_, _ string) error {
		t.Fatal("syncThreadTitle should not run for agent display title")
		return nil
	}

	rows := reconcilePaneTitleRenames([]Row{{
		Kind:              RowHeadless,
		AgentID:           "droid",
		AgentName:         "Droid",
		Title:             "Droid · kitmux",
		AgentTitleDisplay: "Authenticate Logfire MCP server",
		PaneTitle:         "⛬ Authenticate Logfire MCP server",
		SessionName:       droidKitmuxSession,
	}})

	if rows[0].Title != "Droid · kitmux" || rows[0].ThreadTitle != "" {
		t.Fatalf("titles changed = %q/%q", rows[0].Title, rows[0].ThreadTitle)
	}
}

func TestBuildRowsFallsBackToInitialTitleBeforePaneTitle(t *testing.T) {
	rows := buildRows(
		[]tmux.Session{{
			Name:         droidKitmuxSession,
			Path:         "/repo/kitmux",
			Thread:       true,
			AgentID:      "droid",
			InitialTitle: "⛬ Droid · kitmux",
		}},
		[]tmux.Pane{{
			SessionName:       droidKitmuxSession,
			ID:                "%1",
			Path:              "/repo/kitmux",
			Title:             "⛬ Authenticate Logfire MCP server",
			AgentTitleDisplay: "Authenticate Logfire MCP server",
		}},
	)

	if rows[0].Title != "⛬ Droid · kitmux" {
		t.Fatalf("title = %q", rows[0].Title)
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
		SessionName: droidKitmuxSession,
		Path:        "/repo/app",
	}})

	if rows[0].Title != "issue triage" || rows[0].ThreadTitle != "issue triage" {
		t.Fatalf("titles = %q/%q", rows[0].Title, rows[0].ThreadTitle)
	}
	if syncedTitle != "issue triage" {
		t.Fatalf("synced title = %q", syncedTitle)
	}
}

func TestReconcilePaneTitleRenamesSupportsRegisteredAgents(t *testing.T) {
	tests := []struct {
		name       string
		row        Row
		wantTitle  string
		wantPrefix string
	}{
		{
			name: "claude",
			row: Row{
				Kind:        RowHeadless,
				AgentID:     "claude",
				AgentName:   "Claude Code",
				AgentSymbol: "✳",
				AgentState:  "idle",
				ThreadTitle: "old claude title",
				PaneTitle:   "✳ review auth flow",
				SessionName: "claude-kitmux",
				Path:        "/repo/app",
			},
			wantTitle:  "review auth flow",
			wantPrefix: "✳",
		},
		{
			name: "cursor",
			row: Row{
				Kind:        RowHeadless,
				AgentID:     "cursor",
				AgentName:   "Cursor CLI",
				AgentSymbol: "⌬",
				AgentState:  "idle",
				ThreadTitle: "old cursor title",
				PaneTitle:   "⌬ fix search panel",
				SessionName: "cursor-kitmux",
				Path:        "/repo/app",
			},
			wantTitle:  "fix search panel",
			wantPrefix: "⌬",
		},
		{
			name: "opencode",
			row: Row{
				Kind:        RowHeadless,
				AgentID:     "opencode",
				AgentName:   "OpenCode",
				AgentState:  "idle",
				ThreadTitle: "old opencode title",
				PaneTitle:   "audit release notes",
				SessionName: "opencode-kitmux",
				Path:        "/repo/app",
			},
			wantTitle:  "audit release notes",
			wantPrefix: "O",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				syncedPrefix = prefix
				return nil
			}
			refreshThreadClient = func(sessionName string) error {
				refreshedSession = sessionName
				return nil
			}

			rows := reconcilePaneTitleRenames([]Row{tt.row})

			if rows[0].Title != tt.wantTitle || rows[0].ThreadTitle != tt.wantTitle {
				t.Fatalf("titles = %q/%q, want %q", rows[0].Title, rows[0].ThreadTitle, tt.wantTitle)
			}
			if syncedSession != tt.row.SessionName || syncedTitle != tt.wantTitle {
				t.Fatalf("synced session/title = %q/%q", syncedSession, syncedTitle)
			}
			if syncedPrefix != tt.wantPrefix {
				t.Fatalf("prefix = %q, want %q", syncedPrefix, tt.wantPrefix)
			}
			if refreshedSession != tt.row.SessionName {
				t.Fatalf("refreshed session = %q", refreshedSession)
			}
		})
	}
}

func TestReconcilePaneTitleRenamesAgainstTmux(t *testing.T) {
	if os.Getenv("KITMUX_TMUX_INTEGRATION") != "1" {
		t.Skip("set KITMUX_TMUX_INTEGRATION=1 to run tmux integration validation")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}

	tests := []struct {
		agent      string
		paneTitle  string
		wantTitle  string
		wantPrefix string
	}{
		{agent: "claude", paneTitle: "✳ review auth flow", wantTitle: "review auth flow", wantPrefix: "✳"},
		{agent: "cursor", paneTitle: "⌬ fix search panel", wantTitle: "fix search panel", wantPrefix: "⌬"},
		{agent: "opencode", paneTitle: "audit release notes", wantTitle: "audit release notes", wantPrefix: "O"},
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			sessionName := fmt.Sprintf("kitmux-test-%s-%d", tt.agent, time.Now().UnixNano())
			tmuxRun(t, "new-session", "-d", "-s", sessionName, "sleep 60")
			t.Cleanup(func() {
				_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
			})
			tmuxRun(t, "set-option", "-q", "-t", sessionName, "@kitmux_thread", "1")
			tmuxRun(t, "set-option", "-q", "-t", sessionName, "@kitmux_agent", tt.agent)
			tmuxRun(t, "set-option", "-q", "-t", sessionName, "@kitmux_agent_state", "idle")
			tmuxRun(t, "set-option", "-q", "-t", sessionName, "@kitmux_thread_title", "old "+tt.agent)
			tmuxRun(t, "select-pane", "-t", sessionName, "-T", tt.paneTitle)

			msg := loadRows()
			row, ok := findRowBySession(msg.rows, sessionName)
			if !ok {
				t.Fatalf("session %q not found in rows", sessionName)
			}
			if row.Title != tt.wantTitle || row.ThreadTitle != tt.wantTitle {
				t.Fatalf("row titles = %q/%q, want %q", row.Title, row.ThreadTitle, tt.wantTitle)
			}

			gotTitle := strings.TrimSpace(tmuxOutput(t, "show-option", "-qv", "-t", sessionName, "@kitmux_thread_title"))
			if gotTitle != tt.wantTitle {
				t.Fatalf("@kitmux_thread_title = %q, want %q", gotTitle, tt.wantTitle)
			}
			gotPrefix := strings.TrimSpace(tmuxOutput(t, "show-option", "-qv", "-t", sessionName, "@kitmux_agent_title_prefix"))
			if gotPrefix != tt.wantPrefix {
				t.Fatalf("@kitmux_agent_title_prefix = %q, want %q", gotPrefix, tt.wantPrefix)
			}
		})
	}
}

func tmuxRun(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("tmux", args...).CombinedOutput(); err != nil {
		t.Fatalf("tmux %s: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

func tmuxOutput(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		t.Fatalf("tmux %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

func findRowBySession(rows []Row, sessionName string) (Row, bool) {
	for _, row := range rows {
		if row.SessionName == sessionName {
			return row, true
		}
	}
	return Row{}, false
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
		{Kind: RowHeadless, AgentID: "droid", AgentSymbol: "⛬", AgentState: "idle", Title: "Greeting the assistant", TitleOverride: true, SessionName: droidKitmuxSession},
		{Kind: RowHeadless, AgentID: "claude", AgentSymbol: "✳", AgentState: "working", Title: "Still working", TitleOverride: true, SessionName: "claude-kitmux"},
	})

	if rows[0].Title != "Greeting the assistant" {
		t.Fatalf("title changed = %q", rows[0].Title)
	}
	if len(synced) != 1 || synced[0] != droidKitmuxSession+"=⛬" {
		t.Fatalf("synced prefixes = %#v", synced)
	}
}

func TestSyncThreadTitleStateSkipsUnchangedValues(t *testing.T) {
	originalSync := syncThreadTitle
	originalPrefix := syncThreadPrefix
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		syncThreadTitle = originalSync
		syncThreadPrefix = originalPrefix
		refreshThreadClient = originalRefresh
	})

	syncThreadTitle = func(_, _ string) error {
		t.Fatal("syncThreadTitle should not run for unchanged title")
		return nil
	}
	syncThreadPrefix = func(_, _ string) error {
		t.Fatal("syncThreadPrefix should not run for unchanged prefix")
		return nil
	}
	refreshThreadClient = func(string) error {
		t.Fatal("refreshThreadClient should not run for unchanged state")
		return nil
	}

	err := syncThreadTitleState(droidKitmuxSession, threadTitleState{
		title:         "hooks",
		setTitle:      true,
		currentTitle:  "hooks",
		prefix:        "⛬",
		setPrefix:     true,
		currentPrefix: "⛬",
	})
	if err != nil {
		t.Fatalf("syncThreadTitleState() error = %v", err)
	}
}

func TestSyncThreadTitleStateRefreshesOnceForChangedValues(t *testing.T) {
	originalSync := syncThreadTitle
	originalPrefix := syncThreadPrefix
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		syncThreadTitle = originalSync
		syncThreadPrefix = originalPrefix
		refreshThreadClient = originalRefresh
	})

	var syncedTitle, syncedPrefix string
	var refreshes int
	syncThreadTitle = func(_, title string) error {
		syncedTitle = title
		return nil
	}
	syncThreadPrefix = func(_, prefix string) error {
		syncedPrefix = prefix
		return nil
	}
	refreshThreadClient = func(string) error {
		refreshes++
		return nil
	}

	err := syncThreadTitleState(droidKitmuxSession, threadTitleState{
		title:         "new title",
		setTitle:      true,
		currentTitle:  "old title",
		prefix:        "⛬",
		setPrefix:     true,
		currentPrefix: "old",
	})
	if err != nil {
		t.Fatalf("syncThreadTitleState() error = %v", err)
	}
	if syncedTitle != "new title" || syncedPrefix != "⛬" {
		t.Fatalf("synced title/prefix = %q/%q", syncedTitle, syncedPrefix)
	}
	if refreshes != 1 {
		t.Fatalf("refreshes = %d, want 1", refreshes)
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

func TestRelaunchHeadlessUsesPersistedDroidSessionID(t *testing.T) {
	originalResolve := resolveAgentSession
	originalPersist := persistAgentSession
	originalWrap := wrapThreadCommand
	originalRespawn := respawnThreadPane
	originalSupport := applyThreadSupport
	t.Cleanup(func() {
		resolveAgentSession = originalResolve
		persistAgentSession = originalPersist
		wrapThreadCommand = originalWrap
		respawnThreadPane = originalRespawn
		applyThreadSupport = originalSupport
	})

	resolveAgentSession = func(agentresume.Target) (string, error) {
		t.Fatal("resolveAgentSession should not run when the row has a session id")
		return "", nil
	}
	var persistedSession, persistedID string
	persistAgentSession = func(sessionName, id string) error {
		persistedSession = sessionName
		persistedID = id
		return nil
	}
	wrapThreadCommand = func(agentID, sessionName, command string) string {
		if agentID != "droid" || sessionName != droidKitmuxSession {
			t.Fatalf("wrap target = %q/%q", agentID, sessionName)
		}
		return "wrapped:" + command
	}
	var respawnTarget, respawnDir, respawnCommand string
	respawnThreadPane = func(targetPane, dir, command string) error {
		respawnTarget = targetPane
		respawnDir = dir
		respawnCommand = command
		return nil
	}
	var supportSpec agentthread.SupportSpec
	applyThreadSupport = func(spec agentthread.SupportSpec, _ agentthread.Ops) error {
		supportSpec = spec
		return nil
	}

	err := relaunchHeadless(Row{
		Kind:           RowHeadless,
		AgentID:        "droid",
		SessionName:    droidKitmuxSession,
		PaneID:         "%51",
		Path:           "/repo/app",
		Title:          "Droid app",
		AgentSessionID: "abc123",
	})
	if err != nil {
		t.Fatalf("relaunchHeadless() error = %v", err)
	}
	if persistedSession != droidKitmuxSession || persistedID != "abc123" {
		t.Fatalf("persisted session/id = %q/%q", persistedSession, persistedID)
	}
	if respawnTarget != "%51" || respawnDir != "/repo/app" || respawnCommand != "wrapped:droid --resume 'abc123'" {
		t.Fatalf("respawn = %q %q %q", respawnTarget, respawnDir, respawnCommand)
	}
	if supportSpec.SessionName != droidKitmuxSession || supportSpec.TargetPane != "%51" || supportSpec.AgentID != "droid" {
		t.Fatalf("support spec = %#v", supportSpec)
	}
}

func TestRelaunchHeadlessCanonicalizesDroidChildSessionID(t *testing.T) {
	originalResolve := resolveAgentSession
	originalPersist := persistAgentSession
	originalWrap := wrapThreadCommand
	originalRespawn := respawnThreadPane
	originalSupport := applyThreadSupport
	t.Cleanup(func() {
		resolveAgentSession = originalResolve
		persistAgentSession = originalPersist
		wrapThreadCommand = originalWrap
		respawnThreadPane = originalRespawn
		applyThreadSupport = originalSupport
	})

	root := t.TempDir()
	t.Setenv("HOME", root)
	childID := "22222222-2222-4222-8222-222222222222"
	parentID := "11111111-1111-4111-8111-111111111111"
	childPath := filepath.Join(root, ".factory", "sessions", "-repo-app", childID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(childPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		childPath,
		[]byte(`{"type":"session_start","id":"`+childID+`","callingSessionId":"`+parentID+`"}`+"\n"),
		0o600,
	); err != nil {
		t.Fatalf("write child session: %v", err)
	}

	resolveAgentSession = func(agentresume.Target) (string, error) {
		t.Fatal("resolveAgentSession should not run when canonical parent id is available")
		return "", nil
	}
	var persistedID string
	persistAgentSession = func(_, id string) error {
		persistedID = id
		return nil
	}
	wrapThreadCommand = func(_, _, command string) string {
		return "wrapped:" + command
	}
	var respawnCommand string
	respawnThreadPane = func(_, _, command string) error {
		respawnCommand = command
		return nil
	}
	applyThreadSupport = func(agentthread.SupportSpec, agentthread.Ops) error {
		return nil
	}

	err := relaunchHeadless(Row{
		Kind:           RowHeadless,
		AgentID:        "droid",
		SessionName:    droidKitmuxSession,
		PaneID:         "%51",
		Path:           "/repo/app",
		Title:          "hooks",
		AgentSessionID: childID,
	})
	if err != nil {
		t.Fatalf("relaunchHeadless() error = %v", err)
	}
	if persistedID != parentID {
		t.Fatalf("persisted id = %q, want %q", persistedID, parentID)
	}
	if respawnCommand != "wrapped:droid --resume '"+parentID+"'" {
		t.Fatalf("respawn command = %q", respawnCommand)
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
	originalSyncWindow := syncWindowTitle
	originalRefresh := refreshThreadClient
	t.Cleanup(func() {
		syncThreadTitle = originalSyncThread
		syncThreadPrefix = originalSyncPrefix
		syncPaneTitle = originalSyncPane
		syncWindowTitle = originalSyncWindow
		refreshThreadClient = originalRefresh
	})

	var threadTitle, paneTarget, paneTitle, windowTarget, windowTitle, prefix, refreshed string
	syncThreadTitle = func(sessionName, title string) error {
		if sessionName != droidKitmuxSession {
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
	syncWindowTitle = func(target, title string) error {
		windowTarget = target
		windowTitle = title
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
		SessionName: droidKitmuxSession,
		PaneID:      "%51",
		PanePID:     6396,
	}, "Hello")
	if err != nil {
		t.Fatalf("renameRow() error = %v", err)
	}
	if threadTitle != "Hello" || paneTarget != "%51" || paneTitle != "Hello" {
		t.Fatalf("thread/pane titles = %q/%q:%q", threadTitle, paneTarget, paneTitle)
	}
	if windowTarget != droidKitmuxSession+":0" || windowTitle != "Hello" {
		t.Fatalf("window title = %q:%q", windowTarget, windowTitle)
	}
	if prefix != "⛬" {
		t.Fatalf("prefix = %q", prefix)
	}
	if refreshed != droidKitmuxSession {
		t.Fatalf("refreshed = %q", refreshed)
	}
}

func TestNewHeadlessUsesLaunchDir(t *testing.T) {
	originalCreateThread := createThread
	originalInstallHooks := installThreadHooks
	t.Cleanup(func() {
		createThread = originalCreateThread
		installThreadHooks = originalInstallHooks
	})

	var gotSpec agentthread.Spec
	var installedAgent string
	createThread = func(spec agentthread.Spec, _ agentthread.Ops) (agentthread.Resolved, error) {
		gotSpec = spec
		return agentthread.Resolved{SessionName: "droid-current"}, nil
	}
	installThreadHooks = func(agentID string) error {
		installedAgent = agentID
		return nil
	}

	agent, ok := agents.Find("droid")
	if !ok {
		t.Fatal("missing droid agent")
	}

	msg := newHeadlessCmd(agent, "/repo/current")()
	if gotSpec.Dir != "/repo/current" {
		t.Fatalf("dir = %q, want launch dir", gotSpec.Dir)
	}
	if installedAgent != "droid" {
		t.Fatalf("installed agent = %q", installedAgent)
	}
	if got, ok := msg.(messages.SwitchSessionMsg); !ok || got.Name != "droid-current" {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestNewHeadlessReportsHookInstallFailure(t *testing.T) {
	originalCreateThread := createThread
	originalInstallHooks := installThreadHooks
	t.Cleanup(func() {
		createThread = originalCreateThread
		installThreadHooks = originalInstallHooks
	})

	createThread = func(agentthread.Spec, agentthread.Ops) (agentthread.Resolved, error) {
		t.Fatal("createThread should not run when hook install fails")
		return agentthread.Resolved{}, nil
	}
	installThreadHooks = func(string) error {
		return fmt.Errorf("hooks unavailable")
	}

	agent, ok := agents.Find("droid")
	if !ok {
		t.Fatal("missing droid agent")
	}

	msg := newHeadlessCmd(agent, "/repo/current")()
	if _, ok := msg.(loadedMsg); !ok {
		t.Fatalf("msg = %#v, want loadedMsg", msg)
	}
}
