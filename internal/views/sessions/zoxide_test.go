package sessions

import (
	"testing"
)

func TestResolveWorkspace_nonGitDir(t *testing.T) {
	name, dir := resolveWorkspace("/tmp")
	if name != "tmp" {
		t.Errorf("expected name 'tmp', got %q", name)
	}
	if dir != "/tmp" {
		t.Errorf("expected dir '/tmp', got %q", dir)
	}
}

func TestResolveWorkspace_nonExistentDir(t *testing.T) {
	name, dir := resolveWorkspace("/nonexistent/path/myworkspace")
	if name != "myworkspace" {
		t.Errorf("expected name 'myworkspace', got %q", name)
	}
	if dir != "/nonexistent/path/myworkspace" {
		t.Errorf("expected dir unchanged, got %q", dir)
	}
}
