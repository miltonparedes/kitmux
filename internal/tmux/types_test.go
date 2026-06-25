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

func TestParseSessionsOutputReadsThreadTitleAndAgentSessionID(t *testing.T) {
	output := "codex-kitmux\t1\t1\t/Users/me/kitmux\t1781300000\t1\tcodex\tworking\tturn\tcmd\t1781300000000\tRenamed thread\t22222222-2222-4222-8222-222222222222\t⌾\tRenamed thread\n"

	got := parseSessionsOutput(output)
	if len(got) != 1 {
		t.Fatalf("sessions = %#v", got)
	}
	session := got[0]
	if session.Name != "codex-kitmux" || !session.Thread || session.AgentID != "codex" {
		t.Fatalf("session identity = %#v", session)
	}
	if session.ThreadTitle != "Renamed thread" {
		t.Fatalf("ThreadTitle = %q", session.ThreadTitle)
	}
	if session.AgentSessionID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("AgentSessionID = %q", session.AgentSessionID)
	}
	if session.AgentTitlePrefix != "⌾" || session.AgentTitleDisplay != "Renamed thread" {
		t.Fatalf("title prefix/display = %q/%q", session.AgentTitlePrefix, session.AgentTitleDisplay)
	}
}

func TestSingleLineOptionValue(t *testing.T) {
	got := singleLineOptionValue(" hello\n  world\tagain\r ")
	if got != "hello   world again" {
		t.Fatalf("singleLineOptionValue() = %q", got)
	}
}
