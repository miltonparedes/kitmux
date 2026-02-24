package recency

import (
	"testing"
	"time"
)

func TestSortByRecency(t *testing.T) {
	items := []string{"alpha", "bravo", "charlie", "delta"}
	timestamps := map[string]time.Time{
		"charlie": time.Now().Add(-1 * time.Minute),
		"alpha":   time.Now().Add(-5 * time.Minute),
	}

	sorted := SortByRecency(items, timestamps, func(s string) string { return s })

	// charlie (most recent) first, then alpha, then bravo/delta in original order
	want := []string{"charlie", "alpha", "bravo", "delta"}
	for i, got := range sorted {
		if got != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got, want[i])
		}
	}
}

func TestSortByRecencyEmpty(t *testing.T) {
	items := []string{"a", "b"}
	sorted := SortByRecency(items, nil, func(s string) string { return s })
	if len(sorted) != 2 || sorted[0] != "a" {
		t.Errorf("expected original order, got %v", sorted)
	}
}
