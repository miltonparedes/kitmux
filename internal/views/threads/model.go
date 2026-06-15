package threads

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	Kind         RowKind
	AgentID      string
	AgentName    string
	AgentSymbol  string
	AgentState   string
	AgentEvent   string
	AgentDetail  string
	AgentUpdated int64
	Title        string
	SessionName  string
	WindowIndex  int
	PaneIndex    int
	Path         string
	Attached     bool
	Activity     int64
}

type Model struct {
	rows         []Row
	agents       []agents.Agent
	cursor       int
	scroll       int
	height       int
	width        int
	picking      bool
	agentIndex   int
	spinnerFrame int
}

type loadedMsg struct {
	rows []Row
}

type tickMsg struct{}

const refreshEveryFrames = 6

func New() Model {
	return Model{agents: agents.DefaultAgents()}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(syncSupportAndLoadCmd(), tickCmd())
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m Model) IsEditing() bool {
	return m.picking
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.rows = msg.rows
		m.clampCursor()
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
	row := msg.Y - 1
	if row < 0 {
		return m, nil
	}
	idx := m.scroll + row
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
	case "r":
		return m, loadCmd(), true
	case "esc", "q":
		return m, tea.Quit, true
	}
	return m, nil, false
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
			return m, newHeadlessCmd(m.agents[m.agentIndex])
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
	viewHeight := m.height - 2
	if viewHeight < 1 {
		viewHeight = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+viewHeight {
		m.scroll = m.cursor - viewHeight + 1
	}
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
	return loadedMsg{rows: buildRows(sessions, panes)}
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
			Kind:         RowHeadless,
			AgentID:      session.AgentID,
			AgentName:    agentName,
			AgentSymbol:  agentSymbol,
			AgentState:   agentState,
			AgentEvent:   agentEvent,
			AgentDetail:  agentDetail,
			AgentUpdated: agentUpdated,
			Title:        pane.Title,
			SessionName:  session.Name,
			Path:         path,
			Attached:     session.Attached,
			Activity:     session.Activity,
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
			Kind:         RowEphemeral,
			AgentID:      agent.ID,
			AgentName:    agent.Name,
			AgentSymbol:  agent.Symbol,
			AgentState:   normalizeAgentState(pane.AgentState, pane.AgentUpdated),
			AgentEvent:   pane.AgentEvent,
			AgentDetail:  pane.AgentDetail,
			AgentUpdated: pane.AgentUpdated,
			Title:        pane.Title,
			SessionName:  pane.SessionName,
			WindowIndex:  pane.WindowIndex,
			PaneIndex:    pane.PaneIndex,
			Path:         pane.Path,
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

func newHeadlessCmd(agent agents.Agent) tea.Cmd {
	return func() tea.Msg {
		dir, err := tmux.CurrentPanePath()
		if err != nil || dir == "" {
			if cwd, cwdErr := filepath.Abs("."); cwdErr == nil {
				dir = cwd
			}
		}
		resolved, err := agentthread.Ensure(agentthread.Spec{AgentID: agent.ID, Dir: dir}, agentthread.DefaultOps())
		if err != nil {
			return loadedMsg{}
		}
		return messages.SwitchSessionMsg{Name: resolved.SessionName}
	}
}
