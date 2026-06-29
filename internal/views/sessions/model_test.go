package sessions

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

func sessionKeyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func modelWithSession(name string) Model {
	m := New()
	sessions := []tmux.Session{{Name: name, Path: "/tmp/" + name}}
	m.roots = BuildTree(sessions, nil)
	m.visible = Flatten(m.roots)
	return m
}

func TestConfirmKillKillsSessionAndReloads(t *testing.T) {
	originalKill := killTmuxSession
	originalList := listTmuxSessions
	defer func() {
		killTmuxSession = originalKill
		listTmuxSessions = originalList
	}()

	var killed string
	killTmuxSession = func(name string) error {
		killed = name
		return nil
	}
	listTmuxSessions = func() ([]tmux.Session, error) {
		return nil, nil
	}

	m := modelWithSession("kitmux-main")
	updated, _ := m.Update(sessionKeyMsg("d"))
	m = updated
	if !m.confirming {
		t.Fatal("expected kill confirmation")
	}

	updated, cmd := m.Update(sessionKeyMsg("y"))
	m = updated
	if m.confirming {
		t.Fatal("expected confirmation to close after y")
	}
	if cmd == nil {
		t.Fatal("expected kill command")
	}

	msg := cmd()
	if killed != "kitmux-main" {
		t.Fatalf("killed session = %q, want kitmux-main", killed)
	}
	loaded, ok := msg.(sessionsLoadedMsg)
	if !ok {
		t.Fatalf("message = %T, want sessionsLoadedMsg", msg)
	}
	if len(loaded.sessions) != 0 {
		t.Fatalf("expected reload with no sessions, got %+v", loaded.sessions)
	}
}

func TestConfirmKillReportsFailure(t *testing.T) {
	originalKill := killTmuxSession
	originalList := listTmuxSessions
	defer func() {
		killTmuxSession = originalKill
		listTmuxSessions = originalList
	}()

	killTmuxSession = func(string) error {
		return errors.New("tmux refused")
	}
	listTmuxSessions = func() ([]tmux.Session, error) {
		t.Fatal("listTmuxSessions should not run after kill failure")
		return nil, nil
	}

	m := modelWithSession("kitmux-main")
	updated, _ := m.Update(sessionKeyMsg("d"))
	m = updated
	updated, cmd := m.Update(sessionKeyMsg("y"))
	m = updated
	if cmd == nil {
		t.Fatal("expected kill command")
	}

	msg := cmd()
	if _, ok := msg.(sessionKillFailedMsg); !ok {
		t.Fatalf("message = %T, want sessionKillFailedMsg", msg)
	}
	updated, _ = m.Update(msg)
	m = updated
	if !strings.Contains(m.StatusLine(), "tmux refused") {
		t.Fatalf("expected failure in status line, got %q", m.StatusLine())
	}
}
