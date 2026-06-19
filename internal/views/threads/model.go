package threads

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agentrename"
	"github.com/miltonparedes/kitmux/internal/agentresume"
	"github.com/miltonparedes/kitmux/internal/agents"
	"github.com/miltonparedes/kitmux/internal/agentthread"
	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

type RowKind int

const (
	RowHeadless RowKind = iota
	RowEphemeral
)

type Row struct {
	Kind           RowKind
	AgentID        string
	AgentName      string
	AgentSymbol    string
	AgentState     string
	AgentEvent     string
	AgentDetail    string
	AgentUpdated   int64
	Title          string
	TitleOverride  bool
	ThreadTitle    string
	PaneTitle      string
	SessionName    string
	WindowIndex    int
	PaneIndex      int
	PaneID         string
	PanePID        int
	AgentSessionID string
	Path           string
	Project        string
	Branch         string
	Attached       bool
	Activity       int64
}

type Model struct {
	rows         []Row
	agents       []agents.Agent
	cursor       int
	scroll       int
	height       int
	width        int
	picking      bool
	renaming     bool
	renameInput  textinput.Model
	agentIndex   int
	spinnerFrame int
	launchDir    string
}

type loadedMsg struct {
	rows []Row
}

type tickMsg struct{}

const refreshEveryFrames = 6

var (
	lookupAgentTitle    = agentrename.Title
	syncThreadTitle     = tmux.SetThreadTitle
	syncThreadPrefix    = tmux.SetThreadTitlePrefix
	syncPaneTitle       = tmux.SetPaneTitle
	refreshThreadClient = tmux.RefreshClients
	createThread        = agentthread.Create
)

func New(launchDir ...string) Model {
	ri := textinput.New()
	ri.Prompt = "Rename: "
	ri.CharLimit = 96
	return Model{
		agents:      agents.DefaultAgents(),
		renameInput: ri,
		launchDir:   resolveLaunchDir(launchDir...),
	}
}

func resolveLaunchDir(values ...string) string {
	for _, value := range values {
		if dir := strings.TrimSpace(value); dir != "" {
			return dir
		}
	}
	if cwd, err := filepath.Abs("."); err == nil {
		return cwd
	}
	return ""
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(syncSupportAndLoadCmd(), tickCmd())
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m Model) IsEditing() bool {
	return m.picking || m.renaming
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.rows = msg.rows
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case tickMsg:
		m.spinnerFrame++
		if m.spinnerFrame%refreshEveryFrames == 0 {
			return m, tea.Batch(tickCmd(), loadCmd())
		}
		return m, tickCmd()
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		if m.renaming {
			return m.handleRenameKey(msg)
		}
		if m.picking {
			return m.handlePickerKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if msg.Button == tea.MouseButtonWheelUp {
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	}
	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionRelease {
		return m, nil
	}
	rel := msg.Y - headerLines
	if rel < 0 {
		return m, nil
	}
	idx := m.scroll + rel/linesPerCard
	if idx < 0 || idx >= len(m.rows) {
		return m, nil
	}
	m.cursor = idx
	return m, openRowCmd(m.rows[idx])
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if updated, handled := m.handleNav(msg); handled {
		return updated, nil
	}
	if updated, cmd, handled := m.handleAction(msg); handled {
		return updated, cmd
	}
	return m, nil
}

func (m Model) handleNav(msg tea.KeyMsg) (Model, bool) {
	switch msg.String() {
	case "j", "down":
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, true
	case "k", "up":
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, true
	case "g", "home":
		m.cursor = 0
		m.scroll = 0
		return m, true
	case "G", "end":
		m.cursor = len(m.rows) - 1
		m.ensureVisible()
		return m, true
	}
	return m, false
}

func (m Model) handleAction(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		if row := m.selected(); row != nil {
			return m, openRowCmd(*row), true
		}
		return m, nil, true
	case "n":
		m.picking = true
		m.agentIndex = 0
		return m, nil, true
	case "d", "K":
		if row := m.selected(); row != nil && row.Kind == RowHeadless {
			return m, killHeadlessCmd(row.SessionName), true
		}
		return m, nil, true
	case "ctrl+r":
		return m, loadCmd(), true
	case "r":
		if row := m.selected(); row != nil {
			m.renaming = true
			m.renameInput.SetValue(rowTitle(*row))
			m.renameInput.Focus()
			return m, textinput.Blink, true
		}
		return m, nil, true
	case "R":
		if row := m.selected(); row != nil && row.Kind == RowHeadless {
			return m, relaunchHeadlessCmd(*row), true
		}
		return m, nil, true
	case "esc", "q":
		return m, tea.Quit, true
	}
	return m, nil, false
}

