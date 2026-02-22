package palette

import (
	"testing"
)

func TestEnsureVisible_scrollsDown(t *testing.T) {
	m := New()
	m.height = 7 // avail=4, maxVisible=2 (2 items fit in 3 lines)
	m.cursor = 5
	m.scroll = 0
	m.ensureVisible()
	if m.scroll == 0 {
		t.Error("expected scroll to advance when cursor is past visible window")
	}
	visible := m.maxVisible()
	if m.cursor < m.scroll || m.cursor >= m.scroll+visible {
		t.Errorf("cursor %d should be in [%d, %d)", m.cursor, m.scroll, m.scroll+visible)
	}
}

func TestEnsureVisible_scrollsUp(t *testing.T) {
	m := New()
	m.height = 7
	m.cursor = 0
	m.scroll = 3
	m.ensureVisible()
	if m.scroll != 0 {
		t.Errorf("expected scroll=0 when cursor is at top, got %d", m.scroll)
	}
}

func TestReset_clearsScroll(t *testing.T) {
	m := New()
	m.scroll = 5
	m.cursor = 5
	m.Reset()
	if m.scroll != 0 {
		t.Errorf("expected scroll=0 after Reset, got %d", m.scroll)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after Reset, got %d", m.cursor)
	}
}
