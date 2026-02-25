package config

import (
	"os"
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

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
