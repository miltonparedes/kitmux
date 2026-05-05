package config

import "testing"

func TestAgentWorkbench_DefaultAndValidValues(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "")
	if got := AgentWorkbench(); got != "auto" {
		t.Fatalf("expected default auto, got %q", got)
	}

	t.Setenv("KITMUX_AGENT_WORKBENCH", "always")
	if got := AgentWorkbench(); got != "always" {
		t.Fatalf("expected always, got %q", got)
	}

	t.Setenv("KITMUX_AGENT_WORKBENCH", "off")
	if got := AgentWorkbench(); got != "off" {
		t.Fatalf("expected off, got %q", got)
	}
}

func TestAgentWorkbench_InvalidFallsBackToAuto(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH", "invalid")
	if got := AgentWorkbench(); got != "auto" {
		t.Fatalf("expected auto fallback, got %q", got)
	}
}

func TestAgentWorkbenchNumericConfig(t *testing.T) {
	t.Setenv("KITMUX_AGENT_WORKBENCH_MIN_WIDTH", "")
	if got := AgentWorkbenchMinWidth(); got != 160 {
		t.Fatalf("expected default min width 160, got %d", got)
	}
	t.Setenv("KITMUX_AGENT_WORKBENCH_MIN_WIDTH", "200")
	if got := AgentWorkbenchMinWidth(); got != 200 {
		t.Fatalf("expected min width 200, got %d", got)
	}

	t.Setenv("KITMUX_AGENT_WORKBENCH_RATIO", "")
	if got := AgentWorkbenchRatio(); got != 30 {
		t.Fatalf("expected default ratio 30, got %d", got)
	}
	t.Setenv("KITMUX_AGENT_WORKBENCH_RATIO", "35")
	if got := AgentWorkbenchRatio(); got != 35 {
		t.Fatalf("expected ratio 35, got %d", got)
	}
	t.Setenv("KITMUX_AGENT_WORKBENCH_RATIO", "95")
	if got := AgentWorkbenchRatio(); got != 30 {
		t.Fatalf("expected ratio fallback 30, got %d", got)
	}
}

func TestWorkbenchCommand(t *testing.T) {
	t.Setenv("KITMUX_WORKBENCH_COMMAND", "")
	if got := WorkbenchCommand(); got != "kitmux workbench" {
		t.Fatalf("expected default command, got %q", got)
	}
	t.Setenv("KITMUX_WORKBENCH_COMMAND", "custom workbench")
	if got := WorkbenchCommand(); got != "custom workbench" {
		t.Fatalf("expected custom command, got %q", got)
	}
}
