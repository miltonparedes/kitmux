package agents

import "testing"

func TestDefaultAgentsPrioritizesDroidCodexCloudCodex(t *testing.T) {
	got := DefaultAgents()
	want := []string{"droid", "codex-cloud", "codex"}
	if len(got) < len(want) {
		t.Fatalf("expected at least %d agents, got %d", len(want), len(got))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("expected agent %d to be %q, got %q", i, id, got[i].ID)
		}
	}
}

func TestFindAndFindMode(t *testing.T) {
	a, ok := Find("codex-cloud")
	if !ok {
		t.Fatal("expected codex-cloud agent")
	}
	mode, ok := FindMode(a, "default")
	if !ok {
		t.Fatal("expected default codex-cloud mode")
	}
	if a.FullCommand(mode) != "codex cloud" {
		t.Fatalf("expected codex cloud command, got %q", a.FullCommand(mode))
	}
}

func TestCommandMapKeepsFirstAgentForDuplicateCommands(t *testing.T) {
	byCommand := CommandMap()
	if byCommand["codex"].ID != "codex" {
		t.Fatalf("expected codex command to detect codex CLI, got %q", byCommand["codex"].ID)
	}
	if !IsAgentCommand("droid") {
		t.Fatal("expected droid to be detected as an agent command")
	}
}

func TestAgentDisplayNameUsesSymbol(t *testing.T) {
	a, ok := Find("droid")
	if !ok {
		t.Fatal("expected droid agent")
	}
	if a.DisplayName() != "⛬ Droid" {
		t.Fatalf("DisplayName() = %q", a.DisplayName())
	}

	claude, ok := Find("claude")
	if !ok {
		t.Fatal("expected claude agent")
	}
	if claude.DisplayName() != "✳ Claude Code" {
		t.Fatalf("Claude DisplayName() = %q", claude.DisplayName())
	}

	codex, ok := Find("codex")
	if !ok {
		t.Fatal("expected codex agent")
	}
	if codex.DisplayName() != "› Codex CLI" {
		t.Fatalf("Codex DisplayName() = %q", codex.DisplayName())
	}
}
