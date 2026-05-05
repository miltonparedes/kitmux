# kitmux

kitmux is a tmux command palette for moving around projects quickly.

Open one popup, fuzzy-search what you want, and kitmux handles the tmux action:
switch sessions, jump to windows, open worktrees, launch coding agents, or run
project tools.

## Install

Install the latest release from GitHub Releases:

```sh
curl -fsSL https://raw.githubusercontent.com/miltonparedes/kitmux/main/install.sh | sh
```

By default, the script installs to `~/.local/bin`. To choose another directory:

```sh
curl -fsSL https://raw.githubusercontent.com/miltonparedes/kitmux/main/install.sh | INSTALL_DIR="$HOME/bin" sh
```

To install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/miltonparedes/kitmux/main/install.sh | KITMUX_VERSION=vX.Y.Z sh
```

Requirements:

- tmux

Supported binaries:

- macOS arm64 and amd64
- Linux arm64 and amd64

Optional tools unlock extra commands:

- `git` for repo and branch detection
- `zoxide` for discovering workspace directories
- `wt` from [worktrunk](https://github.com/max-sixty/worktrunk) for worktree operations
- `claude`, `codex`, `gemini`, `aichat`, or `opencode` for agent launch commands
- `lazygit` for the lazygit popup

## Quick Start

Add one tmux binding:

```tmux
bind-key p display-popup -E -w 60% -h 80% "kitmux palette"
```

Reload tmux, press your tmux prefix and `p`, then search for a command.

Useful commands:

```sh
kitmux palette      # command palette
kitmux sessions     # session tree
kitmux workspaces   # project dashboard
kitmux worktrees    # git worktree manager
kitmux agents       # coding agent launcher
kitmux workbench    # agent sidecar panel
kitmux windows      # windows in the current session
kitmux commands     # list command IDs
kitmux run <id>     # run a palette command directly
```

Short aliases also work: `p`, `s`, `o`, `wt`, `a`, and `w`.

## The Idea

kitmux treats tmux as a project launcher:

- sessions are grouped by repository
- worktrees stay close to their parent project
- palette commands can be used interactively or bound directly in tmux
- agent commands start from the current session directory

Most commands shell out to tools you already use. If an optional tool is not
installed, only the command that needs it is affected.

## tmux Bindings

Start with the palette binding above. Add direct bindings for views or commands
you use often:

```tmux
bind-key s display-popup -E -w 40% -h 80% "kitmux sessions"
bind-key o display-popup -E -w 60% -h 80% "kitmux workspaces"
bind-key w display-popup -E -w 40% -h 60% "kitmux windows"
bind-key g display-popup -E "kitmux tool_lazygit"
bind-key A display-popup -E "kitmux agent_ab"
```

tmux runs popup commands through `default-shell`. If popups feel slow because
your shell has a heavy startup, use a lightweight default shell for tmux command
execution:

```tmux
set -g default-shell /bin/sh
set -g default-command "exec /path/to/your/shell"
```

## Workspaces

`kitmux workspaces` is the repo dashboard. It shows registered repositories,
active tmux sessions, and worktrees.

From there you can:

- add repos discovered by zoxide
- open or switch to repo sessions
- inspect and open worktrees
- hide a repo from the dashboard

Hiding a workspace only removes it from the dashboard. It does not delete the
repo, branches, worktrees, or tmux state.

## Agent A/B

`kitmux agent_ab` opens Codex and Claude side-by-side with the same prompt.

Optional environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `KITMUX_AB_CODEX_TEMPLATE` | `codex {prompt}` | Command template for Codex |
| `KITMUX_AB_CLAUDE_TEMPLATE` | `claude {prompt}` | Command template for Claude |
| `KITMUX_AB_PLAN_PREFIX` | `/plan ` | Prefix when plan mode is enabled |
| `KITMUX_AB_BASE_BRANCH` | `main` | Base branch for A/B worktrees |

## Agent Workbench

`kitmux workbench` is a compact sidecar panel for agent-heavy tmux layouts. Its
summary view shows progress, branch details, changes, artifacts, sources,
project file/line stats, quick agent launch targets, and actions for existing
kitmux tools.

When launching an agent into the current pane, kitmux can open the agent on the
left and Workbench on the right. Tool actions such as lazygit and Lumen Diff run
inside the Workbench pane so the agent stays visible. Agent actions send the
selected agent command to the neighboring agent pane, and opening the local
editor keeps Workbench alive.

Optional environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `KITMUX_AGENT_WORKBENCH` | `auto` | `auto`, `always`, or `off` |
| `KITMUX_AGENT_WORKBENCH_MIN_WIDTH` | `160` | Minimum tmux client width for `auto` |
| `KITMUX_AGENT_WORKBENCH_RATIO` | `30` | Width percentage for the Workbench pane |
| `KITMUX_WORKBENCH_COMMAND` | `kitmux workbench` | Command used to start the sidecar pane |

## Local Editor Bridge

`open_local_editor` is experimental. It is meant for remote tmux sessions where
you want to open the current remote directory in a local editor over SSH.

On your local machine:

```sh
kitmux bridge install
```

Or run the bridge manually:

```sh
kitmux bridge serve
```

Optional environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `KITMUX_EDITOR` | `zed` | `zed` or `vscode` |
| `KITMUX_SSH_HOST` | auto-detected | SSH host alias for the remote machine |
| `KITMUX_OPEN_EDITOR_SOCK` | `/tmp/kitmux-bridge.sock` | Bridge Unix socket path |

## Release Artifacts

GitHub releases publish tarballs for:

- macOS arm64 and amd64
- Linux arm64 and amd64

Each tarball contains the `kitmux` binary, this README, and `install.sh`.
