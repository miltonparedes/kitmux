package agenthooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/agenttrack"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

const (
	agentStateOption        = "@kitmux_agent_state"
	agentEventOption        = "@kitmux_agent_event"
	agentDetailOption       = "@kitmux_agent_detail"
	agentUpdatedOption      = "@kitmux_agent_updated"
	agentTitlePrefixOption  = "@kitmux_agent_title_prefix"
	agentTitleDisplayOption = "@kitmux_agent_title_display"
)

const (
	stateIdle       = "idle"
	stateWorking    = "working"
	stateInput      = "input"
	statePermission = "permission"
	stateError      = "error"
)

var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var resolveAncestorContext = agenttrack.ResolveAncestor

type StateEvent struct {
	State string
	Bell  bool
}

type AgentEvent struct {
	Agent     string
	Event     string
	State     string
	Detail    string
	Bell      bool
	StdinJSON bool
}

type StateOps struct {
	CurrentPaneTitle        func() (string, error)
	SetCurrentPaneOption    func(string, string) error
	SetCurrentSessionOption func(string, string) error
	SetPaneOption           func(string, string, string) error
	SetSessionOption        func(string, string, string) error
	EmitBell                func(io.Writer) error
	StartSpinner            func(SpinnerTarget) error
	RefreshSessionClients   func(string)
	Now                     func() time.Time
}

type SpinnerTarget struct {
	PaneID      string
	SessionName string
	AgentID     string
	Token       string
}

type hookInput struct {
	HookEventName    string         `json:"hook_event_name"`
	ToolName         string         `json:"tool_name"`
	Message          string         `json:"message"`
	Reason           string         `json:"reason"`
	Trigger          string         `json:"trigger"`
	Source           string         `json:"source"`
	EventType        string         `json:"type"`
	NotificationType string         `json:"notification_type"`
	ToolInput        map[string]any `json:"tool_input"`
}

func DefaultStateOps() StateOps {
	return StateOps{
		CurrentPaneTitle:        tmux.CurrentPaneTitle,
		SetCurrentPaneOption:    tmux.SetCurrentPaneOption,
		SetCurrentSessionOption: tmux.SetCurrentSessionOption,
		SetPaneOption:           tmux.SetPaneOption,
		SetSessionOption:        tmux.SetSessionOption,
		EmitBell:                emitBell,
		StartSpinner:            startSpinner,
		RefreshSessionClients:   refreshSessionClients,
		Now:                     time.Now,
	}
}

func RunStateEvent(event StateEvent, out io.Writer, ops StateOps) error {
	return RunAgentEvent(AgentEvent{
		Event: "legacy-state",
		State: event.State,
		Bell:  event.Bell,
	}, nil, out, ops)
}

func RunAgentEvent(event AgentEvent, in io.Reader, out io.Writer, ops StateOps) error {
	ops = ops.withDefaults()

	var input hookInput
	var rawInput []byte
	if event.StdinJSON && in != nil {
		input, rawInput = readHookInputRaw(in)
	}
	ctx := targetContext()
	agentID := firstNonEmpty(event.Agent, ctx.AgentID)
	eventName := sanitizeDetail(firstNonEmpty(event.Event, input.HookEventName, input.EventType))
	state, err := normalizeState(deriveState(event.State, eventName, input))
	if err != nil {
		return err
	}
	bell := deriveBell(event.Bell, eventName)

	logHookEvent(event, eventName, state, ctx, rawInput)

	detail := sanitizeDetail(firstNonEmpty(event.Detail, deriveDetail(input)))
	updated := fmt.Sprintf("%d", ops.Now().UnixMilli())
	prefix := ""
	displayTitle := ""
	if hasTmuxTarget(ctx) {
		paneTitle := currentPaneTitle(ops)
		prefix = titlePrefix(state, agentID, paneTitle)
		if prefix != "" {
			displayTitle = stripLeadingStateGlyph(paneTitle)
		}
	}

	setPaneOptions(ops, ctx.PaneID, state, eventName, detail, updated, prefix, displayTitle)
	if shouldSyncSession(ctx) {
		setSessionOptions(ops, ctx.SessionName, state, eventName, detail, updated, prefix, displayTitle)
		ops.RefreshSessionClients(ctx.SessionName)
		if state == stateWorking && prefix != "" {
			_ = ops.StartSpinner(SpinnerTarget{
				PaneID:      ctx.PaneID,
				SessionName: ctx.SessionName,
				AgentID:     agentID,
				Token:       updated,
			})
		}
	} else if ctx.PaneID != "" && state == stateWorking && prefix != "" {
		_ = ops.StartSpinner(SpinnerTarget{
			PaneID:  ctx.PaneID,
			AgentID: agentID,
			Token:   updated,
		})
	}
	if bell {
		_ = ops.EmitBell(out)
	}
	return nil
}