func (m Model) handleRenameKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.renaming = false
		row := m.selected()
		if row == nil {
			return m, nil
		}
		title := strings.TrimSpace(m.renameInput.Value())
		return m, renameRowCmd(*row, title)
	case "esc":
		m.renaming = false
		return m, nil
	}
	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.agentIndex++
		m.clampAgentIndex()
	case "k", "up":
		m.agentIndex--
		m.clampAgentIndex()
	case "enter":
		m.picking = false
		if m.agentIndex >= 0 && m.agentIndex < len(m.agents) {
			return m, newHeadlessCmd(m.agents[m.agentIndex], m.launchDir)
		}
	case "esc":
		m.picking = false
	}
	return m, nil
}

func (m Model) selected() *Row {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return &m.rows[m.cursor]
	}
	return nil
}

func (m *Model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) clampAgentIndex() {
	if m.agentIndex < 0 {
		m.agentIndex = 0
	}
	if m.agentIndex >= len(m.agents) {
		m.agentIndex = len(m.agents) - 1
	}
	if m.agentIndex < 0 {
		m.agentIndex = 0
	}
}

func (m *Model) ensureVisible() {
	m.clampCursor()
	if m.scroll < 0 {
		m.scroll = 0
	}
	perView := m.cardsPerView()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+perView {
		m.scroll = m.cursor - perView + 1
	}
}

// linesPerCard is the rendered height of one agent card: a title line and a
// dim meta line, with no spacer between cards.
const linesPerCard = 2

// headerLines (title + blank) and footerLines (separator + help) are the fixed
// chrome rows framing the scrollable card area.
const (
	headerLines = 2
	footerLines = 2
)

// contentHeight returns the number of rows available for cards.
func (m Model) contentHeight() int {
	avail := m.height - headerLines - footerLines
	if avail < 1 {
		avail = 1
	}
	return avail
}

// cardsPerView returns how many agent cards fit in the visible area.
func (m Model) cardsPerView() int {
	n := m.contentHeight() / linesPerCard
	if n < 1 {
		n = 1
	}
	return n
}

func loadCmd() tea.Cmd {
	return func() tea.Msg {
		return loadRows()
	}
}

func syncSupportAndLoadCmd() tea.Cmd {
	return func() tea.Msg {
		_, _ = agentthread.InstallAllSupport(agentthread.DefaultOps())
		return loadRows()
	}
}

func loadRows() loadedMsg {
	sessions, _ := tmux.ListSessions()
	panes, _ := tmux.ListPanes()
	rows := buildRows(sessions, panes)
	rows = reconcilePaneTitleRenames(rows)
	rows = enrichAgentTitles(rows)
	rows = repairThreadTitlePrefixes(rows)
	return loadedMsg{rows: enrichGitMeta(rows)}
}

func reconcilePaneTitleRenames(rows []Row) []Row {
	for i := range rows {
		row := rows[i]
		if row.Kind != RowHeadless || row.SessionName == "" || row.PaneTitle == "" {
			continue
		}
		title := livePaneThreadTitle(row)
		if title == "" || title == row.ThreadTitle {
			continue
		}
		rows[i].Title = title
		rows[i].ThreadTitle = title
		rows[i].TitleOverride = true
		_ = syncThreadTitle(row.SessionName, title)
		if prefix := staticThreadTitlePrefix(rows[i]); prefix != "" {
			_ = syncThreadPrefix(row.SessionName, prefix)
		}
		_ = refreshThreadClient(row.SessionName)
	}
	return rows
}

func livePaneThreadTitle(row Row) string {
	title := strings.TrimSpace(stripLeadingStatusGlyph(row.PaneTitle))
	if title == "" || isDefaultPaneTitle(row, title) {
		return ""
	}
	return title
}

func isDefaultPaneTitle(row Row, title string) bool {
	defaults := []string{row.AgentID, row.AgentName, row.SessionName}
	if row.Path != "" && row.AgentName != "" {
		project := filepath.Base(filepath.Clean(row.Path))
		if project != "." && project != string(filepath.Separator) && project != "" {
			defaults = append(defaults, row.AgentName+" · "+project)
		}
	}
	for _, value := range defaults {
		if value != "" && strings.EqualFold(strings.TrimSpace(value), title) {
			return true
		}
	}
	return false
}

