package agentthread

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

type Spec struct {
	AgentID string
	ModeID  string
	Dir     string
	Name    string
}

type Resolved struct {
	SessionName string
	Title       string
	Dir         string
	Agent       agents.Agent
	Mode        agents.AgentMode
}

type Ops struct {
	HasSession            func(string) bool
	NewSessionWithCommand func(string, string, string) (string, error)
	SetSessionOption      func(string, string, string) error
	SetWindowOption       func(string, string, string) error
	SetPaneOption         func(string, string, string) error
	SetPaneTitle          func(string, string) error
	SetHook               func(string, string, string) error
	ListThreads           func() ([]tmux.Session, error)
	Attach                func(string) error
}

func DefaultOps() Ops {
	return Ops{
		HasSession:            tmux.HasSession,
		NewSessionWithCommand: tmux.NewSessionWithCommand,
		SetSessionOption:      tmux.SetSessionOption,
		SetWindowOption:       tmux.SetWindowOption,
		SetPaneOption:         tmux.SetPaneOption,
		SetPaneTitle:          tmux.SetPaneTitle,
		SetHook:               tmux.SetHook,
		ListThreads:           tmux.ListThreads,
		Attach:                Attach,
	}
}

func Resolve(spec Spec) (Resolved, error) {
	agent, ok := agents.Find(spec.AgentID)
	if !ok {
		return Resolved{}, fmt.Errorf("unknown agent %q", spec.AgentID)
	}

	modeID := strings.TrimSpace(spec.ModeID)
	if modeID == "" {
		modeID = "default"
	}
	mode, ok := agents.FindMode(agent, modeID)
	if !ok {
		return Resolved{}, fmt.Errorf("unknown mode %q for agent %q", modeID, spec.AgentID)
	}

	dir := strings.TrimSpace(spec.Dir)
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Resolved{}, fmt.Errorf("current directory: %w", err)
		}
		dir = cwd
	}

	project := filepath.Base(filepath.Clean(dir))
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = "workspace"
	}

	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = spec.AgentID + "-" + project
	}

	return Resolved{
		SessionName: sanitizeSessionName(name),
		Title:       agent.DisplayName() + " · " + project,
		Dir:         dir,
		Agent:       agent,
		Mode:        mode,
	}, nil
}

func EnsureAndAttach(spec Spec, ops Ops) error {
	resolved, err := Ensure(spec, ops)
	if err != nil {
		return err
	}
	return ops.withDefaults().Attach(resolved.SessionName)
}

func Ensure(spec Spec, ops Ops) (Resolved, error) {
	resolved, err := Resolve(spec)
	if err != nil {
		return Resolved{}, err
	}

	ops = ops.withDefaults()
	targetPane := resolved.SessionName
	created := false
	if !ops.HasSession(resolved.SessionName) {
		paneID, err := ops.NewSessionWithCommand(
			resolved.SessionName,
			resolved.Dir,
			resolved.Agent.FullCommand(resolved.Mode),
		)
		if err != nil {
			return Resolved{}, err
		}
		targetPane = paneID
		created = true
	}
	if err := ApplySupport(SupportSpec{
		SessionName:  resolved.SessionName,
		TargetPane:   targetPane,
		AgentID:      resolved.Agent.ID,
		InitialTitle: resolved.Title,
		Created:      created,
	}, ops); err != nil {
		return Resolved{}, err
	}
	return resolved, nil
}

type SupportSpec struct {
	SessionName  string
	TargetPane   string
	AgentID      string
	InitialTitle string
	Created      bool
}

func InstallAllSupport(ops Ops) (int, error) {
	ops = ops.withDefaults()
	threads, err := ops.ListThreads()
	if err != nil {
		return 0, err
	}
	for _, thread := range threads {
		if err := InstallSupportForSession(thread, ops); err != nil {
			return 0, err
		}
	}
	return len(threads), nil
}

func InstallSupportForSession(session tmux.Session, ops Ops) error {
	title := initialTitle(session)
	return ApplySupport(SupportSpec{
		SessionName:  session.Name,
		TargetPane:   session.Name,
		AgentID:      session.AgentID,
		InitialTitle: title,
	}, ops)
}

