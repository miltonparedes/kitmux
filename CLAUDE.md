# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development

```sh
just build          # Build binary → ./kitmux
just run -- <args>  # Run with args (e.g. just run -- palette)
just test           # go test ./...
just lint           # golangci-lint run
just fmt            # golangci-lint run --fix (gofumpt formatting)
just check          # lint + test
just install        # go install to $GOPATH/bin
```

Run a single test: `go test ./internal/views/sessions/ -run TestBuildTree`

Requires Go 1.25+ and golangci-lint (v2, config in `.golangci.yml` — uses gofumpt formatter).

## Architecture

kitmux is a tmux session manager and command palette TUI built with Bubble Tea. It shells out to `tmux`, `git`, `wt` (worktrunk), and `zoxide` — no client libraries, just `exec.Command`.

### Entry flow

`main.go` → `cmd.Execute()` → cobra root command. Each CLI subcommand (`sessions`, `palette`, `worktrees`, etc.) maps to an `app.Mode` and launches the Bubble Tea program via `runTUI()` in `internal/cmd/views.go`.

### Package structure

- **`internal/app/`** — Top-level Bubble Tea `Model`. Owns view routing (sessions/windows/worktrees/agents), palette overlay toggle, and command execution dispatch (`executeCommand` switch). All inter-view coordination lives here.
- **`internal/app/messages/`** — Shared message types (Bubble Tea `Msg` structs) used across views. This is the message bus — views communicate through these types rather than calling each other directly.
- **`internal/views/`** — Each view is a sub-package with its own `Model`/`Update`/`View` (Bubble Tea pattern). Views are: `sessions`, `windows`, `worktrees`, `agents`, `palette`.
- **`internal/tmux/`** — Thin wrapper over tmux CLI commands (`list-sessions`, `switch-client`, `kill-session`, etc.). All tmux interaction goes through here.
- **`internal/worktree/`** — Wrapper over `wt` (worktrunk) CLI for worktree operations.
- **`internal/agents/`** — Agent registry (`DefaultAgents()`) defining available AI coding agents and their modes.
- **`internal/config/`** — Runtime config (currently just `SuperKey` flag).
- **`internal/theme/`** — Lipgloss styles using ANSI 16 colors for terminal-theme adaptation.
- **`internal/cmd/`** — Cobra command setup. `root.go` builds the CLI tree, `views.go` registers view subcommands, `commands.go` lists palette command IDs, `run.go` handles `kitmux run <id>`.

### Command palette

The palette (`internal/views/palette/commands.go`) defines a flat registry of `Command` structs with string IDs. Commands are dispatched in `app.go:executeCommand()` via a switch on the ID. Every palette command ID is also registered as a hidden cobra subcommand so `kitmux switch_session` works as shorthand for `kitmux run switch_session`.

### Session tree

Sessions are grouped into a tree by git repository root (`BuildTree` in `sessions/tree.go`). Sessions sharing the same repo root (including worktrees) are grouped under the repo basename. Worktree diff stats are loaded asynchronously after the initial session load.

### Key patterns

- **Value-receiver Models**: Bubble Tea requires value receivers on `Update`/`View`. The `returnToPalette` method uses a pointer receiver intentionally (noted in gocritic config).
- **Message-driven**: Views never call each other. All cross-view communication flows through `messages.*Msg` types dispatched via Bubble Tea commands.
- **Pending key injection**: Some palette commands (kill, rename) switch to the sessions view and inject a synthetic key event after sessions finish loading (`pendingKey` + `ConsumeLoaded`).
