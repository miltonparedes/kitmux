package sessions

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

// NodeKind distinguishes session nodes from virtual group headers.
type NodeKind int

const (
	KindSession     NodeKind = iota
	KindGroupHeader          // virtual group (no real session)
)

// TreeNode represents one entry in the session tree.
type TreeNode struct {
	Kind        NodeKind
	Name        string // display name (suffix for children, full for root)
	SessionName string // raw tmux session name (empty for virtual headers)
	Windows     int
	Attached    bool
	Children    []*TreeNode
	Expanded    bool
	Depth       int
	Added       int // working tree lines added
	Deleted     int // working tree lines deleted
}

// BuildTree groups sessions by git repository root.
//
// Sessions that share the same repo root (including worktrees) are grouped together.
// For groups with 2+ sessions, the display name is the repo directory basename.
// Sessions without a repo root fall back to parent-child grouping by name prefix.
// Sorting: -main/-master float to top within their group.
func BuildTree(sessions []tmux.Session, repoRoots map[string]string) []*TreeNode {
	if repoRoots == nil {
		repoRoots = make(map[string]string)
	}

	// Sort: -main/-master first within same prefix, then lexicographic.
	sorted := make([]tmux.Session, len(sessions))
	copy(sorted, sessions)
	sort.Slice(sorted, func(i, j int) bool {
		return sortKey(sorted[i].Name) < sortKey(sorted[j].Name)
	})

	// Build session lookup
	sesMap := make(map[string]tmux.Session)
	for _, s := range sorted {
		sesMap[s.Name] = s
	}

	// Group sessions by repo root
	repoGroups := make(map[string][]tmux.Session)
	var noRepo []tmux.Session
	for _, s := range sorted {
		root := repoRoots[s.Name]
		if root != "" {
			repoGroups[root] = append(repoGroups[root], s)
		} else {
			noRepo = append(noRepo, s)
		}
	}

	var roots []*TreeNode

	// Process repo groups in sorted order
	repoKeys := make([]string, 0, len(repoGroups))
	for k := range repoGroups {
		repoKeys = append(repoKeys, k)
	}
	sort.Strings(repoKeys)

	for _, repoRoot := range repoKeys {
		group := repoGroups[repoRoot]

		if len(group) == 1 {
			s := group[0]
			roots = append(roots, &TreeNode{
				Kind:        KindSession,
				Name:        s.Name,
				SessionName: s.Name,
				Windows:     s.Windows,
				Attached:    s.Attached,
				Depth:       0,
			})
			continue
		}

		groupName := filepath.Base(repoRoot)

		// Check if any session is named exactly like the group (normalized)
		var rootSession *tmux.Session
		var children []tmux.Session
		for i := range group {
			if normalize(group[i].Name) == normalize(groupName) {
				rootSession = &group[i]
			} else {
				children = append(children, group[i])
			}
		}

		var parent *TreeNode
		if rootSession != nil {
			parent = &TreeNode{
				Kind:        KindSession,
				Name:        groupName,
				SessionName: rootSession.Name,
				Windows:     rootSession.Windows,
				Attached:    rootSession.Attached,
				Expanded:    true,
				Depth:       0,
			}
		} else {
			parent = &TreeNode{
				Kind:     KindGroupHeader,
				Name:     groupName,
				Expanded: true,
				Depth:    0,
			}
		}

		for _, cs := range children {
			childName := trimNormalizedPrefix(cs.Name, groupName)
			parent.Children = append(parent.Children, &TreeNode{
				Kind:        KindSession,
				Name:        childName,
				SessionName: cs.Name,
				Windows:     cs.Windows,
				Attached:    cs.Attached,
				Depth:       1,
			})
		}
		roots = append(roots, parent)
	}

	// Process no-repo sessions with findRealParent fallback
	normMap := make(map[string]string, len(noRepo)) // normalized → original
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

	processed := make(map[string]bool)
	for _, s := range noRepo {
		if processed[s.Name] || parentOf[s.Name] != "" {
			continue
		}

		if len(childrenOf[s.Name]) > 0 {
			node := &TreeNode{
				Kind:        KindSession,
				Name:        s.Name,
				SessionName: s.Name,
				Windows:     s.Windows,
				Attached:    s.Attached,
				Expanded:    true,
				Depth:       0,
			}
			for _, cname := range childrenOf[s.Name] {
				cs := sesMap[cname]
				node.Children = append(node.Children, &TreeNode{
					Kind:        KindSession,
					Name:        trimNormalizedPrefix(cname, normalize(s.Name)),
					SessionName: cname,
					Windows:     cs.Windows,
					Attached:    cs.Attached,
					Depth:       1,
				})
				processed[cname] = true
			}
			roots = append(roots, node)
			processed[s.Name] = true
			continue
		}

		roots = append(roots, &TreeNode{
			Kind:        KindSession,
			Name:        s.Name,
			SessionName: s.Name,
			Windows:     s.Windows,
			Attached:    s.Attached,
			Depth:       0,
		})
		processed[s.Name] = true
	}

	return roots
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