func ApplySupport(spec SupportSpec, ops Ops) error {
	ops = ops.withDefaults()
	if spec.TargetPane == "" {
		spec.TargetPane = spec.SessionName
	}
	if spec.InitialTitle == "" {
		spec.InitialTitle = initialTitle(tmux.Session{
			Name:    spec.SessionName,
			Path:    spec.SessionName,
			AgentID: spec.AgentID,
		})
	}
	for _, opt := range []struct {
		name  string
		value string
	}{
		{"status", "off"},
		{"set-titles", "on"},
		{"set-titles-string", threadTitleFormat()},
		{"allow-set-title", "on"},
		{"monitor-bell", "on"},
		{"bell-action", "any"},
		{"visual-bell", "off"},
		{"@kitmux_thread", "1"},
		{"@kitmux_agent", spec.AgentID},
		{"@kitmux_agent_support", supportVersion},
		{"@kitmux_initial_title", spec.InitialTitle},
	} {
		if err := ops.SetSessionOption(spec.SessionName, opt.name, opt.value); err != nil {
			return fmt.Errorf("set session option %s: %w", opt.name, err)
		}
	}
	if err := ops.SetWindowOption(spec.TargetPane, "allow-passthrough", "on"); err != nil {
		return fmt.Errorf("set allow-passthrough: %w", err)
	}
	for _, hook := range threadHooks() {
		if err := ops.SetHook(spec.SessionName, hook.name, hook.command); err != nil {
			return fmt.Errorf("set hook %s: %w", hook.name, err)
		}
	}
	if !spec.Created {
		return nil
	}
	if err := ops.SetPaneTitle(spec.TargetPane, spec.InitialTitle); err != nil {
		return fmt.Errorf("set pane title: %w", err)
	}
	return nil
}

func initialTitle(session tmux.Session) string {
	agentName := agentName(session.AgentID)
	project := filepath.Base(filepath.Clean(session.Path))
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = session.Name
	}
	return agentName + " · " + project
}

func agentName(agentID string) string {
	if agent, ok := agents.Find(agentID); ok {
		return agent.DisplayName()
	}
	if agentID == "" {
		return "Agent"
	}
	return agentID
}

func threadTitleFormat() string {
	title := "#{?#{==:#{pane_title},#{host_short}},#{pane_current_command},#{pane_title}}"
	return title + " - #{session_name}"
}

const supportVersion = "1"

type threadHook struct {
	name    string
	command string
}

func threadHooks() []threadHook {
	return []threadHook{
		{
			name:    "client-attached",
			command: `refresh-client -t "#{hook_client}"`,
		},
		{
			name:    "client-session-changed",
			command: `refresh-client -t "#{hook_client}"`,
		},
		{
			name:    "alert-bell",
			command: alertBellHookCommand(),
		},
	}
}

func alertBellHookCommand() string {
	return `run-shell -b 'tmux -S "#{socket_path}" list-clients -t "#{hook_session}" ` +
		`-F "#{client_tty}" | while IFS= read -r tty; do test -n "$tty" && ` +
		`printf "\007" > "$tty"; done'`
}

func Attach(sessionName string) error {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	return syscall.Exec(tmuxPath, []string{"tmux", "attach-session", "-t", sessionName}, os.Environ())
}

func (ops Ops) withDefaults() Ops {
	defaults := DefaultOps()
	if ops.HasSession == nil {
		ops.HasSession = defaults.HasSession
	}
	if ops.NewSessionWithCommand == nil {
		ops.NewSessionWithCommand = defaults.NewSessionWithCommand
	}
	if ops.SetSessionOption == nil {
		ops.SetSessionOption = defaults.SetSessionOption
	}
	if ops.SetWindowOption == nil {
		ops.SetWindowOption = defaults.SetWindowOption
	}
	if ops.SetPaneOption == nil {
		ops.SetPaneOption = defaults.SetPaneOption
	}
	if ops.SetPaneTitle == nil {
		ops.SetPaneTitle = defaults.SetPaneTitle
	}
	if ops.SetHook == nil {
		ops.SetHook = defaults.SetHook
	}
	if ops.ListThreads == nil {
		ops.ListThreads = defaults.ListThreads
	}
	if ops.Attach == nil {
		ops.Attach = defaults.Attach
	}
	return ops
}

var unsafeSessionChars = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func sanitizeSessionName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = unsafeSessionChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "agent-thread"
	}
	return name
}
