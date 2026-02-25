package agents

import "testing"

func TestRenderPromptTemplate(t *testing.T) {
	t.Parallel()

	got, err := RenderPromptTemplate("codex {prompt}", "fix login flow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "codex 'fix login flow'"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderPromptTemplateEscapesSingleQuote(t *testing.T) {
	t.Parallel()

	got, err := RenderPromptTemplate("claude {prompt}", "user's issue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "claude 'user'\"'\"'s issue'"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderPromptTemplateRequiresPlaceholder(t *testing.T) {
	t.Parallel()

	_, err := RenderPromptTemplate("codex --help", "x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
