package config

import (
	"os"
	"strconv"
	"strings"
)

// SuperKey controls the modifier for digit quick-select shortcuts.
// "alt" = require Alt+digit, "none" = bare digit.
var SuperKey = "none"

const (
	defaultABCodexTemplate  = "codex {prompt}"
	defaultABClaudeTemplate = "claude {prompt}"
	defaultABPlanPrefix     = "/plan "
	defaultABBaseBranch     = "main"

	defaultAgentSidepanel         = "auto"
	defaultAgentSidepanelMinWidth = 160
	defaultAgentSidepanelRatio    = 30
	defaultSidepanelCommand       = "kitmux sidepanel"
)

func ABCodexTemplate() string {
	return envOrDefault("KITMUX_AB_CODEX_TEMPLATE", defaultABCodexTemplate)
}

func ABClaudeTemplate() string {
	return envOrDefault("KITMUX_AB_CLAUDE_TEMPLATE", defaultABClaudeTemplate)
}

func ABPlanPrefix() string {
	return envOrDefault("KITMUX_AB_PLAN_PREFIX", defaultABPlanPrefix)
}

func ABBaseBranch() string {
	return envOrDefault("KITMUX_AB_BASE_BRANCH", defaultABBaseBranch)
}

func AgentSidepanel() string {
	value := strings.ToLower(envOrDefault("KITMUX_AGENT_SIDEPANEL", defaultAgentSidepanel))
	switch value {
	case "auto", "always", "off":
		return value
	default:
		return defaultAgentSidepanel
	}
}

func AgentSidepanelMinWidth() int {
	return envIntOrDefault("KITMUX_AGENT_SIDEPANEL_MIN_WIDTH", defaultAgentSidepanelMinWidth)
}

func AgentSidepanelRatio() int {
	value := envIntOrDefault("KITMUX_AGENT_SIDEPANEL_RATIO", defaultAgentSidepanelRatio)
	if value < 10 || value > 90 {
		return defaultAgentSidepanelRatio
	}
	return value
}

func SidepanelCommand() string {
	return envOrDefault("KITMUX_SIDEPANEL_COMMAND", defaultSidepanelCommand)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
