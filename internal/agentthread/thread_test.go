package agentthread

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestResolveDerivesStableSessionName(t *testing.T) {
	got, err := Resolve(Spec{
		AgentID: "droid",
		Dir:     "/tmp/my.project:main",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.SessionName != "droid-my-project-main" {
		t.Fatalf("SessionName = %q", got.SessionName)
	}
	if got.Title != "⛬ Droid · my.project:main" {
		t.Fatalf("Title = %q", got.Title)
	}
}

func TestEnsureCreatesMissingThread(t *testing.T) {
	var calls []string
	ops := Ops{
		HasSession: func(string) bool { return false },
		NewSessionWithCommand: func(name, dir, command string) (string, error) {
			calls = append(calls, "new:"+name+":"+dir+":"+command)
			return "%1", nil
		},
		SetSessionOption: func(_, option, value string) error {
			calls = append(calls, "session:"+option+"="+value)
			return nil
		},
		SetWindowOption: func(_, option, value string) error {
			calls = append(calls, "window:"+option+"="+value)
			return nil
		},
		SetPaneTitle: func(_, title string) error {
			calls = append(calls, "title:"+title)
			return nil
		},
		SetHook: func(_, hook, command string) error {
			calls = append(calls, "hook:"+hook+"="+command)
			return nil
		},
	}

	resolved, err := Ensure(Spec{AgentID: "droid", Dir: "/tmp/app"}, ops)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if resolved.SessionName != "droid-app" {
		t.Fatalf("SessionName = %q", resolved.SessionName)
	}
	wantPrefix := []string{
		"new:droid-app:/tmp/app:" + threadCommand("droid", "droid-app", "droid"),
		"session:status=off",
		"session:set-titles=on",
		"session:set-titles-string=" + threadTitleFormat(),
	}
	if !reflect.DeepEqual(calls[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("calls prefix = %#v", calls[:len(wantPrefix)])
	}
	if !contains(calls, "session:@kitmux_agent_support="+supportVersion) {
		t.Fatalf("missing support version option: %#v", calls)
	}
	if !contains(calls, "session:@kitmux_initial_title=⛬ Droid · app") {
		t.Fatalf("missing initial title option: %#v", calls)
	}
	if !contains(calls, "session:@kitmux_thread_base_title=Droid · app") {
		t.Fatalf("missing base title option: %#v", calls)
	}
	if !contains(calls, "session:@kitmux_agent_state=idle") {
		t.Fatalf("missing initial agent state option: %#v", calls)
	}
	if !contains(calls, `hook:alert-bell=`+bellHookCommand()) {
		t.Fatalf("missing alert-bell hook: %#v", calls)
	}
}

func TestThreadBaseTitleStripsAgentSymbol(t *testing.T) {
	got := threadBaseTitle(SupportSpec{AgentID: "droid", InitialTitle: "⛬ Droid · app"})
	if got != "Droid · app" {
		t.Fatalf("threadBaseTitle() = %q", got)
	}
}

func TestThreadTitleFormatKeepsSessionNameFallbackDynamic(t *testing.T) {
	got := threadTitleFormat()
	for _, forbidden := range []string{"#{@kitmux_thread_base_title}", "#{@kitmux_agent_title_display}", "#{pane_title}"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("threadTitleFormat() should not contain %q: %q", forbidden, got)
		}
	}
	if !strings.Contains(got, "#{session_name}") || !strings.Contains(got, "#{@kitmux_agent_title_prefix}") {
		t.Fatalf("threadTitleFormat() = %q", got)
	}
}

func TestEnsureSkipsExistingThreadCreate(t *testing.T) {
	called := false
	var titleTarget string
	ops := Ops{
		HasSession: func(string) bool { return true },
		NewSessionWithCommand: func(_, _, _ string) (string, error) {
			called = true
			return "", nil
		},
		SetSessionOption: func(_, _, _ string) error { return nil },
		SetWindowOption:  func(_, _, _ string) error { return nil },
		SetHook:          func(_, _, _ string) error { return nil },
		SetPaneTitle: func(target, _ string) error {
			titleTarget = target
			return nil
		},
	}

	if _, err := Ensure(Spec{AgentID: "droid", Dir: "/tmp/app"}, ops); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if called {
		t.Fatal("NewSessionWithCommand was called for existing thread")
	}
	if titleTarget != "" {
		t.Fatalf("title target = %q", titleTarget)
	}
}

func TestCreateGeneratesUniqueName(t *testing.T) {
	existing := map[string]bool{"droid-app": true}
	var created string
	ops := Ops{
		HasSession:            func(name string) bool { return existing[name] },
		NewSessionWithCommand: func(name, _, _ string) (string, error) { created = name; return "%2", nil },
		SetSessionOption:      func(_, _, _ string) error { return nil },
		SetWindowOption:       func(_, _, _ string) error { return nil },
		SetPaneTitle:          func(_, _ string) error { return nil },
		SetHook:               func(_, _, _ string) error { return nil },
	}

	resolved, err := Create(Spec{AgentID: "droid", Dir: "/tmp/app"}, ops)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resolved.SessionName != "droid-app-2" {
		t.Fatalf("SessionName = %q", resolved.SessionName)
	}
	if created != "droid-app-2" {
		t.Fatalf("created session = %q", created)
	}
}

func TestAttachSwitchesClientInsideTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-123/default,1,0")
	original := switchClient
	t.Cleanup(func() {
		switchClient = original
	})

	var switched string
	switchClient = func(target string) error {
		switched = target
		return nil
	}

	if err := Attach("droid-app-2"); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if switched != "droid-app-2" {
		t.Fatalf("switched target = %q", switched)
	}
}

func TestClientHooksDoNotTargetLiteralHookClient(t *testing.T) {
	for _, hook := range threadHooks() {
		if (hook.name == "client-attached" || hook.name == "client-session-changed") && hook.command != "refresh-client" {
			t.Fatalf("%s command = %q", hook.name, hook.command)
		}
	}
}

func TestInstallAllSupportUpdatesExistingThreads(t *testing.T) {
	var sessions []string
	var hooks []string
	ops := Ops{
		ListThreads: func() ([]tmux.Session, error) {
			return []tmux.Session{
				{Name: "droid-app", Path: "/tmp/app", AgentID: "droid"},
				{Name: "codex-api", Path: "/tmp/api", AgentID: "codex"},
			}, nil
		},
		SetSessionOption: func(target, _, _ string) error {
			sessions = append(sessions, target)
			return nil
		},
		SetWindowOption: func(_, _, _ string) error { return nil },
		SetHook: func(target, hook, _ string) error {
			hooks = append(hooks, target+":"+hook)
			return nil
		},
		SetPaneTitle: func(_, _ string) error {
			t.Fatal("SetPaneTitle should not run when installing support on existing threads")
			return nil
		},
	}

	count, err := InstallAllSupport(ops)
	if err != nil {
		t.Fatalf("InstallAllSupport() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d", count)
	}
	if !contains(sessions, "droid-app") || !contains(sessions, "codex-api") {
		t.Fatalf("sessions = %#v", sessions)
	}
	if !contains(hooks, "droid-app:alert-bell") || !contains(hooks, "codex-api:alert-bell") {
		t.Fatalf("hooks = %#v", hooks)
	}
}

func TestInstallAllSupportClearsStaleWorkingState(t *testing.T) {
	now := time.UnixMilli(1781897000000)
	options := map[string]map[string]string{}
	paneOptions := map[string]map[string]string{}
	ops := Ops{
		ListThreads: func() ([]tmux.Session, error) {
			return []tmux.Session{
				{Name: "droid-stale", Path: "/tmp/stale", AgentID: "droid", AgentState: "working", AgentUpdated: 2026},
				{Name: "droid-fresh", Path: "/tmp/fresh", AgentID: "droid", AgentState: "working", AgentUpdated: now.Add(-time.Minute).UnixMilli()},
			}, nil
		},
		SetSessionOption: func(target, option, value string) error {
			if options[target] == nil {
				options[target] = map[string]string{}
			}
			options[target][option] = value
			return nil
		},
		SetPaneOption: func(target, option, value string) error {
			if paneOptions[target] == nil {
				paneOptions[target] = map[string]string{}
			}
			paneOptions[target][option] = value
			return nil
		},
		SetWindowOption: func(_, _, _ string) error { return nil },
		SetHook:         func(_, _, _ string) error { return nil },
		Now:             func() time.Time { return now },
	}

	if _, err := InstallAllSupport(ops); err != nil {
		t.Fatalf("InstallAllSupport() error = %v", err)
	}
	stale := options["droid-stale"]
	if stale["@kitmux_agent_state"] != "idle" ||
		stale["@kitmux_agent_event"] != "stale-working" ||
		stale["@kitmux_agent_updated"] != "1781897000000" ||
		stale["@kitmux_agent_title_prefix"] != "⛬" {
		t.Fatalf("stale options = %#v", stale)
	}
	if paneOptions["droid-stale"]["@kitmux_agent_state"] != "idle" {
		t.Fatalf("stale pane options = %#v", paneOptions["droid-stale"])
	}
	if options["droid-fresh"]["@kitmux_agent_event"] == "stale-working" {
		t.Fatalf("fresh working state was cleared: %#v", options["droid-fresh"])
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func bellHookCommand() string {
	for _, hook := range threadHooks() {
		if hook.name == "alert-bell" {
			return hook.command
		}
	}
	return ""
}
