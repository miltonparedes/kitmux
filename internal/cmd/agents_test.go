package cmd

import (
	"reflect"
	"testing"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/agentthread"
)

func TestHeadlessAgentCreatesAndAttachesUniqueThread(t *testing.T) {
	originalOps := agentThreadOps
	originalInstall := installLaunchHooksFn
	t.Cleanup(func() {
		agentThreadOps = originalOps
		installLaunchHooksFn = originalInstall
	})

	existing := map[string]bool{"droid-app": true}
	var calls []string
	agentThreadOps = func() agentthread.Ops {
		return agentthread.Ops{
			HasSession: func(name string) bool {
				return existing[name]
			},
			NewSessionWithCommand: func(name, dir, _ string) (string, error) {
				calls = append(calls, "new:"+name+":"+dir)
				existing[name] = true
				return "%2", nil
			},
			SetSessionOption: func(_, _, _ string) error { return nil },
			SetWindowOption:  func(_, _, _ string) error { return nil },
			SetPaneTitle:     func(_, _ string) error { return nil },
			SetHook:          func(_, _, _ string) error { return nil },
			Attach: func(name string) error {
				calls = append(calls, "attach:"+name)
				return nil
			},
		}
	}
	installLaunchHooksFn = func(agentID string) error {
		calls = append(calls, "install:"+agentID)
		return nil
	}

	agent, ok := agents.Find("droid")
	if !ok {
		t.Fatal("missing droid agent")
	}
	cmd := agentCmd(agent)
	cmd.SetArgs([]string{"--headless", "--dir", "/tmp/app"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := []string{"install:droid", "new:droid-app-2:/tmp/app", "attach:droid-app-2"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}
