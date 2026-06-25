package threads

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/miltonparedes/kitmux/internal/agentlaunch"
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
	Kind              RowKind
	AgentID           string
	AgentName         string
	AgentSymbol       string
	AgentState        string
	AgentEvent        string
	AgentDetail       string
	AgentUpdated      int64
	AgentTitlePrefix  string
	AgentTitleDisplay string
	Title             string
	TitleOverride     bool
	ThreadTitle       string
	PaneTitle         string
	SessionName       string
	WindowIndex       int
	PaneIndex         int
	PaneID            string
	PanePID           int
	AgentSessionID    string
	Path              string
	PanePath          string
	Project           string
	Branch            string
	Attached          bool
	Activity          int64
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
	filterDir    string
	showAll      bool
}

type loadedMsg struct {
	rows []Row
}

type tickMsg struct{}

const refreshEveryFrames = 6

const (
	agentStateWorking    = "working"
	agentStateInput      = "input"
	agentStatePermission = "permission"
	agentStateError      = "error"
	agentStateIdle       = "idle"
)

var (
	lookupAgentTitle    = agentrename.Title
	syncThreadTitle     = tmux.SetThreadTitle
	syncThreadPrefix    = tmux.SetThreadTitlePrefix
	syncPaneTitle       = tmux.SetPaneTitle
	refreshThreadClient = tmux.RefreshClients
	createThread        = agentthread.Create
	installThreadHooks  = agentlaunch.InstallHooks
	resolveAgentSession = agentresume.ResolveSessionID
	persistAgentSession = tmux.SetAgentSessionID
	resumeAgentCommand  = agentresume.ResumeCommand
	wrapThreadCommand   = agentthread.ThreadCommand
	respawnThreadPane   = tmux.RespawnPaneInDir
	applyThreadSupport  = agentthread.ApplySupport
)

func New(launchDir ...string) Model {
	ri := textinput.New()
	ri.Prompt = "Rename: "
	ri.CharLimit = 96
	dir := resolveLaunchDir(launchDir...)
	return Model{
		agents:      agents.DefaultAgents(),
		renameInput: ri,
		launchDir:   dir,
		filterDir:   dir,
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
	return tea.Batch(m.syncSupportAndLoadCmd(), tickCmd())
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) SetShowAll(showAll bool) {
	m.showAll = showAll
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
			return m, tea.Batch(tickCmd(), m.loadCmd())
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
			return m, killHeadlessCmd(row.SessionName, m.loadOptions()), true
		}
		return m, nil, true
	case "ctrl+r":
		return m, m.loadCmd(), true
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
			return m, relaunchHeadlessCmd(*row, m.loadOptions()), true
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
		return m, renameRowCmd(*row, title, m.loadOptions())
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
			return m, newHeadlessCmd(m.agents[m.agentIndex], m.launchDir, m.loadOptions())
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

type loadOptions struct {
	filterDir string
	showAll   bool
}

func (m Model) loadOptions() loadOptions {
	return loadOptions{filterDir: m.filterDir, showAll: m.showAll}
}

func (m Model) loadCmd() tea.Cmd {
	return loadCmd(m.loadOptions())
}

func loadCmd(opts ...loadOptions) tea.Cmd {
	return func() tea.Msg {
		return loadRows(opts...)
	}
}

func (m Model) syncSupportAndLoadCmd() tea.Cmd {
	return syncSupportAndLoadCmd(m.loadOptions())
}

func syncSupportAndLoadCmd(opts ...loadOptions) tea.Cmd {
	return func() tea.Msg {
		_, _ = agentthread.InstallAllSupport(agentthread.DefaultOps())
		return loadRows(opts...)
	}
}

func loadRows(opts ...loadOptions) loadedMsg {
	sessions, _ := tmux.ListSessions()
	panes, _ := tmux.ListPanes()
	return loadedMsg{rows: prepareRows(sessions, panes, opts...)}
}

func prepareRows(sessions []tmux.Session, panes []tmux.Pane, opts ...loadOptions) []Row {
	rows := buildRows(sessions, panes)
	rows = reconcilePaneTitleRenames(rows)
	rows = enrichAgentTitles(rows)
	rows = repairThreadTitlePrefixes(rows)
	rows = enrichGitMeta(rows)
	if len(opts) > 0 {
		rows = filterRows(rows, opts[0])
	}
	return rows
}

func filterRows(rows []Row, opts loadOptions) []Row {
	if opts.showAll || strings.TrimSpace(opts.filterDir) == "" {
		return rows
	}
	dir := canonicalDir(opts.filterDir)
	if dir == "" {
		return rows
	}
	filtered := make([]Row, 0, len(rows))
	for _, row := range rows {
		if rowMatchesDir(row, dir) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func rowMatchesDir(row Row, dir string) bool {
	for _, path := range []string{row.Path, row.PanePath} {
		if canonicalDir(path) == dir {
			return true
		}
	}
	return false
}

func canonicalDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path)
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
		prefix := staticThreadTitlePrefix(rows[i])
		_ = syncThreadTitleState(row.SessionName, threadTitleState{
			title:         title,
			setTitle:      true,
			currentTitle:  row.ThreadTitle,
			prefix:        prefix,
			setPrefix:     prefix != "",
			currentPrefix: row.AgentTitlePrefix,
		})
		if prefix != "" {
			rows[i].AgentTitlePrefix = prefix
		}
	}
	return rows
}