func enrichAgentTitles(rows []Row) []Row {
	for i := range rows {
		row := rows[i]
		if row.Kind != RowHeadless || row.PanePID <= 0 {
			continue
		}
		title, err := lookupAgentTitle(agentrename.Target{
			AgentID: row.AgentID,
			PanePID: row.PanePID,
		})
		if err != nil {
			continue
		}
		title = strings.TrimSpace(title)
		if title == "" || title == row.Title {
			continue
		}
		rows[i].Title = title
		rows[i].ThreadTitle = title
		rows[i].TitleOverride = true
		if row.SessionName != "" {
			_ = syncThreadTitle(row.SessionName, title)
			if prefix := staticThreadTitlePrefix(rows[i]); prefix != "" {
				_ = syncThreadPrefix(row.SessionName, prefix)
			}
			_ = refreshThreadClient(row.SessionName)
		}
	}
	return rows
}

func repairThreadTitlePrefixes(rows []Row) []Row {
	for i := range rows {
		row := rows[i]
		if row.Kind != RowHeadless || !row.TitleOverride || row.SessionName == "" {
			continue
		}
		prefix := staticThreadTitlePrefix(row)
		if prefix == "" {
			continue
		}
		_ = syncThreadPrefix(row.SessionName, prefix)
		_ = refreshThreadClient(row.SessionName)
	}
	return rows
}

func staticThreadTitlePrefix(row Row) string {
	if row.AgentState != "" && row.AgentState != "idle" {
		return ""
	}
	return rowSymbol(row)
}

func enrichGitMeta(rows []Row) []Row {
	for i := range rows {
		if rows[i].Path == "" {
			continue
		}
		meta := pathGitMeta(rows[i].Path)
		rows[i].Project = meta.project
		rows[i].Branch = meta.branch
	}
	return rows
}

