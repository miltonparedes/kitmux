package sessions

import (
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/miltonparedes/kitmux/internal/app/messages"
	"github.com/miltonparedes/kitmux/internal/cache"
	"github.com/miltonparedes/kitmux/internal/config"
	"github.com/miltonparedes/kitmux/internal/tmux"
	wsdata "github.com/miltonparedes/kitmux/internal/workspaces/data"
)

// Model is the sessions tree view.
type Model struct {
	roots   []*TreeNode
	visible []*TreeNode // flattened visible nodes
	cursor  int
	scroll  int
	height  int
	width   int

	confirming  bool // kill confirmation
	renaming    bool
	renameInput textinput.Model
	searching   bool
	searchInput textinput.Model
	picking     bool // zoxide directory picker active
	picker      zoxidePicker
	justLoaded  bool // set on sessionsLoadedMsg, cleared by ConsumeLoaded
}

func New() Model {
	ri := textinput.New()
	ri.Prompt = "Rename: "
	ri.CharLimit = 64

	si := textinput.New()
	si.Prompt = "/ "
	si.Placeholder = "search sessions..."
	si.CharLimit = 128

	return Model{
		renameInput: ri,
		searchInput: si,
		picker:      newZoxidePicker(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadSessionsCached()
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// IsEditing returns true when the user is in an input mode (rename, confirm, picking).
func (m Model) IsEditing() bool {
	return m.confirming || m.renaming || m.searching || m.picking
}

// SelectedSessionName returns the session name under the cursor, or empty string.
func (m Model) SelectedSessionName() string {
	if node := m.selected(); node != nil && node.Kind == KindSession {
		return node.SessionName
	}
	return ""
}

type sessionStats struct {
	Added   int
	Deleted int
}

type sessionsLoadedMsg struct {
	sessions  []tmux.Session
	repoRoots map[string]string
}

type statsLoadedMsg struct {
	stats map[string]sessionStats
}

// cachedSnapshotMsg delivers a cached snapshot for immediate display.
type cachedSnapshotMsg struct {
	sessions  []tmux.Session
	repoRoots map[string]string
	stats     map[string]sessionStats
}

func (m Model) loadSessions() tea.Msg {
	sessions, err := tmux.ListSessions()
	if err != nil {
		return sessionsLoadedMsg{}
	}
	snap := cache.Load()
	repoRoots, repoRootsRefreshedAt := resolveRepoRootsIncremental(sessions, snap, time.Now())

	_ = cache.Update(func(curr *cache.Snapshot) {
		curr.Sessions = sessions
		curr.RepoRoots = repoRoots
		curr.RepoRootsRefreshedAt = repoRootsRefreshedAt
	})

	return sessionsLoadedMsg{sessions: sessions, repoRoots: repoRoots}
}

// loadSessionsCached emits a cached snapshot first (if available), then
// returns a command that reconciles with the live tmux state.
func (m Model) loadSessionsCached() tea.Cmd {
	snap := cache.Load()
	if snap == nil || len(snap.Sessions) == 0 {
		return m.loadSessions
	}

	// Prefer the shared SQLite-backed workspace stats keyed by worktree
	// path — that cache is kept fresh by both the dashboard and the
	// background refresh below, and survives restart. Fall back to the
	// session-name-keyed legacy snapshot only when the shared cache is
	// empty (first run after upgrade).
	cachedStats := sharedStatsForSessions(snap.Sessions)
	if len(cachedStats) == 0 && snap.StatsValid() {
		cachedStats = make(map[string]sessionStats, len(snap.Stats))
		for k, v := range snap.Stats {
			cachedStats[k] = sessionStats{Added: v.Added, Deleted: v.Deleted}
		}
	}

	return func() tea.Msg {
		return cachedSnapshotMsg{
			sessions:  snap.Sessions,
			repoRoots: snap.RepoRoots,
			stats:     cachedStats,
		}
	}
}

// sharedStatsForSessions maps each session name to its cached diff stats by
// looking up the session's working directory in the shared workspace_stats
// table. Returns a possibly-empty map on any error.
func sharedStatsForSessions(sessions []tmux.Session) map[string]sessionStats {
	if len(sessions) == 0 {
		return nil
	}
	byPath, err := sharedStatsService().LoadCachedByWorktreePath()
	if err != nil || len(byPath) == 0 {
		return nil
	}
	out := make(map[string]sessionStats, len(sessions))
	for _, s := range sessions {
		if s.Path == "" {
			continue
		}
		if wt, ok := byPath[s.Path]; ok && (wt.Added > 0 || wt.Deleted > 0) {
			out[s.Name] = sessionStats{Added: wt.Added, Deleted: wt.Deleted}
		}
	}
	return out
}

// refreshWorktreeStats repopulates the shared workspace_stats cache for
// every unique repo root backing the current sessions, running `wt list`
// in parallel (single-flighted per path by the service). The returned map
// is the per-session diff summary used by the sessions tree.
func refreshWorktreeStats(sessions []tmux.Session, repoRoots map[string]string) map[string]sessionStats {
	pathToName := make(map[string]string, len(sessions))
	for _, s := range sessions {
		if s.Path != "" {
			pathToName[s.Path] = s.Name
		}
	}

	uniqueRoots := uniqueRepoRoots(repoRoots)
	if len(uniqueRoots) == 0 {
		return nil
	}

	svc := sharedStatsService()
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out = make(map[string]sessionStats)
	)
	for root := range uniqueRoots {
		wg.Add(1)
		go func(root string) {
			defer wg.Done()
			res := svc.Refresh(root)
			if res.Err != nil {
				return
			}
			collectWorktreeStats(res.Stats.Worktrees, pathToName, out, &mu)
		}(root)
	}
	wg.Wait()
	return out
}

func uniqueRepoRoots(repoRoots map[string]string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, root := range repoRoots {
		if root != "" {
			out[root] = struct{}{}
		}
	}
	return out
}

func collectWorktreeStats(
	worktrees []wsdata.WorktreeStat,
	pathToName map[string]string,
	out map[string]sessionStats,
	mu *sync.Mutex,
) {
	mu.Lock()
	defer mu.Unlock()
	for _, wt := range worktrees {
		name, ok := pathToName[wt.WorktreePath]
		if !ok {
			continue
		}
		if wt.Added > 0 || wt.Deleted > 0 {
			out[name] = sessionStats{Added: wt.Added, Deleted: wt.Deleted}
		}
	}
}

// sharedStatsService returns a process-wide StatsService so sessions and the
// workspaces dashboard coalesce concurrent `wt list` invocations. Exported
// hooks (e.g. tests) may override it.
var (
	statsSvcOnce sync.Once
	statsSvc     *wsdata.StatsService
)

func sharedStatsService() *wsdata.StatsService {
	statsSvcOnce.Do(func() {
		statsSvc = wsdata.NewStatsService()
	})
	return statsSvc
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cachedSnapshotMsg:
		return m.handleCachedSnapshot(msg)
	case sessionsLoadedMsg:
		return m.handleSessionsLoaded(msg)
	case statsLoadedMsg:
		applyStats(m.roots, msg.stats)
		return m, nil
	case zoxideEntriesLoadedMsg:
		m.picker.setEntries(msg.entries)
		return m, nil
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.routeKey(msg)
	}
	return m, nil
}

func (m Model) handleCachedSnapshot(msg cachedSnapshotMsg) (Model, tea.Cmd) {
	m.roots = BuildTree(msg.sessions, msg.repoRoots)
	m.visible = Flatten(m.roots)
	m.clampCursor()
	if len(msg.stats) > 0 {
		applyStats(m.roots, msg.stats)
	}
	return m, tea.Batch(m.emitCursorChange(), m.loadSessions)
}

func (m Model) handleSessionsLoaded(msg sessionsLoadedMsg) (Model, tea.Cmd) {
	m.roots = BuildTree(msg.sessions, msg.repoRoots)
	if snapStats := sharedStatsForSessions(msg.sessions); len(snapStats) > 0 {
		applyStats(m.roots, snapStats)
	}
	m.visible = Flatten(m.roots)
	m.clampCursor()
	m.justLoaded = true
	sessions := msg.sessions
	repoRoots := msg.repoRoots
	return m, tea.Batch(m.emitCursorChange(), func() tea.Msg {
		return statsLoadedMsg{stats: refreshWorktreeStats(sessions, repoRoots)}
	})
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if m.IsEditing() {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		return m.handleMouseLeft(msg)
	case tea.MouseButtonWheelUp:
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, m.emitCursorChange()
	case tea.MouseButtonWheelDown:
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, m.emitCursorChange()
	}
	return m, nil
}

func (m Model) handleMouseLeft(msg tea.MouseMsg) (Model, tea.Cmd) {
	if msg.Action != tea.MouseActionRelease {
		return m, nil
	}
	row := msg.Y
	if row%2 != 0 {
		return m, nil
	}
	idx := m.scroll + row/2
	if idx < 0 || idx >= len(m.visible) {
		return m, nil
	}
	node := m.visible[idx]
	if len(node.Children) > 0 {
		node.Expanded = !node.Expanded
		m.visible = Flatten(m.roots)
		m.clampCursor()
		return m, nil
	}
	if node.Kind == KindSession {
		return m, switchSessionCmd(node.SessionName)
	}
	return m, nil
}

func (m Model) routeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case m.picking:
		return m.handlePicker(msg)
	case m.searching:
		return m.handleSearch(msg)
	case m.confirming:
		return m.handleConfirm(msg)
	case m.renaming:
		return m.handleRename(msg)
	default:
		return m.handleNormal(msg)
	}
}

