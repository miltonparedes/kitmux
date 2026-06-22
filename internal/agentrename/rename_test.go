package agentrename

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexThreadIDFromRolloutPath(t *testing.T) {
	path := "/Users/me/.codex/sessions/2026/06/18/rollout-2026-06-18T21-46-00-019eddfc-4d7c-7252-a33b-9a55893dc0f2.jsonl"
	if got := codexThreadIDFromRolloutPath(path); got != "019eddfc-4d7c-7252-a33b-9a55893dc0f2" {
		t.Fatalf("thread id = %q", got)
	}
}

func TestCodexThreadIDFromLsofChoosesNewestOpenRollout(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, ".codex", "sessions", "2026", "06", "18", "rollout-2026-06-18T21-33-10-019eddf0-8d9e-7003-9b8f-98daed09c2dc.jsonl")
	newPath := filepath.Join(root, ".codex", "sessions", "2026", "06", "18", "rollout-2026-06-18T21-46-00-019eddfc-4d7c-7252-a33b-9a55893dc0f2.jsonl")
	for _, path := range []string{oldPath, newPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(oldPath, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	output := "n" + oldPath + "\n" + "n" + newPath + "\n"
	got, err := codexThreadIDFromLsof(output)
	if err != nil {
		t.Fatalf("codexThreadIDFromLsof() error = %v", err)
	}
	if got != "019eddfc-4d7c-7252-a33b-9a55893dc0f2" {
		t.Fatalf("thread id = %q", got)
	}
}

func TestCodexThreadFromLsofFindsStateDBAndNewestOpenRollout(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, ".codex", "state_5.sqlite")
	oldPath := filepath.Join(root, ".codex", "sessions", "2026", "06", "18", "rollout-2026-06-18T21-33-10-019eddf0-8d9e-7003-9b8f-98daed09c2dc.jsonl")
	newPath := filepath.Join(root, ".codex", "sessions", "2026", "06", "18", "rollout-2026-06-18T21-46-00-019eddfc-4d7c-7252-a33b-9a55893dc0f2.jsonl")
	for _, path := range []string{statePath, oldPath, newPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(oldPath, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	output := "n" + statePath + "\n" + "n" + oldPath + "\n" + "n" + newPath + "\n"
	got, err := codexThreadFromLsof(output)
	if err != nil {
		t.Fatalf("codexThreadFromLsof() error = %v", err)
	}
	if got.ID != "019eddfc-4d7c-7252-a33b-9a55893dc0f2" {
		t.Fatalf("thread id = %q", got.ID)
	}
	if got.StateDBPath != statePath {
		t.Fatalf("state db path = %q", got.StateDBPath)
	}
}

func TestCodexThreadTitleFromStateDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state_5.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE threads (id TEXT PRIMARY KEY, title TEXT NOT NULL)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO threads (id, title) VALUES (?, ?)", "019eddfc-4d7c-7252-a33b-9a55893dc0f2", " improve rename "); err != nil {
		t.Fatalf("insert thread: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	got, err := codexThreadTitleFromStateDB(dbPath, "019eddfc-4d7c-7252-a33b-9a55893dc0f2")
	if err != nil {
		t.Fatalf("codexThreadTitleFromStateDB() error = %v", err)
	}
	if got != "improve rename" {
		t.Fatalf("title = %q", got)
	}
}
