package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func appKeyMsg(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestHandleKeyMsgQFallsThroughForWorkspacesView(t *testing.T) {
	m := New(ModeSessions)
	m.view = viewWorkspaces

	updated, cmd, handled := m.handleKeyMsg(appKeyMsg("q"))
	if handled {
		t.Fatal("expected app-level q to fall through for workspaces view")
	}
	if cmd != nil {
		t.Fatal("expected no command when q falls through")
	}
	if updated.view != viewWorkspaces {
		t.Fatalf("expected to remain on workspaces view, got %d", updated.view)
	}
}

func TestHandleKeyMsgAFallsThroughForWorkspacesView(t *testing.T) {
	m := New(ModeSessions)
	m.view = viewWorkspaces

	updated, _, handled := m.handleKeyMsg(appKeyMsg("a"))
	if handled {
		t.Fatal("expected app-level a to fall through for workspaces view")
	}
	if updated.view != viewWorkspaces {
		t.Fatalf("expected to remain on workspaces view, got %d", updated.view)
	}
}

func TestHandleKeyMsgWFallsThroughForWorkspacesView(t *testing.T) {
	m := New(ModeSessions)
	m.view = viewWorkspaces

	updated, _, handled := m.handleKeyMsg(appKeyMsg("w"))
	if handled {
		t.Fatal("expected app-level w to fall through for workspaces view")
	}
	if updated.view != viewWorkspaces {
		t.Fatalf("expected to remain on workspaces view, got %d", updated.view)
	}
}

func TestHandleKeyMsgEscInEmbeddedWorkspacesReturnsToSessions(t *testing.T) {
	m := New(ModeSessions)
	m.view = viewWorkspaces

	updated, cmd, handled := m.handleKeyMsg(appKeyMsg("esc"))
	if !handled {
		t.Fatal("expected esc to be handled in embedded workspaces view")
	}
	if cmd != nil {
		t.Fatal("expected no quit command in embedded workspaces mode")
	}
	if updated.view != viewSessions {
		t.Fatalf("expected esc to return to sessions view, got %d", updated.view)
	}
}

func TestHandleKeyMsgEscInStandaloneWorkspacesUsesViewCommand(t *testing.T) {
	m := New(ModeWorkspaces)

	_, cmd, handled := m.handleKeyMsg(appKeyMsg("esc"))
	if !handled {
		t.Fatal("expected esc to be handled in standalone workspaces mode")
	}
	if cmd == nil {
		t.Fatal("expected workspaces view command (quit) to be returned")
	}
}
