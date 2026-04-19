package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ListSessions returns all tmux sessions.
func ListSessions() ([]Session, error) {
	out, err := exec.Command("tmux", "list-sessions", "-F",
		"#{session_name}\t#{session_windows}\t#{?session_attached,1,0}\t#{session_path}\t#{session_activity}").Output()
	if err != nil {
		return nil, fmt.Errorf("list-sessions: %w", err)
	}
	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 3 {
			continue
		}
		wins, _ := strconv.Atoi(parts[1])
		var path string
		if len(parts) >= 4 {
			path = parts[3]
		}
		var activity int64
		if len(parts) >= 5 {
			activity, _ = strconv.ParseInt(parts[4], 10, 64)
		}
		sessions = append(sessions, Session{
			Name:     parts[0],
			Windows:  wins,
			Attached: parts[2] == "1",
			Path:     path,
			Activity: activity,
		})
	}
	return sessions, nil
}

// ListWindows returns windows for a given session.
func ListWindows(session string) ([]Window, error) {
	out, err := exec.Command("tmux", "list-windows", "-t", session, "-F",
		"#{window_index}\t#{window_name}\t#{?window_active,1,0}").Output()
	if err != nil {
		return nil, fmt.Errorf("list-windows: %w", err)
	}
	var windows []Window
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		idx, _ := strconv.Atoi(parts[0])
		windows = append(windows, Window{
			SessionName: session,
			Index:       idx,
			Name:        parts[1],
			Active:      parts[2] == "1",
		})
	}
	return windows, nil
}

// CurrentSession returns the name of the current tmux session.
func CurrentSession() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return "", fmt.Errorf("display-message: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func CurrentPanePath() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_current_path}").Output()
	if err != nil {
		return "", fmt.Errorf("display-message pane path: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// HasSession returns true if a session with the given name exists.
func HasSession(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// SwitchClient switches the current tmux client to the given target.
func SwitchClient(target string) error {
	return exec.Command("tmux", "switch-client", "-t", target).Run()
}

// KillSession kills the named session.
func KillSession(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

// RenameSession renames a session from old to newName.
func RenameSession(old, newName string) error {
	return exec.Command("tmux", "rename-session", "-t", old, newName).Run()
}

// RenameWindow renames a window given a tmux target (e.g. "session:0").
func RenameWindow(target, newName string) error {
	return exec.Command("tmux", "rename-window", "-t", target, newName).Run()
}

// NewSessionInDir creates a detached session with the given name and working directory.
func NewSessionInDir(name, dir string) error {
	return exec.Command("tmux", "new-session", "-d", "-s", name, "-c", dir).Run()
}

// NewSessionDetached creates a detached session with the given name.
func NewSessionDetached(name string) error {
	return exec.Command("tmux", "new-session", "-d", "-s", name).Run()
}

// SendKeys sends keystrokes to a tmux target pane.
func SendKeys(target, keys string) error {
	return exec.Command("tmux", "send-keys", "-t", target, keys, "Enter").Run()
}

// SplitWindow creates a horizontal split running the given command.
func SplitWindow(command string) error {
	return exec.Command("tmux", "split-window", "-h", command).Run()
}

// NewWindowWithCommand creates a new window running the given command.
func NewWindowWithCommand(name, command string) error {
	return exec.Command("tmux", "new-window", "-n", name, command).Run()
}

func NewWindowInDir(name, dir, command string) (string, error) {
	out, err := exec.Command("tmux", "new-window",
		"-P", "-F", "#{pane_id}",
		"-n", name,
		"-c", dir,
		command,
	).Output()
	if err != nil {
		return "", fmt.Errorf("new-window: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// NewWindowInSession creates a new window inside an existing session,
// optionally starting in `dir` and running `command`. It does not make the
// new window active — callers that want focus should switch-client afterwards.
func NewWindowInSession(session, name, dir, command string) error {
	args := []string{"new-window", "-d", "-t", session + ":"}
	if name != "" {
		args = append(args, "-n", name)
	}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	if command != "" {
		args = append(args, command)
	}
	return exec.Command("tmux", args...).Run()
}

func SplitWindowInDir(targetPane, dir, command string) (string, error) {
	args := []string{
		"split-window", "-h",
		"-P", "-F", "#{pane_id}",
	}
	if targetPane != "" {
		args = append(args, "-t", targetPane)
	}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	args = append(args, command)

	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("split-window: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func SelectLayout(target, layout string) error {
	return exec.Command("tmux", "select-layout", "-t", target, layout).Run()
}

// ListPanes returns all panes across all sessions with their running commands.
func ListPanes() ([]Pane, error) {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_current_command}\t#{pane_pid}\t#{pane_current_path}").Output()
	if err != nil {
		return nil, fmt.Errorf("list-panes: %w", err)
	}
	var panes []Pane
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 6)
		if len(parts) < 4 {
			continue
		}
		winIdx, _ := strconv.Atoi(parts[1])
		paneIdx, _ := strconv.Atoi(parts[2])
		var pid int
		if len(parts) >= 5 {
			pid, _ = strconv.Atoi(parts[4])
		}
		var path string
		if len(parts) >= 6 {
			path = parts[5]
		}
		panes = append(panes, Pane{
			SessionName: parts[0],
			WindowIndex: winIdx,
			PaneIndex:   paneIdx,
			Command:     parts[3],
			PID:         pid,
			Path:        path,
		})
	}
	return panes, nil
}

// DisplayPopup opens a tmux popup running the given command.
func DisplayPopup(command, width, height string) error {
	return exec.Command("tmux", "display-popup",
		"-d", "#{pane_current_path}",
		"-w", width, "-h", height, "-E", command).Run()
}

// DisplayMessage shows a transient message in the tmux status area.
func DisplayMessage(message string) error {
	return exec.Command("tmux", "display-message", message).Run()
}
