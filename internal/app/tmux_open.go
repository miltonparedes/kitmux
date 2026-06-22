package app

import (
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

type openTmuxOps struct {
	SwitchClient func(string) error
	SelectWindow func(string) error
	SelectPane   func(string) error
	Attach       func(string, io.Reader, io.Writer, io.Writer) error
	InTmux       func() bool
}

var tmuxOpenOps = openTmuxOps{
	SwitchClient: tmux.SwitchClient,
	SelectWindow: tmux.SelectWindow,
	SelectPane:   tmux.SelectPane,
	Attach:       attachTmuxSession,
	InTmux:       inTmux,
}

func openTmuxSessionCmd(session string) tea.Cmd {
	return openTmuxTargetCmd(session, session, false)
}

func openTmuxPaneCmd(target string) tea.Cmd {
	return openTmuxTargetCmd(target, sessionFromTarget(target), true)
}

func openTmuxTargetCmd(target, session string, pane bool) tea.Cmd {
	return tea.Exec(&openTmuxTargetCommand{
		target:  target,
		session: session,
		pane:    pane,
	}, func(error) tea.Msg {
		return tea.QuitMsg{}
	})
}

type openTmuxTargetCommand struct {
	target  string
	session string
	pane    bool
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

func (c *openTmuxTargetCommand) SetStdin(r io.Reader) {
	c.stdin = r
}

func (c *openTmuxTargetCommand) SetStdout(w io.Writer) {
	c.stdout = w
}

func (c *openTmuxTargetCommand) SetStderr(w io.Writer) {
	c.stderr = w
}

func (c *openTmuxTargetCommand) Run() error {
	if c.pane {
		_ = tmuxOpenOps.SelectWindow(c.target)
		_ = tmuxOpenOps.SelectPane(c.target)
	}
	if tmuxOpenOps.InTmux() {
		return tmuxOpenOps.SwitchClient(c.session)
	}
	return tmuxOpenOps.Attach(c.session, c.stdin, c.stdout, c.stderr)
}

func sessionFromTarget(target string) string {
	if before, _, ok := strings.Cut(target, ":"); ok {
		return before
	}
	if before, _, ok := strings.Cut(target, "."); ok {
		return before
	}
	return target
}

func attachTmuxSession(session string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command("tmux", "attach-session", "-t", session)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	if stderr == nil {
		stderr = os.Stderr
	}
	cmd.Stderr = stderr
	return cmd.Run()
}

func inTmux() bool {
	return os.Getenv("TMUX") != ""
}
