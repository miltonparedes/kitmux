# kitmux

A tmux command palette and session manager built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Every operation is a command that can be triggered from the palette, bound to a tmux key, or run directly from the CLI.

## Features

- **Command palette** (`palette`) — fuzzy-search and run any command
- **Session tree** (`sessions`) — browse, switch, rename, and kill tmux sessions
- **Project picker** (`projects`) — open zoxide-indexed projects as tmux sessions
- **Window list** (`windows`) — switch windows in the current session
- **Worktree manager** (`worktrees`) — list, create, switch, and remove git worktrees
- **Agent launcher** (`agents`) — launch AI coding agents (Claude, Gemini, Codex, etc.)
- **Direct execution** (`run <id>`) — execute any palette command headlessly

## Usage

```
kitmux <command> [options]

Views:
  sessions (s)    Session tree
  palette  (p)    Command palette
  worktrees (wt)  Worktree manager
  agents   (a)    Agent launcher
  projects (o)    Open a project
  windows  (w)    Window list for current session

Execute:
  run <id>        Run a palette command by ID
  commands        List all available command IDs
  <id>            Shorthand for 'run <id>'

Options:
  --super KEY     Modifier for 1-9 jump (alt|none, default: none)
```

## tmux Bindings

The command palette is the central hub — any operation can be bound as a tmux popup:

```tmux
# Command palette
bind-key p display-popup -E -w 60% -h 80% "kitmux palette"

# Session tree
bind-key s display-popup -E -w 40% -h 80% "kitmux sessions"

# Open project
bind-key o display-popup -E -w 60% -h 80% "kitmux projects"

# Window list
bind-key w display-popup -E -w 40% -h 60% "kitmux windows"

# Direct agent launch
bind-key C display-popup -E "kitmux launch_claude"

# Lazygit in popup
bind-key g display-popup -E "kitmux tool_lazygit"
```

## Requirements

- Go 1.25+
- tmux (running inside a tmux session)

## Install

```sh
go install github.com/miltonparedes/kitmux@latest
```

Or build from source:

```sh
just build
```
