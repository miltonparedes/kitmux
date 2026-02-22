package openlocal

import "testing"

func TestEditorCommand_Zed(t *testing.T) {
	bin, args := EditorCommand(EditorZed, "myhost", "/home/user/project")
	if bin != "zed" {
		t.Fatalf("expected bin=zed, got %s", bin)
	}
	if len(args) != 1 || args[0] != "ssh://myhost//home/user/project" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestEditorCommand_VSCode(t *testing.T) {
	bin, args := EditorCommand(EditorVSCode, "myhost", "/home/user/project")
	if bin != "code" {
		t.Fatalf("expected bin=code, got %s", bin)
	}
	if len(args) != 3 || args[0] != "--remote" || args[1] != "ssh-remote+myhost" || args[2] != "/home/user/project" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestFallbackCommand(t *testing.T) {
	got := FallbackCommand(EditorZed, "myhost", "/path")
	want := "zed ssh://myhost//path"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	got = FallbackCommand(EditorVSCode, "myhost", "/path")
	want = "code --remote ssh-remote+myhost /path"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
