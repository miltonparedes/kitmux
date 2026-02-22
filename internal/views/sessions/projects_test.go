package sessions

import (
	"testing"
)

func TestResolveProject_nonGitDir(t *testing.T) {
	name, dir := resolveProject("/tmp")
	if name != "tmp" {
		t.Errorf("expected name 'tmp', got %q", name)
	}
	if dir != "/tmp" {
		t.Errorf("expected dir '/tmp', got %q", dir)
	}
}

func TestResolveProject_nonExistentDir(t *testing.T) {
	name, dir := resolveProject("/nonexistent/path/myproject")
	if name != "myproject" {
		t.Errorf("expected name 'myproject', got %q", name)
	}
	if dir != "/nonexistent/path/myproject" {
		t.Errorf("expected dir unchanged, got %q", dir)
	}
}