func switchSessionCmd(name string) tea.Cmd {
	return func() tea.Msg {
		return messages.SwitchSessionMsg{Name: name}
	}
}

func applyStats(roots []*TreeNode, stats map[string]sessionStats) {
	if len(stats) == 0 {
		return
	}
	for _, r := range roots {
		if st, ok := stats[r.SessionName]; ok {
			r.Added, r.Deleted = st.Added, st.Deleted
		}
		for _, c := range r.Children {
			if st, ok := stats[c.SessionName]; ok {
				c.Added, c.Deleted = st.Added, st.Deleted
			}
		}
	}
}

func (m Model) handleNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	prevCursor := m.cursor

	if updated, handled := m.handleNormalNav(msg); handled {
		if updated.cursor != prevCursor {
			return updated, updated.emitCursorChange()
		}
		return updated, nil
	}
	if updated, cmd, handled := m.handleNormalAction(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.handleDigitJump(msg); handled {
		return updated, cmd
	}
	return m, nil
}

// handleNormalNav handles cursor/scroll navigation keys.
func (m Model) handleNormalNav(msg tea.KeyMsg) (Model, bool) {
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
		m.cursor = len(m.visible) - 1
		m.ensureVisible()
		return m, true
	case "J":
		m.jumpToNextRoot()
		return m, true
	case "K":
		m.jumpToPrevRoot()
		return m, true
	}
	return m, false
}

