package worktree

import (
	"regexp"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9-]+`)

// GenerateBranchName converts a human description into a branch name.
//
// It detects intent from leading words:
//
//	"fix login redirect bug"       → fix/login-redirect-bug
//	"add user avatars"             → feat/user-avatars
//	"refactor auth middleware"     → refactor/auth-middleware
//	"update payment flow"          → feat/update-payment-flow
//	"something random"             → wip/something-random
func GenerateBranchName(description string) string {
	desc := strings.TrimSpace(strings.ToLower(description))
	if desc == "" {
		return ""
	}

	words := strings.Fields(desc)
	prefix := "wip"
	body := words

	if len(words) > 1 {
		switch words[0] {
		case "fix", "bugfix", "hotfix":
			prefix = "fix"
			body = words[1:]
		case "add", "feat", "feature", "implement":
			prefix = "feat"
			body = words[1:]
		case "refactor", "cleanup", "clean":
			prefix = "refactor"
			body = words[1:]
		case "update", "improve", "enhance":
			prefix = "feat"
			body = words // keep the verb
		case "test", "tests":
			prefix = "test"
			body = words[1:]
		case "docs", "doc", "document":
			prefix = "docs"
			body = words[1:]
		case "chore":
			prefix = "chore"
			body = words[1:]
		}
	}

	slug := nonSlug.ReplaceAllString(strings.Join(body, "-"), "")
	slug = strings.Trim(slug, "-")
	// Collapse multiple dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	if slug == "" {
		return prefix + "/branch"
	}
	return prefix + "/" + slug
}
