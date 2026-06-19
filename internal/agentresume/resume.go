package agentresume

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrUnsupported = errors.New("agent resume unsupported")

type Target struct {
	AgentID           string
	PanePID           int
	ExistingSessionID string
}

func ResolveSessionID(target Target) (string, error) {
	if id := strings.TrimSpace(target.ExistingSessionID); id != "" {
		return id, nil
	}
	if target.PanePID <= 0 {
		return "", fmt.Errorf("invalid pane pid %d", target.PanePID)
	}
	paths, err := lsofPaths(target.PanePID)
	if err != nil {
		return "", err
	}
	return sessionIDFromPaths(target.AgentID, paths)
}

func ResumeCommand(agentID, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", errors.New("empty agent session id")
	}
	switch agentID {
	case "droid":
		return "droid --resume " + shellQuote(sessionID), nil
	case "codex":
		return "codex resume " + shellQuote(sessionID), nil
	case "claude":
		return "claude --resume " + shellQuote(sessionID), nil
	case "cursor":
		return "cursor-agent --resume " + shellQuote(sessionID), nil
	case "opencode":
		return "opencode --session " + shellQuote(sessionID), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupported, agentID)
	}
}

func lsofPaths(pid int) ([]string, error) {
	out, err := exec.Command("lsof", "-Fn", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil, fmt.Errorf("list agent files: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		path := strings.TrimPrefix(line, "n")
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func sessionIDFromPaths(agentID string, paths []string) (string, error) {
	switch agentID {
	case "droid":
		return bestPathID(paths, isDroidSessionPath, sessionIDFromPath)
	case "codex":
		return bestPathID(paths, isCodexRolloutPath, codexThreadIDFromPath)
	case "claude":
		return bestPathID(paths, isClaudeSessionPath, sessionIDFromPath)
	case "cursor":
		return bestPathID(paths, isCursorTranscriptPath, sessionIDFromPath)
	case "opencode":
		return bestPathID(paths, isOpenCodeSessionPath, openCodeSessionIDFromPath)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupported, agentID)
	}
}

type (
	pathPredicate func(string) bool
	pathIDFunc    func(string) string
)

func bestPathID(paths []string, match pathPredicate, extract pathIDFunc) (string, error) {
	var bestPath string
	var bestMod time.Time
	for _, path := range paths {
		if !match(path) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if bestPath == "" || info.ModTime().After(bestMod) {
			bestPath = path
			bestMod = info.ModTime()
		}
	}
	if bestPath == "" {
		return "", errors.New("agent session file not found for process")
	}
	id := extract(bestPath)
	if id == "" {
		return "", fmt.Errorf("agent session id not found in path: %s", bestPath)
	}
	return id, nil
}

func isDroidSessionPath(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(path, string(filepath.Separator)+".factory"+string(filepath.Separator)+"sessions"+string(filepath.Separator)) &&
		(strings.HasSuffix(base, ".jsonl") || strings.HasSuffix(base, ".settings.json"))
}

func isCodexRolloutPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "rollout-") && strings.HasSuffix(base, ".jsonl") &&
		strings.Contains(path, string(filepath.Separator)+".codex"+string(filepath.Separator)+"sessions"+string(filepath.Separator))
}

func isClaudeSessionPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".jsonl") &&
		strings.Contains(path, string(filepath.Separator)+".claude"+string(filepath.Separator)+"projects"+string(filepath.Separator))
}

func isCursorTranscriptPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".jsonl") &&
		strings.Contains(path, string(filepath.Separator)+".cursor"+string(filepath.Separator)) &&
		strings.Contains(path, string(filepath.Separator)+"agent-transcripts"+string(filepath.Separator))
}

func isOpenCodeSessionPath(path string) bool {
	base := filepath.Base(path)
	sessionDir := strings.Join([]string{
		".local",
		"share",
		"opencode",
		"storage",
		"session",
	}, string(filepath.Separator))
	return strings.HasPrefix(base, "ses_") && strings.HasSuffix(base, ".json") &&
		strings.Contains(path, string(filepath.Separator)+sessionDir+string(filepath.Separator))
}

var uuidPattern = regexp.MustCompile(
	`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`,
)
var opencodeSessionPattern = regexp.MustCompile(`ses_[A-Za-z0-9]+`)

func sessionIDFromPath(path string) string {
	return uuidPattern.FindString(filepath.Base(path))
}

func codexThreadIDFromPath(path string) string {
	matches := uuidPattern.FindAllString(filepath.Base(path), -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func openCodeSessionIDFromPath(path string) string {
	return opencodeSessionPattern.FindString(filepath.Base(path))
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
