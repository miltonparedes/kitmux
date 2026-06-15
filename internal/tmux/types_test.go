package tmux

import "testing"

func TestNormalSessionsFiltersThreads(t *testing.T) {
	sessions := []Session{
		{Name: "app"},
		{Name: "droid-app", Thread: true},
		{Name: "api"},
	}

	got := NormalSessions(sessions)
	if len(got) != 2 || got[0].Name != "app" || got[1].Name != "api" {
		t.Fatalf("NormalSessions() = %#v", got)
	}
}

func TestThreadSessionsFiltersNormalSessions(t *testing.T) {
	sessions := []Session{
		{Name: "app"},
		{Name: "droid-app", Thread: true, AgentID: "droid"},
		{Name: "api"},
	}

	got := ThreadSessions(sessions)
	if len(got) != 1 || got[0].Name != "droid-app" || got[0].AgentID != "droid" {
		t.Fatalf("ThreadSessions() = %#v", got)
	}
}
