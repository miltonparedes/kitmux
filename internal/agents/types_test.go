package agents

import "testing"

func TestDefaultAgentsPrioritizesCoreAgentCLIs(t *testing.T) {
	got := DefaultAgents()
	want := []string{"droid", "codex", "cursor"}
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
	a, ok := Find("codex")
	if !ok {
		t.Fatal("expected codex agent")
	}
	mode, ok := FindMode(a, "default")
	if !ok {
		t.Fatal("expected default codex mode")
	}
	if a.FullCommand(mode) != "codex" {
		t.Fatalf("expected codex command, got %q", a.FullCommand(mode))
	}
	if _, ok := Find("gemini"); ok {
		t.Fatal("expected gemini to be unsupported")
	}
	if _, ok := Find("aichat"); ok {
		t.Fatal("expected aichat to be unsupported")
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
	if !IsAgentCommand("cursor-agent") {
		t.Fatal("expected cursor-agent to be detected as an agent command")
	}
	if IsAgentCommand("gemini") {
		t.Fatal("expected gemini to be unsupported")
	}
	if IsAgentCommand("aichat") {
		t.Fatal("expected aichat to be unsupported")
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
	if codex.DisplayName() != "⌾ Codex CLI" {
		t.Fatalf("Codex DisplayName() = %q", codex.DisplayName())
	}

	cursor, ok := Find("cursor")
	if !ok {
		t.Fatal("expected cursor agent")
	}
	if cursor.DisplayName() != "⌬ Cursor CLI" {
		t.Fatalf("Cursor DisplayName() = %q", cursor.DisplayName())
	}
}
