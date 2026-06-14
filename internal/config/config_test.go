package config

import "testing"

func TestAgentSidepanel_DefaultAndValidValues(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "")
	if got := AgentSidepanel(); got != "auto" {
		t.Fatalf("expected default auto, got %q", got)
	}

	t.Setenv("KITMUX_AGENT_SIDEPANEL", "always")
	if got := AgentSidepanel(); got != "always" {
		t.Fatalf("expected always, got %q", got)
	}

	t.Setenv("KITMUX_AGENT_SIDEPANEL", "off")
	if got := AgentSidepanel(); got != "off" {
		t.Fatalf("expected off, got %q", got)
	}
}

func TestAgentSidepanel_InvalidFallsBackToAuto(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL", "invalid")
	if got := AgentSidepanel(); got != "auto" {
		t.Fatalf("expected auto fallback, got %q", got)
	}
}

func TestAgentSidepanelNumericConfig(t *testing.T) {
	t.Setenv("KITMUX_AGENT_SIDEPANEL_MIN_WIDTH", "")
	if got := AgentSidepanelMinWidth(); got != 160 {
		t.Fatalf("expected default min width 160, got %d", got)
	}
	t.Setenv("KITMUX_AGENT_SIDEPANEL_MIN_WIDTH", "200")
	if got := AgentSidepanelMinWidth(); got != 200 {
		t.Fatalf("expected min width 200, got %d", got)
	}

	t.Setenv("KITMUX_AGENT_SIDEPANEL_RATIO", "")
	if got := AgentSidepanelRatio(); got != 30 {
		t.Fatalf("expected default ratio 30, got %d", got)
	}
	t.Setenv("KITMUX_AGENT_SIDEPANEL_RATIO", "35")
	if got := AgentSidepanelRatio(); got != 35 {
		t.Fatalf("expected ratio 35, got %d", got)
	}
	t.Setenv("KITMUX_AGENT_SIDEPANEL_RATIO", "95")
	if got := AgentSidepanelRatio(); got != 30 {
		t.Fatalf("expected ratio fallback 30, got %d", got)
	}
}

func TestSidepanelCommand(t *testing.T) {
	t.Setenv("KITMUX_SIDEPANEL_COMMAND", "")
	if got := SidepanelCommand(); got != "kitmux sidepanel" {
		t.Fatalf("expected default command, got %q", got)
	}
	t.Setenv("KITMUX_SIDEPANEL_COMMAND", "custom sidepanel")
	if got := SidepanelCommand(); got != "custom sidepanel" {
		t.Fatalf("expected custom command, got %q", got)
	}
}