func (m *Model) jumpToNextRoot() {
	for i := m.cursor + 1; i < len(m.visible); i++ {
		if m.visible[i].Depth == 0 {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
	m.ensureVisible()
}

func (m *Model) jumpToPrevRoot() {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.visible[i].Depth == 0 {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
	m.ensureVisible()
}

// handleNormalAction handles non-navigation keys (enter/space/search/delete/rename/new/open).
func (m Model) handleNormalAction(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		return m.actionEnter()
	case " ":
		return m.actionToggleExpand()
	case "/":
		return m.actionStartSearch()
	case "d":
		return m.actionDelete()
	case "r":
		return m.actionStartRename()
	case "n":
		return m.actionOpenPicker()
	case "ctrl+o":
		return m, openLocalEditorExecuteCmd(), true
	}
	return m, nil, false
}

func (m Model) actionEnter() (Model, tea.Cmd, bool) {
	node := m.selected()
	if node == nil || node.Kind != KindSession {
		return m, nil, true
	}
	return m, switchSessionCmd(node.SessionName), true
}

func (m Model) actionToggleExpand() (Model, tea.Cmd, bool) {
	node := m.selected()
	if node == nil || len(node.Children) == 0 {
		return m, nil, true
	}
	node.Expanded = !node.Expanded
	m.visible = Flatten(m.roots)
	m.clampCursor()
	return m, nil, true
}

func (m Model) actionStartSearch() (Model, tea.Cmd, bool) {
	m.searching = true
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	m.filterSessions()
	return m, textinput.Blink, true
}

func (m Model) actionDelete() (Model, tea.Cmd, bool) {
	if node := m.selected(); node != nil && node.Kind == KindSession {
		m.confirming = true
	}
	return m, nil, true
}

func (m Model) actionStartRename() (Model, tea.Cmd, bool) {
	node := m.selected()
	if node == nil || node.Kind != KindSession {
		return m, nil, true
	}
	m.renaming = true
	m.renameInput.SetValue(node.SessionName)
	m.renameInput.Focus()
	return m, textinput.Blink, true
}

func (m Model) actionOpenPicker() (Model, tea.Cmd, bool) {
	m.picking = true
	m.picker.input.SetValue("")
	m.picker.input.Focus()
	return m, tea.Batch(textinput.Blink, loadZoxideEntries), true
}

func (m Model) handleDigitJump(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
	default:
		return m, nil, false
	}
	if !digitJumpActive(msg) {
		return m, nil, true
	}
	target := int(msg.Runes[0] - '0')
	if name := m.sessionNameAt(target); name != "" {
		return m, switchSessionCmd(name), true
	}
	return m, nil, true
}

func digitJumpActive(msg tea.KeyMsg) bool {
	if config.SuperKey == "none" && !msg.Alt {
		return true
	}
	if config.SuperKey == "alt" && msg.Alt {
		return true
	}
	return false
}

func (m Model) sessionNameAt(ordinal int) string {
	n := 0
	for _, node := range m.visible {
		if node.Kind != KindSession {
			continue
		}
		n++
		if n == ordinal {
			return node.SessionName
		}
	}
	return ""
}

func openLocalEditorExecuteCmd() tea.Cmd {
	return func() tea.Msg {
		return messages.ExecuteCommandMsg{ID: "open_local_editor"}
	}
}

func (m Model) handleConfirm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirming = false
		if node := m.selected(); node != nil && node.Kind == KindSession {
			name := node.SessionName
			return m, func() tea.Msg {
				_ = tmux.KillSession(name)
				// Reload sessions from tmux after kill
				sessions, _ := tmux.ListSessions()
				repoRoots := resolveRepoRoots(sessions)
				return sessionsLoadedMsg{sessions: sessions, repoRoots: repoRoots}
			}
		}
	case "n", "N", "esc":
		m.confirming = false
	}
	return m, nil
}