func targetContext() tmux.ThreadContext {
	ctx := tmux.ThreadContext{}
	ctx.PaneID = os.Getenv("KITMUX_TMUX_PANE")
	if sessionName := os.Getenv("KITMUX_TMUX_SESSION"); sessionName != "" {
		ctx.SessionName = sessionName
	}
	if os.Getenv("KITMUX_TMUX_THREAD") == "1" {
		ctx.Thread = true
	}
	ctx.AgentID = os.Getenv("KITMUX_AGENT_ID")
	if hasTmuxTarget(ctx) {
		return ctx
	}
	if tracked, ok := resolveAncestorContext(os.Getppid()); ok {
		ctx.AgentID = firstNonEmpty(ctx.AgentID, tracked.AgentID)
		ctx.SessionName = tracked.SessionName
		ctx.PaneID = tracked.PaneID
		ctx.Thread = tracked.Thread
	}
	return ctx
}

func shouldSyncSession(ctx tmux.ThreadContext) bool {
	return ctx.SessionName != "" && (ctx.Thread || ctx.PaneID == "")
}

func hasTmuxTarget(ctx tmux.ThreadContext) bool {
	return ctx.PaneID != "" || shouldSyncSession(ctx)
}

func setPaneOptions(ops StateOps, paneID, state, eventName, detail, updated, prefix, displayTitle string) {
	if paneID == "" {
		return
	}
	set := func(option, value string) error {
		return ops.SetPaneOption(paneID, option, value)
	}
	_ = set(agentStateOption, state)
	_ = set(agentEventOption, eventName)
	_ = set(agentDetailOption, detail)
	_ = set(agentUpdatedOption, updated)
	_ = set(agentTitlePrefixOption, prefix)
	_ = set(agentTitleDisplayOption, displayTitle)
}

func setSessionOptions(ops StateOps, sessionName, state, eventName, detail, updated, prefix, displayTitle string) {
	if sessionName == "" {
		return
	}
	set := ops.SetCurrentSessionOption
	if sessionName != "" && ops.SetSessionOption != nil {
		set = func(option, value string) error {
			return ops.SetSessionOption(sessionName, option, value)
		}
	}
	_ = set(agentStateOption, state)
	_ = set(agentEventOption, eventName)
	_ = set(agentDetailOption, detail)
	_ = set(agentUpdatedOption, updated)
	_ = set(agentTitlePrefixOption, prefix)
	_ = set(agentTitleDisplayOption, displayTitle)
}

func readHookInputRaw(in io.Reader) (hookInput, []byte) {
	data, err := io.ReadAll(in)
	if err != nil {
		return hookInput{}, nil
	}
	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return hookInput{}, data
	}
	return input, data
}

func logHookEvent(event AgentEvent, eventName, state string, ctx tmux.ThreadContext, raw []byte) {
	path := os.Getenv("KITMUX_HOOK_LOG")
	if path == "" {
		return
	}
	// #nosec G304 G703 -- path comes from the user-controlled KITMUX_HOOK_LOG debug env var.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	rawLine := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(string(raw))
	if len(rawLine) > 600 {
		rawLine = rawLine[:600]
	}
	_, _ = fmt.Fprintf(f,
		"%s agent=%s flagEvent=%q flagState=%q -> event=%q state=%q pane=%q session=%q thread=%t raw=%s\n",
		time.Now().Format("15:04:05.000"), event.Agent, event.Event, event.State,
		eventName, state, ctx.PaneID, ctx.SessionName, ctx.Thread, rawLine,
	)
}

