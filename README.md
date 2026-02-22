# kitmux

A tmux session manager and command palette built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **Session sidebar** (`-s`) — browse and switch tmux sessions/windows
- **Command palette** (`-p`) — fuzzy-search commands, agents, and worktree actions
- **Worktree manager** (`-w`) — list, create, switch, and remove git worktrees
- **Agent launcher** (`-a`) — launch AI coding agents (Claude, Gemini, Codex, etc.) in tmux panes

## Usage

```
kitmux <flag>

  -s, --sidebar     Session sidebar
  -p, --palette     Command palette
  -w, --worktrees   Worktree manager
  -a, --agents      Agent launcher
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
