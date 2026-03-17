# kitmux

Think [Raycast](https://www.raycast.com/), but for tmux — a command palette, session manager, worktree navigator, and AI agent launcher that lives inside your terminal.

Fast, lightweight, and built in Go. Every operation is a command you can fuzzy-search from the palette, bind to a tmux key, or fire directly from the CLI.

## Features

- **Command palette** (`palette`) — fuzzy-search and run any command
- **Session tree** (`sessions`) — browse, switch, rename, and kill tmux sessions
- **Workspace dashboard** (`workspaces`) — browse registered repos, open or switch tmux sessions, inspect worktrees, and hide repos from the dashboard
- **Window list** (`windows`) — switch windows in the current session
- **Worktree manager** (`worktrees`) — list, create, switch, and remove git worktrees
- **Agent launcher** (`agents`) — launch AI coding agents from your session directory
- **A/B agent launch** (`agent_ab`) — launch Codex and Claude side-by-side with a shared prompt
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
  workspaces (o)  Workspace dashboard
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

# Workspace dashboard
bind-key o display-popup -E -w 60% -h 80% "kitmux workspaces"

# Window list
bind-key w display-popup -E -w 40% -h 60% "kitmux windows"

# Direct agent launch
bind-key C display-popup -E "kitmux launch_claude"

# A/B launch (Codex + Claude)
bind-key A display-popup -E "kitmux agent_ab"

# Lazygit in popup
bind-key g display-popup -E "kitmux tool_lazygit"
```

> **Tip: faster popups.** tmux uses `default-shell` to run popup commands.
> If your shell is fish/zsh (~650ms startup), popups will feel sluggish. Set
> `default-shell` to `/bin/sh` (~3ms) and use `default-command` for interactive
> sessions:
>
> ```tmux
> set -g default-shell /bin/sh
> set -g default-command "exec /path/to/fish"
> ```
>
> Popup commands like `kitmux palette` will now run via `/bin/sh -c` (instant),
> while new panes and windows still start your preferred shell.

## Workspace Dashboard

`workspaces` is the repo-level dashboard for kitmux:

- shows registered repositories and their active tmux sessions
- lets you add repos from zoxide and open them directly
- lets you inspect project worktrees before opening or creating a session
- `d` only hides a workspace from the dashboard; it does not delete branches, worktrees, or other repo state

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

- Go 1.25+ (build only)
- tmux

## Recommended Dependencies

kitmux shells out to external tools for most of its features. Install the ones you plan to use:

**Core — highly recommended for the full experience:**

| Tool | Features it unlocks |
|------|-------------------|
| [git](https://git-scm.com/) | Repo detection, branch info, worktree support |
| [zoxide](https://github.com/ajeetdsouza/zoxide) | Workspace discovery — find and register project directories |
| [worktrunk](https://github.com/max-sixty/worktrunk) (`wt`) | Worktree management — create, switch, and remove git worktrees |

**AI agents — special support for these coding agents (others work too):**

| Tool | Description |
|------|-------------|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | Anthropic's CLI coding agent |
| [Codex](https://github.com/openai/codex) | OpenAI's CLI coding agent |
| [Droid](https://docs.factory.ai/droid/overview) | Factory's CLI coding agent (coming soon) |

**Extras:**

| Tool | What it does |
|------|-------------|
| [lazygit](https://github.com/jesseduffield/lazygit) | Git TUI — launched via the `tool_lazygit` palette command |

## A/B Mode Configuration

`agent_ab` uses these optional environment variables:

| Environment variable | Description | Default |
|---------------------|-------------|---------|
| `KITMUX_AB_CODEX_TEMPLATE` | Command template for Codex (`{prompt}` placeholder required) | `codex {prompt}` |
| `KITMUX_AB_CLAUDE_TEMPLATE` | Command template for Claude (`{prompt}` placeholder required) | `claude {prompt}` |
| `KITMUX_AB_PLAN_PREFIX` | Prefix added to prompt when plan mode is enabled | `/plan ` |
| `KITMUX_AB_BASE_BRANCH` | Base branch used to create/reuse A/B worktrees | `main` |

## Install

```sh
go install github.com/miltonparedes/kitmux@latest
```

Or build from source (requires [just](https://github.com/casey/just)):

```sh
just build
```
