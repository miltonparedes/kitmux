package sessions

import (
	"testing"

	"github.com/miltonparedes/kitmux/internal/cache"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

func TestBuildTree_GroupsByRepoRoot(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "myrepo-feature", Windows: 1},
		{Name: "myrepo-main", Windows: 2},
	}
	repoRoots := map[string]string{
		"myrepo-main":    "/home/user/myrepo",
		"myrepo-feature": "/home/user/myrepo",
	}

	roots := BuildTree(sessions, repoRoots)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if roots[0].Kind != KindGroupHeader {
		t.Errorf("expected KindGroupHeader, got %d", roots[0].Kind)
	}
	if roots[0].Name != "myrepo" {
		t.Errorf("expected group name 'myrepo', got %q", roots[0].Name)
	}
	if len(roots[0].Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(roots[0].Children))
	}
	// main should sort first
	if roots[0].Children[0].SessionName != "myrepo-main" {
		t.Errorf("expected first child 'myrepo-main', got %q", roots[0].Children[0].SessionName)
	}
	if roots[0].Children[0].Name != "main" {
		t.Errorf("expected child display name 'main', got %q", roots[0].Children[0].Name)
	}
}

func TestBuildTree_SessionNamedAsGroupBecomesRoot(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "dotfiles", Windows: 1},
		{Name: "dotfiles-main", Windows: 2},
	}
	repoRoots := map[string]string{
		"dotfiles":      "/home/user/dotfiles",
		"dotfiles-main": "/home/user/dotfiles",
	}

	roots := BuildTree(sessions, repoRoots)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if roots[0].Kind != KindSession {
		t.Errorf("expected KindSession (real parent), got %d", roots[0].Kind)
	}
	if roots[0].SessionName != "dotfiles" {
		t.Errorf("expected root session 'dotfiles', got %q", roots[0].SessionName)
	}
	if len(roots[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(roots[0].Children))
	}
	if roots[0].Children[0].Name != "main" {
		t.Errorf("expected child display name 'main', got %q", roots[0].Children[0].Name)
	}
}

func TestBuildTree_SingleSessionInRepoIsStandalone(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "myrepo-main", Windows: 1},
	}
	repoRoots := map[string]string{
		"myrepo-main": "/home/user/myrepo",
	}

	roots := BuildTree(sessions, repoRoots)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if roots[0].Kind != KindSession {
		t.Errorf("expected KindSession, got %d", roots[0].Kind)
	}
	if roots[0].Name != "myrepo-main" {
		t.Errorf("expected name 'myrepo-main', got %q", roots[0].Name)
	}
	if len(roots[0].Children) != 0 {
		t.Errorf("expected no children, got %d", len(roots[0].Children))
	}
}

func TestBuildTree_DifferentReposNotGrouped(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "supply-origination-bot-backend-main", Windows: 1},
		{Name: "supply", Windows: 1},
		{Name: "ai-core-cli", Windows: 1},
	}
	// supply has no repo root (its path pointed elsewhere)
	// supply-origination-bot-backend-main has its own repo
	repoRoots := map[string]string{
		"supply-origination-bot-backend-main": "/home/user/supply-origination-bot-backend",
		"ai-core-cli":                         "/home/user/ai-core-cli",
	}

	roots := BuildTree(sessions, repoRoots)

	if len(roots) != 3 {
		t.Fatalf("expected 3 standalone roots, got %d", len(roots))
	}
	for _, r := range roots {
		if len(r.Children) != 0 {
			t.Errorf("session %q should have no children, got %d", r.Name, len(r.Children))
		}
	}
}

func TestBuildTree_MismatchedPathNotGrouped(t *testing.T) {
	// Simulates the real bug: "supply" session has path pointing to dotfiles repo.
	// resolveRepoRoots should NOT include it (name validation), so repoRoots
	// only has dotfiles-main. They should NOT be grouped together.
	sessions := []tmux.Session{
		{Name: "dotfiles-main", Windows: 2},
		{Name: "supply", Windows: 1},
	}
	// Only dotfiles-main passes name validation; supply is excluded from repoRoots
	repoRoots := map[string]string{
		"dotfiles-main": "/home/user/dotfiles",
	}

	roots := BuildTree(sessions, repoRoots)

	if len(roots) != 2 {
		t.Fatalf("expected 2 standalone roots, got %d", len(roots))
	}
}

func TestBuildTree_NoRepoFallbackParentChild(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "project", Windows: 1},
		{Name: "project-dev", Windows: 2},
	}

	roots := BuildTree(sessions, nil)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root (parent-child), got %d", len(roots))
	}
	if roots[0].Kind != KindSession {
		t.Errorf("expected KindSession, got %d", roots[0].Kind)
	}
	if roots[0].SessionName != "project" {
		t.Errorf("expected root session 'project', got %q", roots[0].SessionName)
	}
	if len(roots[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(roots[0].Children))
	}
	if roots[0].Children[0].Name != "dev" {
		t.Errorf("expected child name 'dev', got %q", roots[0].Children[0].Name)
	}
}

func TestBuildTree_MainMasterSortFirst(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "myrepo-feature", Windows: 1},
		{Name: "myrepo-bugfix", Windows: 1},
		{Name: "myrepo-main", Windows: 2},
	}
	repoRoots := map[string]string{
		"myrepo-feature": "/home/user/myrepo",
		"myrepo-main":    "/home/user/myrepo",
		"myrepo-bugfix":  "/home/user/myrepo",
	}

	roots := BuildTree(sessions, repoRoots)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if len(roots[0].Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(roots[0].Children))
	}
	if roots[0].Children[0].SessionName != "myrepo-main" {
		t.Errorf("expected first child 'myrepo-main', got %q", roots[0].Children[0].SessionName)
	}
}

