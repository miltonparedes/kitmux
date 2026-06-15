package agenthooks

import (
	"fmt"
	"io"
	"os"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

const agentStateOption = "@kitmux_agent_state"

type StateEvent struct {
	State string
	Bell  bool
}

type StateOps struct {
	CurrentThreadContext    func() (tmux.ThreadContext, error)
	SetCurrentPaneOption    func(string, string) error
	SetCurrentSessionOption func(string, string) error
	EmitBell                func(io.Writer) error
}

func DefaultStateOps() StateOps {
	return StateOps{
		CurrentThreadContext:    tmux.CurrentThreadContext,
		SetCurrentPaneOption:    tmux.SetCurrentPaneOption,
		SetCurrentSessionOption: tmux.SetCurrentSessionOption,
		EmitBell:                emitBell,
	}
}

func RunStateEvent(event StateEvent, out io.Writer, ops StateOps) error {
	state, err := normalizeState(event.State)
	if err != nil {
		return err
	}

	ops = ops.withDefaults()
	_ = ops.SetCurrentPaneOption(agentStateOption, state)
	if ctx, err := ops.CurrentThreadContext(); err == nil && ctx.Thread {
		_ = ops.SetCurrentSessionOption(agentStateOption, state)
	}
	if event.Bell {
		_ = ops.EmitBell(out)
	}
	return nil
}

func normalizeState(state string) (string, error) {
	switch state {
	case "idle", "working", "input":
		return state, nil
	default:
		return "", fmt.Errorf("invalid agent state %q", state)
	}
}

func (ops StateOps) withDefaults() StateOps {
	defaults := DefaultStateOps()
	if ops.CurrentThreadContext == nil {
		ops.CurrentThreadContext = defaults.CurrentThreadContext
	}
	if ops.SetCurrentPaneOption == nil {
		ops.SetCurrentPaneOption = defaults.SetCurrentPaneOption
	}
	if ops.SetCurrentSessionOption == nil {
		ops.SetCurrentSessionOption = defaults.SetCurrentSessionOption
	}
	if ops.EmitBell == nil {
		ops.EmitBell = defaults.EmitBell
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
