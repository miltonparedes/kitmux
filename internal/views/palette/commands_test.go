package palette

import "testing"

func TestIsValidCommand_Canonical(t *testing.T) {
	if !IsValidCommand("open_workspace") {
		t.Error("expected open_workspace to be a valid command")
	}
	if !IsValidCommand("switch_session") {
		t.Error("expected switch_session to be a valid command")
	}
	if IsValidCommand("nope") {
		t.Error("expected 'nope' to be invalid")
	}
}

func TestDefaultCommands_HasUniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, c := range DefaultCommands() {
		if seen[c.ID] {
			t.Errorf("duplicate command ID %q", c.ID)
		}
		seen[c.ID] = true
	}
}
