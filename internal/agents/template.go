package agents

import (
	"fmt"
	"strings"
)

func RenderPromptTemplate(template, prompt string) (string, error) {
	if !strings.Contains(template, "{prompt}") {
		return "", fmt.Errorf("template must include {prompt}")
	}
	quoted := shellQuote(prompt)
	return strings.ReplaceAll(template, "{prompt}", quoted), nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