func deriveState(explicit, eventName string, input hookInput) string {
	key := eventKey(eventName)
	toolKey := eventKey(input.ToolName)
	if key == "elicitation" {
		return stateInput
	}
	// AskUser (droid) / AskUserQuestion (claude) blocks waiting for the user, so the
	// PreToolUse phase is an attention state. PostToolUse means the user answered.
	if strings.HasPrefix(toolKey, "askuser") && !strings.HasPrefix(key, "posttool") {
		return stateInput
	}
	if key == "elicitationresult" {
		return stateWorking
	}
	if key == "permissionrequest" || key == "permissionasked" {
		return statePermission
	}
	if key == "permissiondenied" || key == "sessionerror" {
		return stateError
	}
	if key == "notification" {
		return notificationState(input)
	}
	if explicit != "" {
		return explicit
	}
	switch key {
	case "sessionstart", "stop", "stopfailure", "sessionend", "sessionidle":
		return stateIdle
	case "userpromptsubmit", "pretooluse", "posttooluse", "posttoolusefailure", "posttoolbatch",
		"toolexecutebefore", "toolexecuteafter", "permissionreplied":
		return stateWorking
	default:
		return stateIdle
	}
}

func deriveBell(explicit bool, eventName string) bool {
	if explicit {
		return true
	}
	switch eventKey(eventName) {
	case "notification", "permissionrequest", "permissionasked", "permissiondenied",
		"elicitation", "stop", "stopfailure", "sessionidle", "sessionerror":
		return true
	default:
		return false
	}
}

func notificationState(input hookInput) string {
	notifType := eventKey(input.NotificationType)
	if notifType == "permissionprompt" || notifType == "permissionrequest" ||
		strings.Contains(strings.ToLower(input.Message), statePermission) {
		return statePermission
	}
	return stateInput
}

func deriveDetail(input hookInput) string {
	if input.ToolName != "" {
		return input.ToolName
	}
	if input.Message != "" {
		return input.Message
	}
	if input.Reason != "" {
		return input.Reason
	}
	if input.Trigger != "" {
		return input.Trigger
	}
	if input.Source != "" {
		return input.Source
	}
	if description, ok := input.ToolInput["description"].(string); ok {
		return description
	}
	return ""
}

func normalizeState(state string) (string, error) {
	switch state {
	case stateIdle, stateWorking, stateInput, statePermission, stateError:
		return state, nil
	default:
		return "", fmt.Errorf("invalid agent state %q", state)
	}
}