func (m Model) handleRename(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.renaming = false
		if node := m.selected(); node != nil {
			old := node.SessionName
			newName := m.renameInput.Value()
			if newName != "" && newName != old {
				return m, func() tea.Msg {
					_ = tmux.RenameSession(old, newName)
					return m.loadSessions()
				}
			}
		}
		return m, nil
	case "esc":
		m.renaming = false
		return m, nil
	}
	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m Model) handlePicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if entry := m.picker.selected(); entry != nil {
			m.picking = false
			return m, openZoxideEntry(*entry)
		}
		return m, nil
	case "esc":
		m.picking = false
		return m, nil
	case "up", "ctrl+k":
		m.picker.cursor--
		m.picker.clampCursor()
		m.picker.ensureVisible(m.pickerMaxVisible())
		return m, nil
	case "down", "ctrl+j":
		m.picker.cursor++
		m.picker.clampCursor()
		m.picker.ensureVisible(m.pickerMaxVisible())
		return m, nil
	}

	var cmd tea.Cmd
	m.picker.input, cmd = m.picker.input.Update(msg)
	m.picker.filter()
	return m, cmd
}

func (m Model) handleSearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if node := m.selected(); node != nil && node.Kind == KindSession {
			m.searching = false
			m.visible = Flatten(m.roots)
			m.clampCursor()
			return m, func() tea.Msg {
				return messages.SwitchSessionMsg{Name: node.SessionName}
			}
		}
		return m, nil
	case "esc":
		m.searching = false
		m.visible = Flatten(m.roots)
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case "up", "ctrl+k":
		m.cursor--
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	case "down", "ctrl+j":
		m.cursor++
		m.clampCursor()
		m.ensureVisible()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.filterSessions()
	return m, cmd
}

