package sessions

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/miltonparedes/kitmux/internal/cache"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

const repoRootsRevalidateTTL = 10 * time.Minute

// NodeKind distinguishes session nodes from virtual group headers.
type NodeKind int

const (
	KindSession     NodeKind = iota
	KindGroupHeader          // virtual group (no real session)
	KindWorktree             // inactive worktree branch (no running session)
)

// TreeNode represents one entry in the session tree.
type TreeNode struct {
	Kind        NodeKind
	Name        string // display name (suffix for children, full for root)
	Path        string
	SessionName string // raw tmux session name (empty for virtual headers)
	Windows     int
	Attached    bool
	Children    []*TreeNode
	Expanded    bool
	Depth       int
	Added       int
	Deleted     int
	Activity    int64
}

// BuildTree groups sessions by git repository root.
//
// Sessions that share the same repo root (including worktrees) are grouped together.
// For groups with 2+ sessions, the display name is the repo directory basename.
// Sessions without a repo root fall back to parent-child grouping by name prefix.
// Sorting: groups ordered by most-recent activity; -main/-master float to top within groups,
// remaining children sorted by activity descending.
func BuildTree(sessions []tmux.Session, repoRoots map[string]string) []*TreeNode {
	if repoRoots == nil {
		repoRoots = make(map[string]string)
	}

	activityOf := make(map[string]int64, len(sessions))
	for _, s := range sessions {
		activityOf[s.Name] = s.Activity
	}

	sorted := sortSessionsForGrouping(sessions)
	sesMap := make(map[string]tmux.Session, len(sorted))
	for _, s := range sorted {
		sesMap[s.Name] = s
	}

	repoGroups, noRepo := splitByRepoRoot(sorted, repoRoots)

	roots := buildRepoRoots(repoGroups)
	roots = append(roots, buildNoRepoRoots(noRepo, sesMap, activityOf)...)

	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].Activity > roots[j].Activity
	})
	return roots
}

// sortSessionsForGrouping returns sessions ordered so that -main/-master sink
// to the top of their prefix group, falling back to activity descending.
func sortSessionsForGrouping(sessions []tmux.Session) []tmux.Session {
	sorted := make([]tmux.Session, len(sessions))
	copy(sorted, sessions)
	sort.SliceStable(sorted, func(i, j int) bool {
		ki, kj := sortKey(sorted[i].Name), sortKey(sorted[j].Name)
		if ki != kj {
			return ki < kj
		}
		return sorted[i].Activity > sorted[j].Activity
	})
	return sorted
}

// splitByRepoRoot partitions sessions between those mapped to a repo root and
// those that have no mapping (handled by the name-prefix fallback).
func splitByRepoRoot(sorted []tmux.Session, repoRoots map[string]string) (map[string][]tmux.Session, []tmux.Session) {
	repoGroups := make(map[string][]tmux.Session)
	var noRepo []tmux.Session
	for _, s := range sorted {
		if root := repoRoots[s.Name]; root != "" {
			repoGroups[root] = append(repoGroups[root], s)
		} else {
			noRepo = append(noRepo, s)
		}
	}
	return repoGroups, noRepo
}

func buildRepoRoots(repoGroups map[string][]tmux.Session) []*TreeNode {
	var roots []*TreeNode
	for _, repoRoot := range sortedKeys(repoGroups) {
		group := repoGroups[repoRoot]
		if len(group) == 1 {
			roots = append(roots, newSessionRoot(group[0]))
			continue
		}
		roots = append(roots, buildRepoGroupNode(repoRoot, group))
	}
	return roots
}

func buildRepoGroupNode(repoRoot string, group []tmux.Session) *TreeNode {
	groupName := filepath.Base(repoRoot)
	rootSession, children := pickRepoGroupRoot(group, groupName)

	sort.SliceStable(children, func(i, j int) bool {
		mi, mj := isMainBranch(children[i].Name), isMainBranch(children[j].Name)
		if mi != mj {
			return mi
		}
		return children[i].Activity > children[j].Activity
	})

	parent := newRepoGroupParent(groupName, rootSession)
	for _, cs := range children {
		parent.Children = append(parent.Children, &TreeNode{
			Kind:        KindSession,
			Name:        trimNormalizedPrefix(cs.Name, groupName),
			SessionName: cs.Name,
			Windows:     cs.Windows,
			Attached:    cs.Attached,
			Activity:    cs.Activity,
			Depth:       1,
		})
		if cs.Activity > parent.Activity {
			parent.Activity = cs.Activity
		}
	}
	return parent
}

