package worktree

import "testing"

func TestABBranchName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		base  string
		agent string
		want  string
	}{
		{name: "main codex", base: "main", agent: "codex", want: "ab/main-codex"},
		{name: "feature claude", base: "feature/refactor", agent: "claude", want: "ab/feature/refactor-claude"},
		{name: "default base", base: " ", agent: "codex", want: "ab/main-codex"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := abBranchName(tc.base, tc.agent)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