func livePaneThreadTitle(row Row) string {
	title := strings.TrimSpace(stripLeadingStatusGlyph(row.PaneTitle))
	if title == "" || isDefaultPaneTitle(row, title) || isTransientAgentTitle(title) {
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

func isTransientAgentTitle(title string) bool {
	title = strings.ToLower(strings.TrimSpace(title))
	return strings.HasPrefix(title, "worker:") || strings.HasPrefix(title, "subagent:")
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
			prefix := staticThreadTitlePrefix(rows[i])
			_ = syncThreadTitleState(row.SessionName, threadTitleState{
				title:         title,
				setTitle:      true,
				currentTitle:  row.ThreadTitle,
				prefix:        prefix,
				setPrefix:     prefix != "",
				currentPrefix: row.AgentTitlePrefix,
			})
			if prefix != "" {
				rows[i].AgentTitlePrefix = prefix
			}
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
		_ = syncThreadTitleState(row.SessionName, threadTitleState{
			prefix:        prefix,
			setPrefix:     true,
			currentPrefix: row.AgentTitlePrefix,
		})
		rows[i].AgentTitlePrefix = prefix
	}
	return rows
}

type threadTitleState struct {
	title         string
	setTitle      bool
	currentTitle  string
	prefix        string
	setPrefix     bool
	currentPrefix string
}

func syncThreadTitleState(sessionName string, state threadTitleState) error {
	if sessionName == "" {
		return nil
	}
	changed := false
	if state.setTitle && state.title != state.currentTitle {
		if err := syncThreadTitle(sessionName, state.title); err != nil {
			return err
		}
		changed = true
	}
	if state.setPrefix && state.prefix != state.currentPrefix {
		_ = syncThreadPrefix(sessionName, state.prefix)
		changed = true
	}
	if changed {
		_ = refreshThreadClient(sessionName)
	}
	return nil
}

func staticThreadTitlePrefix(row Row) string {
	if row.AgentState != "" && row.AgentState != agentStateIdle {
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
			agentMetadata{State: session.AgentState, Event: session.AgentEvent, Detail: session.AgentDetail, Updated: session.AgentUpdated},
			agentMetadata{State: pane.AgentState, Event: pane.AgentEvent, Detail: pane.AgentDetail, Updated: pane.AgentUpdated},
		)
		path := session.Path
		if path == "" {
			path = pane.Path
		}
		threadSet[session.Name] = struct{}{}
		rows = append(rows, Row{
			Kind:              RowHeadless,
			AgentID:           session.AgentID,
			AgentName:         agentName,
			AgentSymbol:       agentSymbol,
			AgentState:        agentState,
			AgentEvent:        agentEvent,
			AgentDetail:       agentDetail,
			AgentUpdated:      agentUpdated,
			AgentTitlePrefix:  firstNonEmpty(session.AgentTitlePrefix, pane.AgentTitlePrefix),
			AgentTitleDisplay: firstNonEmpty(session.AgentTitleDisplay, pane.AgentTitleDisplay),
			Title:             firstNonEmpty(session.ThreadTitle, pane.Title),
			TitleOverride:     session.ThreadTitle != "",
			ThreadTitle:       session.ThreadTitle,
			PaneTitle:         pane.Title,
			SessionName:       session.Name,
			WindowIndex:       pane.WindowIndex,
			PaneIndex:         pane.PaneIndex,
			PaneID:            pane.ID,
			PanePID:           pane.PID,
			AgentSessionID:    canonicalAgentSessionID(session.AgentID, firstNonEmpty(session.AgentSessionID, pane.AgentSessionID)),
			Path:              path,
			PanePath:          pane.Path,
			Attached:          session.Attached,
			Activity:          session.Activity,
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
			Kind:              RowEphemeral,
			AgentID:           agent.ID,
			AgentName:         agent.Name,
			AgentSymbol:       agent.Symbol,
			AgentState:        normalizeAgentState(pane.AgentState, pane.AgentUpdated),
			AgentEvent:        pane.AgentEvent,
			AgentDetail:       pane.AgentDetail,
			AgentUpdated:      pane.AgentUpdated,
			AgentTitlePrefix:  pane.AgentTitlePrefix,
			AgentTitleDisplay: pane.AgentTitleDisplay,
			Title:             pane.Title,
			SessionName:       pane.SessionName,
			WindowIndex:       pane.WindowIndex,
			PaneIndex:         pane.PaneIndex,
			PaneID:            pane.ID,
			PanePID:           pane.PID,
			AgentSessionID:    canonicalAgentSessionID(agent.ID, pane.AgentSessionID),
			Path:              pane.Path,
			PanePath:          pane.Path,
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
	case agentStateWorking:
		if updated == 0 {
			return agentStateIdle
		}
		if time.Since(time.UnixMilli(updated)) > 2*time.Hour {
			return agentStateIdle
		}
		return state
	case agentStateInput, agentStatePermission, agentStateError, agentStateIdle:
		return state
	default:
		return agentStateIdle
	}
}

func rowAgentMetadata(records ...agentMetadata) (string, string, string, int64) {
	var best agentMetadata
	found := false
	for _, record := range records {
		if record.State == "" {
			continue
		}
		state := normalizeAgentState(record.State, record.Updated)
		if state == agentStateIdle && record.State != agentStateIdle {
			continue
		}
		candidate := agentMetadata{
			State:   state,
			Event:   record.Event,
			Detail:  record.Detail,
			Updated: record.Updated,
		}
		if !found || newerAgentMetadata(candidate, best) {
			best = candidate
			found = true
		}
	}
	if found {
		return best.State, best.Event, best.Detail, best.Updated
	}
	return agentStateIdle, "", "", 0
}

func newerAgentMetadata(candidate, current agentMetadata) bool {
	if candidate.Updated == current.Updated {
		return agentStatePriority(candidate.State) > agentStatePriority(current.State)
	}
	if candidate.Updated == 0 {
		return false
	}
	if current.Updated == 0 {
		return true
	}
	return candidate.Updated > current.Updated
}

func agentStatePriority(state string) int {
	switch state {
	case agentStateError:
		return 5
	case agentStatePermission:
		return 4
	case agentStateInput:
		return 3
	case agentStateIdle:
		return 2
	case agentStateWorking:
		return 1
	default:
		return 0
	}
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

func killHeadlessCmd(sessionName string, opts loadOptions) tea.Cmd {
	return func() tea.Msg {
		_ = tmux.KillSession(sessionName)
		return loadCmd(opts)()
	}
}

func relaunchHeadlessCmd(row Row, opts loadOptions) tea.Cmd {
	return func() tea.Msg {
		_ = relaunchHeadless(row)
		return loadCmd(opts)()
	}
}

func relaunchHeadless(row Row) error {
	if row.Kind != RowHeadless {
		return nil
	}
	sessionID := canonicalAgentSessionID(row.AgentID, row.AgentSessionID)
	if sessionID == "" {
		resolvedID, err := resolveAgentSession(agentresume.Target{
			AgentID: row.AgentID,
			PanePID: row.PanePID,
		})
		if err != nil {
			return err
		}
		sessionID = canonicalAgentSessionID(row.AgentID, resolvedID)
	}
	_ = persistAgentSession(row.SessionName, sessionID)
	resumeCommand, err := resumeAgentCommand(row.AgentID, sessionID)
	if err != nil {
		return err
	}
	target := rowPaneTarget(row)
	command := wrapThreadCommand(row.AgentID, row.SessionName, resumeCommand)
	if err := respawnThreadPane(target, row.Path, command); err != nil {
		return err
	}
	_ = applyThreadSupport(agentthread.SupportSpec{
		SessionName:  row.SessionName,
		TargetPane:   target,
		AgentID:      row.AgentID,
		InitialTitle: rowTitle(row),
	}, agentthread.DefaultOps())
	return nil
}

func canonicalAgentSessionID(agentID, sessionID string) string {
	return agentresume.CanonicalSessionID(agentID, sessionID, "")
}

func renameRowCmd(row Row, title string, opts loadOptions) tea.Cmd {
	return func() tea.Msg {
		_ = renameRow(row, title)
		return loadCmd(opts)()
	}
}

func renameRow(row Row, title string) error {
	switch row.Kind {
	case RowHeadless:
		prefix := staticThreadTitlePrefix(row)
		if err := syncThreadTitleState(row.SessionName, threadTitleState{
			title:         title,
			setTitle:      true,
			currentTitle:  row.ThreadTitle,
			prefix:        prefix,
			setPrefix:     prefix != "",
			currentPrefix: row.AgentTitlePrefix,
		}); err != nil {
			return err
		}
		if title != "" {
			_ = syncPaneTitle(rowPaneTarget(row), title)
		}
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

func newHeadlessCmd(agent agents.Agent, launchDir string, opts ...loadOptions) tea.Cmd {
	return func() tea.Msg {
		if err := installThreadHooks(agent.ID); err != nil {
			_ = tmux.DisplayMessage(fmt.Sprintf("create thread: %v", err))
			return loadRows(opts...)
		}
		dir := resolveLaunchDir(launchDir)
		resolved, err := createThread(agentthread.Spec{AgentID: agent.ID, Dir: dir}, agentthread.DefaultOps())
		if err != nil {
			_ = tmux.DisplayMessage(fmt.Sprintf("create thread: %v", err))
			return loadRows(opts...)
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