func (m *Model) filterSessions() {
	allRoots := expandedCopy(m.roots)
	allNodes := Flatten(allRoots)

	// Collect only session nodes
	var sessions []*TreeNode
	for _, n := range allNodes {
		if n.Kind == KindSession {
			sessions = append(sessions, n)
		}
	}

	query := m.searchInput.Value()
	if query == "" {
		// Show all sessions flat
		flat := make([]*TreeNode, len(sessions))
		for i, s := range sessions {
			flat[i] = &TreeNode{
				Kind:        KindSession,
				Name:        s.SessionName,
				SessionName: s.SessionName,
				Windows:     s.Windows,
				Attached:    s.Attached,
				Depth:       0,
			}
		}
		m.visible = flat
	} else {
		names := make([]string, len(sessions))
		for i, s := range sessions {
			names[i] = s.SessionName
		}
		matches := fuzzy.Find(query, names)
		flat := make([]*TreeNode, len(matches))
		for i, match := range matches {
			s := sessions[match.Index]
			flat[i] = &TreeNode{
				Kind:        KindSession,
				Name:        s.SessionName,
				SessionName: s.SessionName,
				Windows:     s.Windows,
				Attached:    s.Attached,
				Depth:       0,
			}
		}
		m.visible = flat
	}
	m.cursor = 0
	m.scroll = 0
}

// expandedCopy returns a shallow copy of roots with all nodes expanded,
// so Flatten returns every node including children of collapsed groups.
func expandedCopy(roots []*TreeNode) []*TreeNode {
	out := make([]*TreeNode, len(roots))
	for i, r := range roots {
		cp := *r
		cp.Expanded = true
		out[i] = &cp
	}
	return out
}

func (m Model) pickerMaxVisible() int {
	avail := m.height - 4 // input + separator + footer separator + help
	if avail < 1 {
		avail = 1
	}
	return (avail + 1) / 2
}

func (m Model) selected() *TreeNode {
	if m.cursor >= 0 && m.cursor < len(m.visible) {
		return m.visible[m.cursor]
	}
	return nil
}

func (m *Model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) ensureVisible() {
	// Each item takes 2 lines (item + separator), last takes 1.
	// Available lines = height - 2 (footer sep + help)
	avail := m.height - 2
	if avail < 1 {
		avail = 1
	}
	maxVisible := (avail + 1) / 2
	if maxVisible < 1 {
		maxVisible = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+maxVisible {
		m.scroll = m.cursor - maxVisible + 1
	}
}

func (m Model) emitCursorChange() tea.Cmd {
	name := m.SelectedSessionName()
	if name == "" {
		return nil
	}
	return func() tea.Msg {
		return messages.SessionCursorMsg{SessionName: name}
	}
}

// Reload triggers a session reload from tmux.
func (m Model) Reload() tea.Cmd {
	return m.loadSessions
}

// HasData returns true when sessions have been loaded.
func (m Model) HasData() bool {
	return len(m.visible) > 0
}

// ConsumeLoaded returns true once after a sessionsLoadedMsg was processed,
// then resets the flag. Use this to gate deferred actions on a fresh reload.
func (m *Model) ConsumeLoaded() bool {
	if m.justLoaded {
		m.justLoaded = false
		return true
	}
	return false
}

// SetPickingMode activates the zoxide directory picker state.
// Use this from a value-receiver context (e.g. Init) where pointer-receiver
// mutations would be lost.
func (m *Model) SetPickingMode() {
	m.picking = true
	m.picker.input.SetValue("")
	m.picker.input.Focus()
}

// ZoxidePickerCmds returns the commands needed for the zoxide picker
// without mutating state.
func (m Model) ZoxidePickerCmds() tea.Cmd {
	return tea.Batch(textinput.Blink, loadZoxideEntries)
}

// InitZoxidePicker activates the zoxide picker and starts loading entries.
func (m *Model) InitZoxidePicker() tea.Cmd {
	m.SetPickingMode()
	return m.ZoxidePickerCmds()
}
