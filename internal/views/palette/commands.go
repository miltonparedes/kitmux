package palette

import tea "github.com/charmbracelet/bubbletea"

// Command represents an executable command in the palette.
type Command struct {
	ID          string
	Title       string
	Description string
	Category    string
	Action      func() tea.Msg
}

// IsValidCommand returns true if the given ID matches a registered command.
func IsValidCommand(id string) bool {
	for _, cmd := range DefaultCommands() {
		if cmd.ID == id {
			return true
		}
	}
	return false
}

// DefaultCommands returns the built-in command registry.
func DefaultCommands() []Command {
	return []Command{
		// Session
		{
			ID:          "switch_session",
			Title:       "Switch Session",
			Description: "Switch to a session (use the tree view)",
			Category:    "Session",
		},
		{
			ID:          "kill_session",
			Title:       "Kill Session",
			Description: "Kill the selected session",
			Category:    "Session",
		},
		{
			ID:          "kill_current_session",
			Title:       "Kill Current Session",
			Description: "Kill the current tmux session and switch to another",
			Category:    "Session",
		},
		{
			ID:          "rename_session",
			Title:       "Rename Session",
			Description: "Rename the selected session",
			Category:    "Session",
		},
		{
			ID:          "open_project",
			Title:       "Open Project",
			Description: "Open a recent project as a new session",
			Category:    "Session",
		},

		// Worktree
		{
			ID:          "wt_switch",
			Title:       "Switch Worktree",
			Description: "Switch to a worktree branch",
			Category:    "Worktree",
		},
		{
			ID:          "wt_create",
			Title:       "Create Worktree",
			Description: "Create a new worktree branch",
			Category:    "Worktree",
		},
		{
			ID:          "wt_create_describe",
			Title:       "Create Worktree from Description",
			Description: "Describe a task and auto-generate a branch name",
			Category:    "Worktree",
		},
		{
			ID:          "wt_remove",
			Title:       "Remove Worktree",
			Description: "Remove the selected worktree",
			Category:    "Worktree",
		},
		{
			ID:          "wt_merge",
			Title:       "Merge Worktree",
			Description: "Merge a worktree branch into main",
			Category:    "Worktree",
		},
		{
			ID:          "wt_commit",
			Title:       "LLM Commit",
			Description: "Generate a commit message with LLM",
			Category:    "Worktree",
		},

		// Agent
		{
			ID:          "launch_claude",
			Title:       "Launch Claude Code",
			Description: "Start Claude Code in the current pane",
			Category:    "Agent",
		},
		{
			ID:          "launch_gemini",
			Title:       "Launch Gemini CLI",
			Description: "Start Gemini CLI in the current pane",
			Category:    "Agent",
		},
		{
			ID:          "launch_codex",
			Title:       "Launch Codex CLI",
			Description: "Start Codex CLI in the current pane",
			Category:    "Agent",
		},
		{
			ID:          "launch_aichat",
			Title:       "Launch AIChat",
			Description: "Start AIChat in the current pane",
			Category:    "Agent",
		},
		{
			ID:          "launch_opencode",
			Title:       "Launch OpenCode",
			Description: "Start OpenCode in the current pane",
			Category:    "Agent",
		},
		{
			ID:          "agent_ab",
			Title:       "Launch A/B (Codex + Claude)",
			Description: "Start Codex and Claude side-by-side in a new tmux window",
			Category:    "Agent",
		},

		// Editor
		{
			ID:          "open_local_editor",
			Title:       "Open in Local Editor",
			Description: "Open current session in your local editor",
			Category:    "Editor",
		},

		// Tools
		{
			ID:          "tool_lazygit",
			Title:       "Lazygit",
			Description: "Open lazygit in a popup",
			Category:    "Tool",
		},
		{
			ID:          "tool_lumen_diff",
			Title:       "Lumen Diff",
			Description: "Open lumen diff in a popup",
			Category:    "Tool",
		},

		// View
		{
			ID:          "view_sessions",
			Title:       "Sessions View",
			Description: "Switch to sessions view",
			Category:    "View",
		},
		{
			ID:          "view_worktrees",
			Title:       "Worktrees View",
			Description: "Switch to worktrees view",
			Category:    "View",
		},
		{
			ID:          "view_agents",
			Title:       "Agents View",
			Description: "Switch to agents view",
			Category:    "View",
		},
	}
}