func pickRepoGroupRoot(group []tmux.Session, groupName string) (*tmux.Session, []tmux.Session) {
	var rootSession *tmux.Session
	var children []tmux.Session
	for i := range group {
		if normalize(group[i].Name) == normalize(groupName) {
			rootSession = &group[i]
		} else {
			children = append(children, group[i])
		}
	}
	return rootSession, children
}

func newRepoGroupParent(groupName string, rootSession *tmux.Session) *TreeNode {
	if rootSession != nil {
		return &TreeNode{
			Kind:        KindSession,
			Name:        groupName,
			SessionName: rootSession.Name,
			Windows:     rootSession.Windows,
			Attached:    rootSession.Attached,
			Activity:    rootSession.Activity,
			Expanded:    true,
			Depth:       0,
		}
	}
	return &TreeNode{
		Kind:     KindGroupHeader,
		Name:     groupName,
		Expanded: true,
		Depth:    0,
	}
}

func newSessionRoot(s tmux.Session) *TreeNode {
	return &TreeNode{
		Kind:        KindSession,
		Name:        s.Name,
		SessionName: s.Name,
		Windows:     s.Windows,
		Attached:    s.Attached,
		Activity:    s.Activity,
		Depth:       0,
	}
}

// buildNoRepoRoots groups sessions without a repo root using name-prefix heuristics.
func buildNoRepoRoots(noRepo []tmux.Session, sesMap map[string]tmux.Session, activityOf map[string]int64) []*TreeNode {
	parentOf, childrenOf := computeNoRepoParents(noRepo)
	processed := make(map[string]bool)
	var roots []*TreeNode
	for _, s := range noRepo {
		if processed[s.Name] || parentOf[s.Name] != "" {
			continue
		}
		if len(childrenOf[s.Name]) > 0 {
			roots = append(roots, buildNoRepoParent(s, childrenOf[s.Name], sesMap, activityOf, processed))
			processed[s.Name] = true
			continue
		}
		roots = append(roots, newSessionRoot(s))
		processed[s.Name] = true
	}
	return roots
}

func computeNoRepoParents(noRepo []tmux.Session) (map[string]string, map[string][]string) {
	normMap := make(map[string]string, len(noRepo))
	nameSet := make(map[string]bool, len(noRepo))
	for _, s := range noRepo {
		norm := normalize(s.Name)
		nameSet[norm] = true
		if _, exists := normMap[norm]; !exists {
			normMap[norm] = s.Name
		}
	}
	parentOf := make(map[string]string)
	childrenOf := make(map[string][]string)
	for _, s := range noRepo {
		normParent := findRealParent(normalize(s.Name), nameSet)
		if normParent != "" {
			origParent := normMap[normParent]
			parentOf[s.Name] = origParent
			childrenOf[origParent] = append(childrenOf[origParent], s.Name)
		}
	}
	return parentOf, childrenOf
}

func buildNoRepoParent(
	s tmux.Session,
	cnames []string,
	sesMap map[string]tmux.Session,
	activityOf map[string]int64,
	processed map[string]bool,
) *TreeNode {
	node := &TreeNode{
		Kind:        KindSession,
		Name:        s.Name,
		SessionName: s.Name,
		Windows:     s.Windows,
		Attached:    s.Attached,
		Activity:    s.Activity,
		Expanded:    true,
		Depth:       0,
	}
	sort.SliceStable(cnames, func(i, j int) bool {
		return activityOf[cnames[i]] > activityOf[cnames[j]]
	})
	for _, cname := range cnames {
		cs := sesMap[cname]
		node.Children = append(node.Children, &TreeNode{
			Kind:        KindSession,
			Name:        trimNormalizedPrefix(cname, normalize(s.Name)),
			SessionName: cname,
			Windows:     cs.Windows,
			Attached:    cs.Attached,
			Activity:    cs.Activity,
			Depth:       1,
		})
		if cs.Activity > node.Activity {
			node.Activity = cs.Activity
		}
		processed[cname] = true
	}
	return node
}

// sortedKeys returns map keys sorted alphabetically (used as fallback grouping order
// before the final activity-based sort).
func sortedKeys(m map[string][]tmux.Session) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isMainBranch returns true if the session name ends with -main or -master.
func isMainBranch(name string) bool {
	norm := normalize(name)
	return strings.HasSuffix(norm, "-main") || strings.HasSuffix(norm, "-master")
}

// Flatten returns the visible (expanded) nodes in order.
func Flatten(roots []*TreeNode) []*TreeNode {
	var out []*TreeNode
	for _, r := range roots {
		out = append(out, r)
		if r.Expanded {
			out = append(out, r.Children...)
		}
	}
	return out
}

