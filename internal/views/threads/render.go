package threads

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/miltonparedes/kitmux/internal/theme"
)

// titleCol is the column where the title and meta text begin: a left gutter,
// the state icon at column 3, two spaces, then the title. Keeping it constant
// across rows lets meta lines align under their titles.
const titleCol = 6

func (m Model) View() string {
	if m.picking {
		return m.pickerView()
	}

	var b strings.Builder
	b.WriteString(m.headerLine() + "\n\n")

	avail := m.contentHeight()
	lines := m.cardLines()
	for i := 0; i < avail; i++ {
		if i < len(lines) {
			b.WriteString(lines[i])
		}
		b.WriteString("\n")
	}

	b.WriteString(m.separatorLine() + "\n")
	b.WriteString(m.footerLine())
	return b.String()
}

func (m Model) headerLine() string {
	left := " " + theme.TreeNodeSelected.Render("Agent Threads")
	right := ""
	if n := len(m.rows); n > 0 {
		label := "threads"
		if n == 1 {
			label = "thread"
		}
		right = theme.TreeMeta.Render(fmt.Sprintf("%d %s ", n, label))
	}
	return padBetween(left, right, m.width)
}

func (m Model) separatorLine() string {
	sepW := m.width - 2
	if sepW < 1 {
		sepW = 1
	}
	return " " + theme.TreeConnector.Render(strings.Repeat("─", sepW))
}

func (m Model) footerLine() string {
	help := theme.HelpStyle.Render(" ⏎ open   n new   d/K kill   r refresh   q quit")
	pager := ""
	if len(m.rows) > 0 {
		pager = theme.TreeMeta.Render(fmt.Sprintf("%d / %d ", m.cursor+1, len(m.rows)))
	}
	return padBetween(help, pager, m.width)
}

func (m Model) cardLines() []string {
	if len(m.rows) == 0 {
		return []string{theme.HelpStyle.Render("   no running agents")}
	}

	perView := m.cardsPerView()
	start := m.scroll
	end := start + perView
	if end > len(m.rows) {
		end = len(m.rows)
	}

	lines := make([]string, 0, (end-start)*linesPerCard)
	for i := start; i < end; i++ {
		title, meta := m.cardRow(i)
		lines = append(lines, title, meta)
	}
	return lines
}

// cardRow renders the two lines of one thread: the title line (icon, title,
// age and badges) and the dim meta line (project · branch · location).
func (m Model) cardRow(i int) (string, string) {
	row := m.rows[i]
	selected := i == m.cursor
	right := m.rightCluster(row, selected)
	meta := metaLine(row)

	if selected {
		return m.selectedTitleLine(row, right), m.selectedMetaLine(meta)
	}
	return m.normalTitleLine(row, right), m.normalMetaLine(meta)
}

func (m Model) normalTitleLine(row Row, right string) string {
	icon := m.iconRender(row, false)
	titleMax := m.width - titleCol - lipgloss.Width(right) - 2
	title := theme.TreeNodeNormal.Render(truncate(rowTitle(row), titleMax))
	left := "   " + icon + "  " + title
	return joinLine(left, right, m.width)
}

func (m Model) normalMetaLine(meta string) string {
	indent := strings.Repeat(" ", titleCol)
	return indent + theme.TreeMeta.Render(truncate(meta, m.width-titleCol-1))
}

func (m Model) selectedTitleLine(row Row, right string) string {
	icon := m.iconRender(row, true)
	titleMax := m.width - titleCol - lipgloss.Width(right) - 2
	title := theme.SelectionTitle.Render(truncate(rowTitle(row), titleMax))
	left := theme.SelectionBar.Render("  ") + icon + theme.SelectionBar.Render("  ") + title
	return joinSelectedLine(" ", left, right, m.width)
}

func (m Model) selectedMetaLine(meta string) string {
	indent := theme.SelectionBar.Render(strings.Repeat(" ", titleCol-1))
	body := theme.SelectionMeta.Render(truncate(meta, m.width-titleCol-1))
	left := indent + body
	return joinSelectedLine(" ", left, "", m.width)
}

// rightCluster builds the right-aligned column: relative age plus the detected
// and attached badges, styled for the selection bar when the row is selected.
func (m Model) rightCluster(row Row, selected bool) string {
	metaStyle := theme.TreeMeta
	modeStyle := theme.AgentMode
	attachStyle := theme.AttachedBadge
	if selected {
		metaStyle = theme.SelectionMeta
		modeStyle = theme.SelectionMeta
		attachStyle = theme.SelectionBar.Foreground(theme.Purple)
	}

	var parts []string
	if age := rowAge(row); age != "" {
		parts = append(parts, metaStyle.Render(age))
	}
	if row.Kind == RowEphemeral {
		parts = append(parts, modeStyle.Render("detected"))
	}
	if row.Attached {
		parts = append(parts, attachStyle.Render("●"))
	}
	if len(parts) == 0 {
		return ""
	}
	sep := "  "
	if selected {
		sep = theme.SelectionBar.Render("  ")
	}
	return strings.Join(parts, sep)
}

