package agentthread

import (
	"reflect"
	"testing"

	"github.com/miltonparedes/kitmux/internal/agentenv"
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
		"new:droid-app:/tmp/app:" + agentenv.WrapTmuxCommand("droid", "droid-app", "droid", true),
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
	if !contains(calls, "session:@kitmux_agent_state=idle") {
		t.Fatalf("missing initial agent state option: %#v", calls)
	}
	if !contains(calls, `hook:alert-bell=`+bellHookCommand()) {
		t.Fatalf("missing alert-bell hook: %#v", calls)
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
