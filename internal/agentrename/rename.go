package agentrename

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // register sqlite database driver
)

var ErrUnsupported = errors.New("agent rename unsupported")

type Target struct {
	AgentID string
	PanePID int
}

func Rename(target Target, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	switch target.AgentID {
	case "codex":
		return renameCodex(target.PanePID, title)
	default:
		return ErrUnsupported
	}
}

func Title(target Target) (string, error) {
	switch target.AgentID {
	case "codex":
		return codexThreadTitleFromPID(target.PanePID)
	default:
		return "", ErrUnsupported
	}
}

func renameCodex(pid int, title string) error {
	threadID, err := codexThreadIDFromPID(pid)
	if err != nil {
		return err
	}
	return setCodexThreadName(threadID, title)
}

func codexThreadIDFromPID(pid int) (string, error) {
	thread, err := codexThreadFromPID(pid)
	if err != nil {
		return "", err
	}
	return thread.ID, nil
}

func codexThreadTitleFromPID(pid int) (string, error) {
	thread, err := codexThreadFromPID(pid)
	if err != nil {
		return "", err
	}
	if thread.StateDBPath == "" {
		return "", errors.New("codex state database not found for process")
	}
	return codexThreadTitleFromStateDB(thread.StateDBPath, thread.ID)
}

func codexThreadFromPID(pid int) (codexThread, error) {
	if pid <= 0 {
		return codexThread{}, fmt.Errorf("invalid codex pid %d", pid)
	}
	out, err := exec.Command("lsof", "-Fn", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return codexThread{}, fmt.Errorf("list codex files: %w", err)
	}
	return codexThreadFromLsof(string(out))
}

func codexThreadIDFromLsof(output string) (string, error) {
	thread, err := codexThreadFromLsof(output)
	if err != nil {
		return "", err
	}
	return thread.ID, nil
}

type codexThread struct {
	ID          string
	RolloutPath string
	StateDBPath string
}

func codexThreadFromLsof(output string) (codexThread, error) {
	var bestPath string
	var bestMod time.Time
	var stateDBPath string
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		path := strings.TrimPrefix(line, "n")
		if isCodexStateDBPath(path) && stateDBPath == "" {
			stateDBPath = path
			continue
		}
		if !isCodexRolloutPath(path) {
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
		return codexThread{}, errors.New("codex rollout file not found for process")
	}
	threadID := codexThreadIDFromRolloutPath(bestPath)
	if threadID == "" {
		return codexThread{}, fmt.Errorf("codex rollout path has no thread id: %s", bestPath)
	}
	return codexThread{ID: threadID, RolloutPath: bestPath, StateDBPath: stateDBPath}, nil
}

func isCodexRolloutPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "rollout-") && strings.HasSuffix(base, ".jsonl") &&
		strings.Contains(path, string(filepath.Separator)+".codex"+string(filepath.Separator)+"sessions"+string(filepath.Separator))
}

func isCodexStateDBPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "state_") && strings.HasSuffix(base, ".sqlite") &&
		strings.Contains(path, string(filepath.Separator)+".codex"+string(filepath.Separator))
}

var codexThreadIDPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func codexThreadIDFromRolloutPath(path string) string {
	matches := codexThreadIDPattern.FindAllString(filepath.Base(path), -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func codexThreadTitleFromStateDB(dbPath, threadID string) (string, error) {
	if dbPath == "" {
		return "", errors.New("empty codex state database path")
	}
	if threadID == "" {
		return "", errors.New("empty codex thread id")
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("open codex state database: %w", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	_, _ = db.Exec("PRAGMA busy_timeout = 1000;")
	_, _ = db.Exec("PRAGMA query_only = ON;")

	var title string
	if err := db.QueryRow("SELECT title FROM threads WHERE id = ?", threadID).Scan(&title); err != nil {
		return "", fmt.Errorf("query codex thread title: %w", err)
	}
	return strings.TrimSpace(title), nil
}

func setCodexThreadName(threadID, title string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codex", "app-server")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	send := func(msg map[string]any) error {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := stdin.Write(append(data, '\n')); err != nil {
			return err
		}
		return nil
	}

	if err := send(map[string]any{
		"method": "initialize",
		"id":     1,
		"params": map[string]any{
			"clientInfo": map[string]any{
				"name":    "kitmux",
				"title":   "kitmux",
				"version": "0.0.0",
			},
			"capabilities": nil,
		},
	}); err != nil {
		return err
	}
	if err := send(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
		return err
	}
	if err := send(map[string]any{
		"method": "thread/name/set",
		"id":     2,
		"params": map[string]any{
			"threadId": threadID,
			"name":     title,
		},
	}); err != nil {
		return err
	}
	return waitCodexRenameResponse(stdout, &stderr)
}

func waitCodexRenameResponse(stdout io.Reader, stderr *bytes.Buffer) error {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var msg map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if !isResponseID(msg["id"], 2) {
			continue
		}
		if rawErr, ok := msg["error"]; ok {
			return fmt.Errorf("codex thread rename failed: %v", rawErr)
		}
		return nil
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if stderr.Len() > 0 {
		return fmt.Errorf("codex app-server exited: %s", strings.TrimSpace(stderr.String()))
	}
	return errors.New("codex app-server closed before rename response")
}

func isResponseID(value any, want int) bool {
	switch v := value.(type) {
	case float64:
		return int(v) == want
	case string:
		return v == strconv.Itoa(want)
	default:
		return false
	}
}