func eventKey(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeDetail(value string) string {
	value = strings.TrimSpace(strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(value))
	if len(value) > 96 {
		value = value[:96]
	}
	return value
}

func currentPaneTitle(ops StateOps) string {
	if ops.CurrentPaneTitle == nil {
		return ""
	}
	title, err := ops.CurrentPaneTitle()
	if err != nil {
		return ""
	}
	return title
}

func titlePrefix(state, agentID, paneTitle string) string {
	switch state {
	case stateWorking:
		return SpinnerFrames[0]
	case stateInput:
		return "?"
	case statePermission:
		return "!"
	case stateError:
		return "×"
	case "idle":
		symbol := agentSymbol(agentID)
		if symbol == "" || strings.TrimSpace(stripLeadingStateGlyph(paneTitle)) == "" {
			return ""
		}
		return symbol
	default:
		return ""
	}
}

func stripLeadingStateGlyph(title string) string {
	title = strings.TrimLeft(title, " \t")
	for title != "" {
		stripped := false
		for index, r := range title {
			if index != 0 {
				break
			}
			if strings.ContainsRune(statusGlyphs, r) {
				title = strings.TrimLeft(title[len(string(r)):], " \t")
				stripped = true
			}
			break
		}
		if !stripped {
			return title
		}
	}
	return title
}

const statusGlyphs = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⠂⠆⠤⠰⠠⠐⛬✳✻✶✢✤✱›"

func agentSymbol(agentID string) string {
	if agent, ok := agents.Find(agentID); ok {
		if agent.Symbol != "" {
			return agent.Symbol
		}
		return fallbackSymbol(agent.Name)
	}
	return fallbackSymbol(agentID)
}

func fallbackSymbol(value string) string {
	value = strings.TrimSpace(value)
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return strings.ToUpper(string(r))
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (ops StateOps) withDefaults() StateOps {
	defaults := DefaultStateOps()
	if ops.CurrentPaneTitle == nil {
		ops.CurrentPaneTitle = defaults.CurrentPaneTitle
	}
	if ops.SetCurrentPaneOption == nil {
		ops.SetCurrentPaneOption = defaults.SetCurrentPaneOption
	}
	if ops.SetCurrentSessionOption == nil {
		ops.SetCurrentSessionOption = defaults.SetCurrentSessionOption
	}
	if ops.SetPaneOption == nil {
		ops.SetPaneOption = defaults.SetPaneOption
	}
	if ops.SetSessionOption == nil {
		ops.SetSessionOption = defaults.SetSessionOption
	}
	if ops.EmitBell == nil {
		ops.EmitBell = defaults.EmitBell
	}
	if ops.StartSpinner == nil {
		ops.StartSpinner = defaults.StartSpinner
	}
	if ops.RefreshSessionClients == nil {
		ops.RefreshSessionClients = defaults.RefreshSessionClients
	}
	if ops.Now == nil {
		ops.Now = defaults.Now
	}
	return ops
}

func emitBell(out io.Writer) error {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err == nil {
		defer func() { _ = tty.Close() }()
		_, err = fmt.Fprint(tty, "\a")
		return err
	}
	if out == nil {
		return nil
	}
	_, err = fmt.Fprint(out, "\a")
	return err
}

func startSpinner(target SpinnerTarget) error {
	if target.PaneID == "" || target.Token == "" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"hook", "agent-spinner", "--pane", target.PaneID, "--token", target.Token}
	if target.SessionName != "" {
		args = append(args, "--session", target.SessionName)
	}
	if target.AgentID != "" {
		args = append(args, "--agent", target.AgentID)
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func RunSpinner(target SpinnerTarget) error {
	if target.PaneID == "" || target.Token == "" {
		return nil
	}
	ticker := time.NewTicker(140 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(6 * time.Hour)
	frame := 0
	for {
		select {
		case <-deadline:
			restoreStaticPrefix(target)
			return nil
		case <-ticker.C:
			state, token, err := readSpinnerState(target.PaneID)
			if err != nil {
				return nil
			}
			if state != stateWorking {
				restoreStaticPrefix(target)
				return nil
			}
			if token != target.Token {
				return nil
			}
			frame++
			setSpinnerPrefix(target.PaneID, target.SessionName, SpinnerFrames[frame%len(SpinnerFrames)])
		}
	}
}

func restoreStaticPrefix(target SpinnerTarget) {
	state, _, err := readSpinnerState(target.PaneID)
	if err != nil || state == stateWorking {
		return
	}
	paneTitle := ""
	if out, err := exec.Command("tmux", "display-message", "-p", "-t", target.PaneID, "#{pane_title}").Output(); err == nil {
		paneTitle = strings.TrimSpace(string(out))
	}
	setSpinnerPrefix(target.PaneID, target.SessionName, titlePrefix(state, target.AgentID, paneTitle))
}

func readSpinnerState(paneID string) (string, string, error) {
	format := fmt.Sprintf("#{%s}\t#{%s}", agentStateOption, agentUpdatedOption)
	out, err := exec.Command("tmux", "display-message", "-p", "-t", paneID, format).Output()
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	if len(parts) < 2 {
		return "", "", nil
	}
	return parts[0], parts[1], nil
}

func setSpinnerPrefix(paneID, sessionName, prefix string) {
	_ = exec.Command("tmux", "set-option", "-p", "-q", "-t", paneID, agentTitlePrefixOption, prefix).Run()
	if sessionName != "" {
		_ = exec.Command("tmux", "set-option", "-q", "-t", sessionName, agentTitlePrefixOption, prefix).Run()
		refreshSessionClients(sessionName)
	}
}

func refreshSessionClients(sessionName string) {
	if sessionName == "" {
		return
	}
	out, err := exec.Command("tmux", "list-clients", "-t", sessionName, "-F", "#{client_name}").Output()
	if err != nil {
		return
	}
	for _, client := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if client == "" {
			continue
		}
		_ = exec.Command("tmux", "refresh-client", "-t", client).Run()
	}
}
