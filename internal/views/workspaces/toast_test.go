package workspaces

import (
	"strings"
	"testing"
)

func TestToastMsgRendersInFooter(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(toastMsg{text: "boom", level: toastError})
	m = updated.(Model)

	if m.toast != "boom" {
		t.Fatalf("expected toast text 'boom', got %q", m.toast)
	}

	out := m.footer()
	if !strings.Contains(out, "boom") {
		t.Errorf("expected toast in footer, got %q", out)
	}
}

func TestToastClearMsgClearsCurrentToast(t *testing.T) {
	m := newSeededModel()
	updated, _ := m.Update(toastMsg{text: "first", level: toastInfo})
	m = updated.(Model)
	firstSeq := m.toastSeq

	// A stale clear for a later toast should not clear current.
	updated, _ = m.Update(toastClearMsg{seq: firstSeq + 5})
	m = updated.(Model)
	if m.toast == "" {
		t.Fatal("stale clear should not wipe current toast")
	}

	// The matching sequence clears it.
	updated, _ = m.Update(toastClearMsg{seq: firstSeq})
	m = updated.(Model)
	if m.toast != "" {
		t.Errorf("expected toast cleared, got %q", m.toast)
	}
}
