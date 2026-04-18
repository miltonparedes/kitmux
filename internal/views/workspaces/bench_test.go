package workspaces

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/store"
	wsreg "github.com/miltonparedes/kitmux/internal/workspaces"
	wsdata "github.com/miltonparedes/kitmux/internal/workspaces/data"
)

// BenchmarkLoadInitialSnapshotCold runs the initial load with an empty
// SQLite cache. It should still be O(tmux + registry); no wt list calls.
func BenchmarkLoadInitialSnapshotCold(b *testing.B) {
	seedHome(b)
	seedRegistry(b, 20)

	svc := wsdata.NewStatsService()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = loadInitialSnapshot(svc)
	}
}

// BenchmarkLoadInitialSnapshotWarm runs the initial load with SQLite stats
// pre-populated. Should be indistinguishable from the cold path because we
// only read cached rows; we just want to catch regressions where someone
// re-introduces a sync wt list on the hot path.
func BenchmarkLoadInitialSnapshotWarm(b *testing.B) {
	seedHome(b)
	seedRegistry(b, 20)
	seedStats(b, 20)

	svc := wsdata.NewStatsService()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = loadInitialSnapshot(svc)
	}
}

func seedHome(tb testing.TB) {
	tb.Helper()
	dir := tb.TempDir()
	tb.Setenv("HOME", dir)
}

func seedRegistry(tb testing.TB, n int) {
	tb.Helper()
	ws := make([]wsreg.Workspace, 0, n)
	for i := 0; i < n; i++ {
		ws = append(ws, wsreg.Workspace{
			Name:    "ws" + itoa(i),
			Path:    filepath.Join(os.TempDir(), "ws", itoa(i)),
			AddedAt: 1,
		})
	}
	if err := wsreg.SaveRegistry(ws); err != nil {
		tb.Fatalf("SaveRegistry: %v", err)
	}
}

func seedStats(tb testing.TB, n int) {
	tb.Helper()
	now := time.Now()
	for i := 0; i < n; i++ {
		path := filepath.Join(os.TempDir(), "ws", itoa(i))
		stats := []store.WorktreeStat{
			{Branch: "main", WorktreePath: path, IsMain: true},
			{Branch: "feature", WorktreePath: path + "-feature", Added: i, Deleted: i / 2, Modified: true},
		}
		if err := store.ReplaceWorkspaceStats(path, stats, now); err != nil {
			tb.Fatalf("ReplaceWorkspaceStats: %v", err)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [12]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
