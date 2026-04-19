package data

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miltonparedes/kitmux/internal/worktree"
)

func useTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func sampleWorktree(branch, path string, added, deleted int, isMain bool) worktree.Worktree {
	return worktree.Worktree{
		Branch: branch,
		Path:   path,
		IsMain: isMain,
		WorkingTree: worktree.WorkingTree{
			Modified: added > 0 || deleted > 0,
			Diff:     worktree.Diff{Added: added, Deleted: deleted},
		},
		Commit: worktree.Commit{SHA: "abc123", Timestamp: 1},
	}
}

func TestRefreshPersistsStats(t *testing.T) {
	useTempHome(t)

	fetch := func(_ string) ([]worktree.Worktree, error) {
		return []worktree.Worktree{
			sampleWorktree("main", "/tmp/ws", 0, 0, true),
			sampleWorktree("feature", "/tmp/ws-feature", 10, 2, false),
		}, nil
	}
	svc := newStatsService(fetch)

	res := svc.Refresh("/tmp/ws")
	if res.Err != nil {
		t.Fatalf("refresh: %v", res.Err)
	}
	if len(res.Stats.Worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(res.Stats.Worktrees))
	}

	cached, err := svc.LoadCached("/tmp/ws")
	if err != nil {
		t.Fatalf("load cached: %v", err)
	}
	if len(cached.Worktrees) != 2 {
		t.Fatalf("expected 2 cached worktrees, got %d", len(cached.Worktrees))
	}
	added, deleted := cached.TotalDiff()
	if added != 10 || deleted != 2 {
		t.Errorf("expected +10/-2, got +%d/-%d", added, deleted)
	}
}

func TestRefreshSingleFlightDedupes(t *testing.T) {
	useTempHome(t)

	var calls int32
	gate := make(chan struct{})
	fetch := func(_ string) ([]worktree.Worktree, error) {
		atomic.AddInt32(&calls, 1)
		<-gate
		return []worktree.Worktree{sampleWorktree("main", "/tmp/ws", 1, 1, true)}, nil
	}
	svc := newStatsService(fetch)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.Refresh("/tmp/ws")
		}()
	}

	// Give goroutines a chance to start and join the flight.
	time.Sleep(20 * time.Millisecond)
	close(gate)
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 fetch call due to single-flight, got %d", got)
	}
}

func TestRefreshKeepsCachedOnError(t *testing.T) {
	useTempHome(t)

	fetch := func(_ string) ([]worktree.Worktree, error) {
		return []worktree.Worktree{sampleWorktree("main", "/tmp/ws", 3, 0, true)}, nil
	}
	svc := newStatsService(fetch)
	if res := svc.Refresh("/tmp/ws"); res.Err != nil {
		t.Fatalf("seed refresh: %v", res.Err)
	}

	svc.fetch = func(_ string) ([]worktree.Worktree, error) {
		return nil, errors.New("boom")
	}
	res := svc.Refresh("/tmp/ws")
	if res.Err == nil {
		t.Fatal("expected error on refresh")
	}
	if len(res.Stats.Worktrees) != 1 {
		t.Fatalf("expected cached stats returned on error, got %d worktrees", len(res.Stats.Worktrees))
	}
}

func TestInvalidatePurgesCache(t *testing.T) {
	useTempHome(t)

	fetch := func(_ string) ([]worktree.Worktree, error) {
		return []worktree.Worktree{sampleWorktree("main", "/tmp/ws", 5, 1, true)}, nil
	}
	svc := newStatsService(fetch)
	_ = svc.Refresh("/tmp/ws")
	if err := svc.Invalidate("/tmp/ws"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	cached, err := svc.LoadCached("/tmp/ws")
	if err != nil {
		t.Fatalf("load cached: %v", err)
	}
	if len(cached.Worktrees) != 0 {
		t.Fatalf("expected empty cache after invalidate, got %d", len(cached.Worktrees))
	}
}

func TestLastRefreshReflectsRecentRefresh(t *testing.T) {
	useTempHome(t)

	fetch := func(_ string) ([]worktree.Worktree, error) {
		return []worktree.Worktree{sampleWorktree("main", "/tmp/ws", 0, 0, true)}, nil
	}
	svc := newStatsService(fetch)

	before := time.Now().Add(-time.Second)
	_ = svc.Refresh("/tmp/ws")
	ts, err := svc.LastRefresh("/tmp/ws")
	if err != nil {
		t.Fatalf("last refresh: %v", err)
	}
	if !ts.After(before) {
		t.Fatalf("expected last refresh after %v, got %v", before, ts)
	}
}
