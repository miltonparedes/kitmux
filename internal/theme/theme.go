package theme

import "github.com/charmbracelet/lipgloss"

// Mellifluous palette extracted from tmux/darwin.conf
var (
	Accent = lipgloss.Color("#c0af8c")
	Dim    = lipgloss.Color("#5b5b5b")
	Fg     = lipgloss.Color("#dadada")
	Border = lipgloss.Color("#5b5b5b")
	Muted  = lipgloss.Color("#3d3d3d")
	Purple = lipgloss.Color("#a8a1be")
	Green  = lipgloss.Color("#8fae80")
	Red    = lipgloss.Color("#c47a7a")
	Yellow = lipgloss.Color("#d4a959")
)

// Reusable styles
var (
	TreeNodeNormal = lipgloss.NewStyle().
			Foreground(Fg)

	TreeNodeSelected = lipgloss.NewStyle().
				Foreground(Accent).
				Bold(true)

	TreeGroupHeader = lipgloss.NewStyle().
			Foreground(Dim).
			Italic(true)

	TreeConnector = lipgloss.NewStyle().
			Foreground(Dim)

	TreeMeta = lipgloss.NewStyle().
			Foreground(Dim)

	PaletteContainer = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Border).
				Padding(1, 2)

	PaletteItem = lipgloss.NewStyle().
			Foreground(Fg)

	PaletteItemSelected = lipgloss.NewStyle().
				Foreground(Accent).
				Bold(true)

	PaletteCategory = lipgloss.NewStyle().
			Foreground(Dim).
			Italic(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(Dim)

	AttachedBadge = lipgloss.NewStyle().
			Foreground(Purple)

	// Panel styles
	PanelActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Accent)

	PanelInactive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Muted)

	PanelTitle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true).
			Padding(0, 1)

	PanelTitleInactive = lipgloss.NewStyle().
				Foreground(Dim).
				Padding(0, 1)

	// Worktree styles
	DirtyBadge = lipgloss.NewStyle().
			Foreground(Yellow)

	CleanBadge = lipgloss.NewStyle().
			Foreground(Green)

	DiffAdded = lipgloss.NewStyle().
			Foreground(Green)

	DiffRemoved = lipgloss.NewStyle().
			Foreground(Red)

	// Agent styles
	AgentName = lipgloss.NewStyle().
			Foreground(Fg)

	AgentMode = lipgloss.NewStyle().
			Foreground(Dim)

	AgentModeSelected = lipgloss.NewStyle().
				Foreground(Purple)
)
