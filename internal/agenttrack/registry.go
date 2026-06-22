package agenttrack

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Context struct {
	AgentID     string `json:"agent_id"`
	SessionName string `json:"session_name"`
	PaneID      string `json:"pane_id"`
	Thread      bool   `json:"thread"`
	UpdatedAt   int64  `json:"updated_at"`
}

func Register(pid int, ctx Context) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	dir, err := registryDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create agent registry dir: %w", err)
	}
	ctx.UpdatedAt = time.Now().Unix()
	data, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("marshal agent context: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, strconv.Itoa(pid)+".json"), data, 0o600)
}

func ResolveAncestor(startPID int) (Context, bool) {
	if startPID <= 0 {
		return Context{}, false
	}
	parents, err := processParents()
	if err != nil {
		return Context{}, false
	}
	for pid := startPID; pid > 0; pid = parents[pid] {
		if ctx, ok := readPID(pid); ok {
			return ctx, true
		}
		if parents[pid] == 0 || parents[pid] == pid {
			break
		}
	}
	return Context{}, false
}

func readPID(pid int) (Context, bool) {
	dir, err := registryDir()
	if err != nil {
		return Context{}, false
	}
	// #nosec G304 -- filename is derived from a validated integer PID inside kitmux's registry dir.
	data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(pid)+".json"))
	if err != nil {
		return Context{}, false
	}
	var ctx Context
	if err := json.Unmarshal(data, &ctx); err != nil {
		return Context{}, false
	}
	return ctx, true
}

func processParents() (map[int]int, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=").Output()
	if err != nil {
		return nil, fmt.Errorf("list process parents: %w", err)
	}
	parents := make(map[int]int)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, pidErr := strconv.Atoi(fields[0])
		ppid, ppidErr := strconv.Atoi(fields[1])
		if pidErr == nil && ppidErr == nil {
			parents[pid] = ppid
		}
	}
	return parents, nil
}

func registryDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "kitmux", "agent-pids"), nil
}