func (m Model) iconRender(row Row, selected bool) string {
	style := lipgloss.NewStyle().Foreground(stateColor(row.AgentState))
	if selected {
		style = style.Background(theme.Dim)
	}
	return style.Render(rowIcon(row, m.spinnerFrame))
}

func (m Model) pickerView() string {
	var b strings.Builder
	b.WriteString(" " + theme.TreeNodeSelected.Render("New Headless Agent") + "\n")
	viewHeight := m.height - 2
	if viewHeight < 1 {
		viewHeight = 1
	}

	end := viewHeight
	if end > len(m.agents) {
		end = len(m.agents)
	}
	for i := 0; i < end; i++ {
		agent := m.agents[i]
		if i == m.agentIndex {
			b.WriteString(theme.TreeNodeSelected.Render("› " + agent.DisplayName()))
		} else {
			b.WriteString("  " + theme.TreeNodeNormal.Render(agent.DisplayName()))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	for rendered := end; rendered < viewHeight; rendered++ {
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render(" ⏎ create  esc back"))
	return b.String()
}

// metaLine builds the dim second line: project · branch, with the ephemeral
// window.pane location appended. Empty parts are omitted.
func metaLine(row Row) string {
	var parts []string
	if row.Project != "" {
		parts = append(parts, row.Project)
	}
	if row.Branch != "" {
		parts = append(parts, row.Branch)
	}
	meta := strings.Join(parts, " · ")
	if row.Kind == RowEphemeral {
		loc := fmt.Sprintf("%d.%d", row.WindowIndex, row.PaneIndex)
		if meta == "" {
			return loc
		}
		return meta + " · " + loc
	}
	return meta
}

func badges(row Row) string {
	var parts []string
	if row.Kind == RowEphemeral {
		parts = append(parts, theme.AgentMode.Render("detected"))
	}
	if row.Attached {
		parts = append(parts, theme.AttachedBadge.Render("●"))
	}
	return strings.Join(parts, " ")
}

func truncate(s string, limit int) string {
	if limit < 1 {
		return ""
	}
	return ansi.Truncate(s, limit, "…")
}

// joinLine right-aligns right within width, leaving at least one trailing cell.
func joinLine(left, right string, width int) string {
	if right == "" {
		return left
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// joinSelectedLine fills the gap between left and right with the selection bar
// so the highlight spans the full row. The right column lands at the same
// position as on unselected rows (one trailing highlighted cell after it).
func joinSelectedLine(prefix, left, right string, width int) string {
	gap := width - lipgloss.Width(prefix) - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 0 {
		gap = 0
	}
	return prefix + left + theme.SelectionBar.Render(strings.Repeat(" ", gap)) + right + theme.SelectionBar.Render(" ")
}

func padBetween(left, right string, width int) string {
	if right == "" {
		return left
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// rowAge renders the time since the thread was last active as a compact label
// (now, 5m, 2h, 3d), falling back to the agent hook timestamp.
func rowAge(row Row) string {
	epoch := row.Activity
	if epoch == 0 && row.AgentUpdated != 0 {
		epoch = row.AgentUpdated / 1000
	}
	if epoch == 0 {
		return ""
	}
	d := time.Since(time.Unix(epoch, 0))
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func stateColor(state string) lipgloss.Color {
	switch state {
	case "working":
		return theme.Accent
	case "input", "permission":
		return theme.Yellow
	case "error":
		return theme.Red
	default:
		return theme.Green
	}
}

// rowTitle returns the thread title without a leading status glyph, since the
// state icon is rendered separately in its own column.
func rowTitle(row Row) string {
	title := row.Title
	if title == "" {
		title = row.AgentName
	}
	if title == "" {
		title = row.AgentID
	}
	return stripLeadingStatusGlyph(title)
}

func rowIcon(row Row, spinnerFrame int) string {
	switch row.AgentState {
	case "working":
		return spinnerFrames[spinnerFrame%len(spinnerFrames)]
	case "input":
		return "⮞"
	case "permission":
		return "!"
	case "error":
		return "×"
	default:
		return rowSymbol(row)
	}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func stripLeadingStatusGlyph(title string) string {
	title = strings.TrimLeft(title, " \t")
	for title != "" {
		stripped := false
		for index, r := range title {
			if index != 0 {
				break
			}
			if strings.ContainsRune(statusGlyphs, r) {
				title = strings.TrimLeft(title[len(string(r)):], " \t")
				stripped = true
			}
			break
		}
		if !stripped {
			return title
		}
	}
	return title
}

const statusGlyphs = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⠂⠆⠤⠰⠠⠐⛬✳✻✶✢✤✱›⌾⌬⮞"

func rowSymbol(row Row) string {
	if row.AgentSymbol != "" {
		return row.AgentSymbol
	}
	value := row.AgentName
	if value == "" {
		value = row.AgentID
	}
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return strings.ToUpper(string(r))
		}
	}
	return ""
}