// normalize replaces _ and spaces with - so grouping treats all separators equally.
func normalize(name string) string {
	s := strings.ReplaceAll(name, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// trimNormalizedPrefix removes a normalized prefix from a raw session name.
// It works because _, space, and - are all single-byte characters, so the
// normalized prefix length matches the raw prefix length.
func trimNormalizedPrefix(rawName, normPrefix string) string {
	normName := normalize(rawName)
	normWithSep := normPrefix + "-"
	if strings.HasPrefix(normName, normWithSep) {
		return rawName[len(normWithSep):]
	}
	return rawName
}

// sortKey produces a key where -main/-master sort first within their prefix group.
func sortKey(name string) string {
	norm := normalize(name)
	if strings.HasSuffix(norm, "-main") {
		return norm[:len(norm)-5] + "\x01main"
	}
	if strings.HasSuffix(norm, "-master") {
		return norm[:len(norm)-7] + "\x01master"
	}
	return norm
}

// findRealParent returns the longest existing session name that is a dash-prefix of name.
func findRealParent(name string, nameSet map[string]bool) string {
	best := ""
	tmp := name
	for {
		idx := strings.LastIndex(tmp, "-")
		if idx < 0 {
			break
		}
		tmp = tmp[:idx]
		if nameSet[tmp] && len(tmp) > len(best) {
			best = tmp
		}
	}
	return best
}

// resolveRepoRoot returns the git repository root for a directory,
// resolving worktrees to the common repo root. Returns "" if not a git repo.
func resolveRepoRoot(dir string) string {
	if dir == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return ""
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(dir, commonDir)
	}
	return filepath.Dir(filepath.Clean(commonDir))
}

// resolveRepoRoots resolves the git repo root for each session with a path.
// Only includes a session if its name matches the repo basename (exact or dash-prefix),
// to avoid false grouping when a session's path doesn't reflect its actual project.
// Returns a map from session name to repo root.
func resolveRepoRoots(sessions []tmux.Session) map[string]string {
	roots := make(map[string]string)
	for _, s := range sessions {
		if s.Path == "" {
			continue
		}
		root := resolveRepoRoot(s.Path)
		if root == "" {
			continue
		}
		base := filepath.Base(root)
		normName := normalize(s.Name)
		normBase := normalize(base)
		if normName == normBase || strings.HasPrefix(normName, normBase+"-") {
			roots[s.Name] = root
		}
	}
	return roots
}

// resolveRepoRootsIncremental reuses cached repo roots when the session path
// hasn't changed, and only calls git for new or changed sessions.
func resolveRepoRootsIncremental(sessions []tmux.Session, snap *cache.Snapshot, now time.Time) (map[string]string, time.Time) {
	if !canReuseRepoRootsCache(snap, now) {
		return resolveRepoRoots(sessions), now
	}

	cachedPaths := make(map[string]string, len(snap.Sessions))
	for _, s := range snap.Sessions {
		cachedPaths[s.Name] = s.Path
	}

	roots := make(map[string]string, len(sessions))
	for _, s := range sessions {
		if s.Path == "" {
			continue
		}
		if root, ok := reuseCachedRepoRoot(s, cachedPaths, snap.RepoRoots); ok {
			roots[s.Name] = root
			continue
		}
		if root, ok := freshRepoRoot(s); ok {
			roots[s.Name] = root
		}
	}
	return roots, snap.RepoRootsRefreshedAt
}

func canReuseRepoRootsCache(snap *cache.Snapshot, now time.Time) bool {
	if snap == nil || len(snap.RepoRoots) == 0 || snap.RepoRootsRefreshedAt.IsZero() {
		return false
	}
	return now.Sub(snap.RepoRootsRefreshedAt) <= repoRootsRevalidateTTL
}

func reuseCachedRepoRoot(s tmux.Session, cachedPaths, cachedRoots map[string]string) (string, bool) {
	cachedPath, ok := cachedPaths[s.Name]
	if !ok || cachedPath != s.Path {
		return "", false
	}
	root, ok := cachedRoots[s.Name]
	if !ok {
		return "", false
	}
	if !repoRootMatchesSessionName(root, s.Name) {
		return "", false
	}
	return root, true
}

func freshRepoRoot(s tmux.Session) (string, bool) {
	root := resolveRepoRoot(s.Path)
	if root == "" {
		return "", false
	}
	if !repoRootMatchesSessionName(root, s.Name) {
		return "", false
	}
	return root, true
}

func repoRootMatchesSessionName(root, sessionName string) bool {
	base := normalize(filepath.Base(root))
	name := normalize(sessionName)
	return name == base || strings.HasPrefix(name, base+"-")
}
