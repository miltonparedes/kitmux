package agenttrack

import "testing"

func TestRegisterStoresPIDContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := Register(12345, Context{
		AgentID:     "droid",
		SessionName: "droid-app",
		PaneID:      "%1",
		Thread:      true,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := readPID(12345)
	if !ok {
		t.Fatal("registered pid was not readable")
	}
	if got.AgentID != "droid" || got.SessionName != "droid-app" || got.PaneID != "%1" || !got.Thread {
		t.Fatalf("context = %#v", got)
	}
}
