package theme

import "github.com/charmbracelet/lipgloss"

// ANSI 16 colors — adapts to the terminal's active theme.
var (
	Accent = lipgloss.Color("6")  // Cyan
	Dim    = lipgloss.Color("8")  // BrightBlack
	Border = lipgloss.Color("8")  // BrightBlack
	Muted  = lipgloss.Color("8")  // BrightBlack
	Purple = lipgloss.Color("5")  // Magenta
	Green  = lipgloss.Color("2")  // Green
	Red    = lipgloss.Color("1")  // Red
	Yellow = lipgloss.Color("3")  // Yellow
)

// Reusable styles
var (
	TreeNodeNormal = lipgloss.NewStyle()

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

	PaletteItem = lipgloss.NewStyle()

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
	AgentName = lipgloss.NewStyle()

	AgentMode = lipgloss.NewStyle().
			Foreground(Dim)

	AgentModeSelected = lipgloss.NewStyle().
				Foreground(Purple)
)
