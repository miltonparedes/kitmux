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

	defaultAgentWorkbench         = "auto"
	defaultAgentWorkbenchMinWidth = 160
	defaultAgentWorkbenchRatio    = 30
	defaultWorkbenchCommand       = "kitmux workbench"
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

func AgentWorkbench() string {
	value := strings.ToLower(envOrDefault("KITMUX_AGENT_WORKBENCH", defaultAgentWorkbench))
	switch value {
	case "auto", "always", "off":
		return value
	default:
		return defaultAgentWorkbench
	}
}

func AgentWorkbenchMinWidth() int {
	return envIntOrDefault("KITMUX_AGENT_WORKBENCH_MIN_WIDTH", defaultAgentWorkbenchMinWidth)
}

func AgentWorkbenchRatio() int {
	value := envIntOrDefault("KITMUX_AGENT_WORKBENCH_RATIO", defaultAgentWorkbenchRatio)
	if value < 10 || value > 90 {
		return defaultAgentWorkbenchRatio
	}
	return value
}

func WorkbenchCommand() string {
	return envOrDefault("KITMUX_WORKBENCH_COMMAND", defaultWorkbenchCommand)
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
