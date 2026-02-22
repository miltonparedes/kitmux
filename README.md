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
- **Open in local editor** (`open_local_editor`) — open remote sessions in your local editor via SSH (coming soon)
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

Other:
  completion      Generate shell autocompletion scripts

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

## Open in Local Editor (coming soon)

A bridge that lets you open remote tmux session directories in your local editor (Zed or VS Code) over SSH.

**How it works:**

1. A lightweight bridge server runs on your **local machine**, listening on a Unix socket.
2. When you trigger `open_local_editor` from the palette on a **remote machine**, kitmux sends the current session path and SSH host to the bridge.
3. The bridge launches your local editor with the appropriate remote connection (e.g., `zed ssh://host/path` or `code --remote ssh-remote+host path`).

**Setup:**

```sh
# On your local machine — install the bridge as a macOS LaunchAgent
kitmux bridge install

# Or run it manually
kitmux bridge serve
```

**Configuration:**

| Environment variable | Description | Default |
|---------------------|-------------|---------|
| `KITMUX_EDITOR` | Editor to use (`zed` or `vscode`) | `zed` |
| `KITMUX_SSH_HOST` | SSH host alias for the remote machine | auto-detected / cached |
| `KITMUX_OPEN_EDITOR_SOCK` | Bridge Unix socket path | `/tmp/kitmux-bridge.sock` |

## Requirements

**Required:**

- Go 1.25+
- tmux

**Runtime dependencies (used by specific features):**

| Dependency | Used by | Required? |
|------------|---------|-----------|
| [zoxide](https://github.com/ajeetdsouza/zoxide) | `projects` — indexes project directories | Yes, for project picker |
| [git](https://git-scm.com/) | `sessions`, `projects` — detects repos and worktrees | Yes, for git-aware features |
| [worktrunk](https://github.com/max-sixty/worktrunk) (`wt`) | `worktrees` — manages git worktrees | Yes, for worktree manager |
| [lazygit](https://github.com/jesseduffield/lazygit) | `tool_lazygit` palette command | Optional |
| [claude](https://docs.anthropic.com/en/docs/claude-code) | `agents` — Claude Code | Optional |
| [gemini](https://github.com/google-gemini/gemini-cli) | `agents` — Gemini CLI | Optional |
| [codex](https://github.com/openai/codex) | `agents` — Codex CLI | Optional |

## Install

```sh
go install github.com/miltonparedes/kitmux@latest
```

Or build from source (requires [just](https://github.com/casey/just)):

```sh
just build
```