func buildRows(sessions []tmux.Session, panes []tmux.Pane) []Row {
	threadSessions := tmux.ThreadSessions(sessions)
	threadSet := make(map[string]struct{}, len(threadSessions))
	panesBySession := firstPaneBySession(panes)
	rows := make([]Row, 0, len(threadSessions)+len(panes))
	for _, session := range threadSessions {
		agentName, agentSymbol := agentDisplayParts(session.AgentID)
		pane := panesBySession[session.Name]
		agentState, agentEvent, agentDetail, agentUpdated := rowAgentMetadata(
			agentMetadata{State: pane.AgentState, Event: pane.AgentEvent, Detail: pane.AgentDetail, Updated: pane.AgentUpdated},
			agentMetadata{State: session.AgentState, Event: session.AgentEvent, Detail: session.AgentDetail, Updated: session.AgentUpdated},
		)
		path := session.Path
		if path == "" {
			path = pane.Path
		}
		threadSet[session.Name] = struct{}{}
		rows = append(rows, Row{
			Kind:           RowHeadless,
			AgentID:        session.AgentID,
			AgentName:      agentName,
			AgentSymbol:    agentSymbol,
			AgentState:     agentState,
			AgentEvent:     agentEvent,
			AgentDetail:    agentDetail,
			AgentUpdated:   agentUpdated,
			Title:          firstNonEmpty(session.ThreadTitle, pane.Title),
			TitleOverride:  session.ThreadTitle != "",
			ThreadTitle:    session.ThreadTitle,
			PaneTitle:      pane.Title,
			SessionName:    session.Name,
			WindowIndex:    pane.WindowIndex,
			PaneIndex:      pane.PaneIndex,
			PaneID:         pane.ID,
			PanePID:        pane.PID,
			AgentSessionID: firstNonEmpty(session.AgentSessionID, pane.AgentSessionID),
			Path:           path,
			Attached:       session.Attached,
			Activity:       session.Activity,
		})
	}

	byCommand := agents.CommandMap()
	for _, pane := range panes {
		if _, ok := threadSet[pane.SessionName]; ok {
			continue
		}
		agent, ok := byCommand[pane.Command]
		if !ok {
			continue
		}
		rows = append(rows, Row{
			Kind:           RowEphemeral,
			AgentID:        agent.ID,
			AgentName:      agent.Name,
			AgentSymbol:    agent.Symbol,
			AgentState:     normalizeAgentState(pane.AgentState, pane.AgentUpdated),
			AgentEvent:     pane.AgentEvent,
			AgentDetail:    pane.AgentDetail,
			AgentUpdated:   pane.AgentUpdated,
			Title:          pane.Title,
			SessionName:    pane.SessionName,
			WindowIndex:    pane.WindowIndex,
			PaneIndex:      pane.PaneIndex,
			PaneID:         pane.ID,
			PanePID:        pane.PID,
			AgentSessionID: pane.AgentSessionID,
			Path:           pane.Path,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		if rows[i].Activity != rows[j].Activity {
			return rows[i].Activity > rows[j].Activity
		}
		return rows[i].SessionName < rows[j].SessionName
	})
	return rows
}

func firstPaneBySession(panes []tmux.Pane) map[string]tmux.Pane {
	bySession := make(map[string]tmux.Pane, len(panes))
	for _, pane := range panes {
		if _, ok := bySession[pane.SessionName]; ok {
			continue
		}
		bySession[pane.SessionName] = pane
	}
	return bySession
}

func agentDisplayParts(agentID string) (string, string) {
	if agent, ok := agents.Find(agentID); ok {
		return agent.Name, agent.Symbol
	}
	if agentID == "" {
		return "Agent", ""
	}
	return agentID, ""
}

type agentMetadata struct {
	State   string
	Event   string
	Detail  string
	Updated int64
}

func normalizeAgentState(state string, updated int64) string {
	switch state {
	case "working":
		if updated == 0 {
			return "idle"
		}
		if time.Since(time.UnixMilli(updated)) > 2*time.Hour {
			return "idle"
		}
		return state
	case "input", "permission", "error", "idle":
		return state
	default:
		return "idle"
	}
}

func rowAgentMetadata(records ...agentMetadata) (string, string, string, int64) {
	for _, record := range records {
		state := normalizeAgentState(record.State, record.Updated)
		if state != "idle" || record.State == "idle" {
			return state, record.Event, record.Detail, record.Updated
		}
	}
	return "idle", "", "", 0
}

func tickCmd() tea.Cmd {
	return tea.Tick(140*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func openRowCmd(row Row) tea.Cmd {
	if row.Kind == RowHeadless {
		return func() tea.Msg {
			return messages.SwitchSessionMsg{Name: row.SessionName}
		}
	}
	target := fmt.Sprintf("%s:%d.%d", row.SessionName, row.WindowIndex, row.PaneIndex)
	return func() tea.Msg {
		return messages.SwitchWindowMsg{Target: target}
	}
}

func killHeadlessCmd(sessionName string) tea.Cmd {
	return func() tea.Msg {
		_ = tmux.KillSession(sessionName)
		return loadCmd()()
	}
}

func relaunchHeadlessCmd(row Row) tea.Cmd {
	return func() tea.Msg {
		if row.Kind != RowHeadless {
			return loadCmd()()
		}
		sessionID := strings.TrimSpace(row.AgentSessionID)
		if sessionID == "" {
			resolvedID, err := agentresume.ResolveSessionID(agentresume.Target{
				AgentID: row.AgentID,
				PanePID: row.PanePID,
			})
			if err != nil {
				return loadCmd()()
			}
			sessionID = resolvedID
			_ = tmux.SetAgentSessionID(row.SessionName, sessionID)
		}
		resumeCommand, err := agentresume.ResumeCommand(row.AgentID, sessionID)
		if err != nil {
			return loadCmd()()
		}
		target := rowPaneTarget(row)
		command := agentthread.ThreadCommand(row.AgentID, row.SessionName, resumeCommand)
		if err := tmux.RespawnPaneInDir(target, row.Path, command); err != nil {
			return loadCmd()()
		}
		_ = agentthread.ApplySupport(agentthread.SupportSpec{
			SessionName:  row.SessionName,
			TargetPane:   target,
			AgentID:      row.AgentID,
			InitialTitle: rowTitle(row),
		}, agentthread.DefaultOps())
		return loadCmd()()
	}
}

func renameRowCmd(row Row, title string) tea.Cmd {
	return func() tea.Msg {
		_ = renameRow(row, title)
		return loadCmd()()
	}
}

func renameRow(row Row, title string) error {
	switch row.Kind {
	case RowHeadless:
		if err := syncThreadTitle(row.SessionName, title); err != nil {
			return err
		}
		if title != "" {
			_ = syncPaneTitle(rowPaneTarget(row), title)
		}
		if prefix := staticThreadTitlePrefix(row); prefix != "" {
			_ = syncThreadPrefix(row.SessionName, prefix)
		}
		_ = refreshThreadClient(row.SessionName)
		_ = agentrename.Rename(agentrename.Target{
			AgentID: row.AgentID,
			PanePID: row.PanePID,
		}, title)
	case RowEphemeral:
		return syncPaneTitle(rowPaneTarget(row), title)
	}
	return nil
}

func rowPaneTarget(row Row) string {
	if row.PaneID != "" {
		return row.PaneID
	}
	return fmt.Sprintf("%s:%d.%d", row.SessionName, row.WindowIndex, row.PaneIndex)
}

func newHeadlessCmd(agent agents.Agent, launchDir string) tea.Cmd {
	return func() tea.Msg {
		dir := resolveLaunchDir(launchDir)
		resolved, err := createThread(agentthread.Spec{AgentID: agent.ID, Dir: dir}, agentthread.DefaultOps())
		if err != nil {
			return loadedMsg{}
		}
		return messages.SwitchSessionMsg{Name: resolved.SessionName}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
