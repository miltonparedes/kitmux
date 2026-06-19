package agentresume

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResumeCommand(t *testing.T) {
	tests := []struct {
		agent string
		id    string
		want  string
	}{
		{"droid", "11111111-1111-4111-8111-111111111111", "droid --resume '11111111-1111-4111-8111-111111111111'"},
		{"codex", "22222222-2222-4222-8222-222222222222", "codex resume '22222222-2222-4222-8222-222222222222'"},
		{"claude", "33333333-3333-4333-8333-333333333333", "claude --resume '33333333-3333-4333-8333-333333333333'"},
		{"cursor", "44444444-4444-4444-8444-444444444444", "cursor-agent --resume '44444444-4444-4444-8444-444444444444'"},
		{"opencode", "ses_abc123", "opencode --session 'ses_abc123'"},
	}
	for _, tt := range tests {
		got, err := ResumeCommand(tt.agent, tt.id)
		if err != nil {
			t.Fatalf("ResumeCommand(%q) error = %v", tt.agent, err)
		}
		if got != tt.want {
			t.Fatalf("ResumeCommand(%q) = %q, want %q", tt.agent, got, tt.want)
		}
	}
}

func TestResumeCommandUnsupported(t *testing.T) {
	_, err := ResumeCommand("unsupported", "id")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestSessionIDFromPaths(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	tests := []struct {
		agent string
		path  string
		want  string
	}{
		{
			agent: "droid",
			path:  filepath.Join(root, ".factory", "sessions", "-tmp-app", "11111111-1111-4111-8111-111111111111.jsonl"),
			want:  "11111111-1111-4111-8111-111111111111",
		},
		{
			agent: "codex",
			path:  filepath.Join(root, ".codex", "sessions", "2026", "06", "18", "rollout-2026-06-18T21-46-00-22222222-2222-4222-8222-222222222222.jsonl"),
			want:  "22222222-2222-4222-8222-222222222222",
		},
		{
			agent: "claude",
			path:  filepath.Join(root, ".claude", "projects", "-tmp-app", "33333333-3333-4333-8333-333333333333.jsonl"),
			want:  "33333333-3333-4333-8333-333333333333",
		},
		{
			agent: "cursor",
			path:  filepath.Join(root, ".cursor", "projects", "tmp-app", "agent-transcripts", "44444444-4444-4444-8444-444444444444", "44444444-4444-4444-8444-444444444444.jsonl"),
			want:  "44444444-4444-4444-8444-444444444444",
		},
		{
			agent: "opencode",
			path:  filepath.Join(root, ".local", "share", "opencode", "storage", "session", "project", "ses_abc123.json"),
			want:  "ses_abc123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			writeTestFile(t, tt.path, now)
			got, err := sessionIDFromPaths(tt.agent, []string{tt.path})
			if err != nil {
				t.Fatalf("sessionIDFromPaths() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("sessionIDFromPaths() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSessionIDFromPathsPicksNewestMatchingFile(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, ".factory", "sessions", "-tmp-app", "11111111-1111-4111-8111-111111111111.jsonl")
	newPath := filepath.Join(root, ".factory", "sessions", "-tmp-app", "22222222-2222-4222-8222-222222222222.jsonl")
	writeTestFile(t, oldPath, time.Now().Add(-time.Hour))
	writeTestFile(t, newPath, time.Now())

	got, err := sessionIDFromPaths("droid", []string{oldPath, newPath})
	if err != nil {
		t.Fatalf("sessionIDFromPaths() error = %v", err)
	}
	if got != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("sessionIDFromPaths() = %q", got)
	}
}

func writeTestFile(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set times: %v", err)
	}
}