func TestBuildTree_ChildNameTrimmed(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "myrepo-main", Windows: 1},
		{Name: "myrepo-feat-login", Windows: 1},
	}
	repoRoots := map[string]string{
		"myrepo-main":       "/home/user/myrepo",
		"myrepo-feat-login": "/home/user/myrepo",
	}

	roots := BuildTree(sessions, repoRoots)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	// "myrepo-feat-login" should have prefix "myrepo-" trimmed → "feat-login"
	found := false
	for _, c := range roots[0].Children {
		if c.SessionName == "myrepo-feat-login" {
			found = true
			if c.Name != "feat-login" {
				t.Errorf("expected display name 'feat-login', got %q", c.Name)
			}
		}
	}
	if !found {
		t.Error("child 'myrepo-feat-login' not found")
	}
}

func TestBuildTree_EmptyInput(t *testing.T) {
	roots := BuildTree(nil, nil)
	if len(roots) != 0 {
		t.Errorf("expected 0 roots for nil input, got %d", len(roots))
	}
}

func TestBuildTree_UnderscoreRepoGrouping(t *testing.T) {
	// Sessions with underscores should group when they share a repo root
	sessions := []tmux.Session{
		{Name: "myrepo_main", Windows: 1},
		{Name: "myrepo_feature", Windows: 1},
	}
	repoRoots := map[string]string{
		"myrepo_main":    "/home/user/myrepo",
		"myrepo_feature": "/home/user/myrepo",
	}

	roots := BuildTree(sessions, repoRoots)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if len(roots[0].Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(roots[0].Children))
	}
	// main should sort first due to normalize in sortKey
	if roots[0].Children[0].SessionName != "myrepo_main" {
		t.Errorf("expected first child 'myrepo_main', got %q", roots[0].Children[0].SessionName)
	}
	if roots[0].Children[0].Name != "main" {
		t.Errorf("expected child display name 'main', got %q", roots[0].Children[0].Name)
	}
}

func TestBuildTree_NoRepoFallbackUnderscoreParentChild(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "myapp", Windows: 1},
		{Name: "myapp_feature", Windows: 1},
	}

	roots := BuildTree(sessions, nil)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if roots[0].Kind != KindSession {
		t.Fatalf("expected real session parent, got %v", roots[0].Kind)
	}
	if roots[0].SessionName != "myapp" {
		t.Fatalf("expected parent 'myapp', got %q", roots[0].SessionName)
	}
	if len(roots[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(roots[0].Children))
	}
	if roots[0].Children[0].Name != "feature" {
		t.Errorf("expected child display name 'feature', got %q", roots[0].Children[0].Name)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo-bar", "foo-bar"},
		{"foo_bar", "foo-bar"},
		{"foo bar", "foo-bar"},
		{"a_b-c d", "a-b-c-d"},
	}
	for _, tt := range tests {
		got := normalize(tt.input)
		if got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTrimNormalizedPrefix(t *testing.T) {
	tests := []struct {
		rawName    string
		normPrefix string
		want       string
	}{
		{"api-tests", "api", "tests"},
		{"api_tests", "api", "tests"},
		{"project-worktree-1", "project-worktree", "1"},
		{"project_worktree_2", "project-worktree", "2"},
		{"unrelated", "api", "unrelated"},
	}
	for _, tt := range tests {
		got := trimNormalizedPrefix(tt.rawName, tt.normPrefix)
		if got != tt.want {
			t.Errorf("trimNormalizedPrefix(%q, %q) = %q, want %q",
				tt.rawName, tt.normPrefix, got, tt.want)
		}
	}
}

func TestSortKey_UnderscoreMain(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"api-main", "api\x01main"},
		{"api_main", "api\x01main"},
		{"api-master", "api\x01master"},
		{"api_master", "api\x01master"},
		{"api-tests", "api-tests"},
		{"api_tests", "api-tests"},
	}
	for _, tt := range tests {
		got := sortKey(tt.name)
		if got != tt.want {
			t.Errorf("sortKey(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestResolveRepoRootsIncremental_ReusesCachedRoots(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "myrepo-main", Windows: 1, Path: "/home/user/myrepo/main"},
		{Name: "myrepo-feat", Windows: 1, Path: "/home/user/myrepo/feat"},
	}
	snap := &cache.Snapshot{
		Sessions: []tmux.Session{
			{Name: "myrepo-main", Windows: 1, Path: "/home/user/myrepo/main"},
			{Name: "myrepo-feat", Windows: 1, Path: "/home/user/myrepo/feat"},
		},
		RepoRoots: map[string]string{
			"myrepo-main": "/home/user/myrepo",
			"myrepo-feat": "/home/user/myrepo",
		},
	}

	roots := resolveRepoRootsIncremental(sessions, snap)

	if roots["myrepo-main"] != "/home/user/myrepo" {
		t.Errorf("expected cached root for myrepo-main, got %q", roots["myrepo-main"])
	}
	if roots["myrepo-feat"] != "/home/user/myrepo" {
		t.Errorf("expected cached root for myrepo-feat, got %q", roots["myrepo-feat"])
	}
}

func TestResolveRepoRootsIncremental_NilSnapshot(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "tmp", Windows: 1, Path: "/tmp"},
	}
	roots := resolveRepoRootsIncremental(sessions, nil)
	// /tmp is not a git repo, so no roots expected
	if len(roots) != 0 {
		t.Errorf("expected 0 roots for non-git path with nil snapshot, got %d", len(roots))
	}
}
