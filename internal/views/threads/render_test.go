package threads

import (
	"strings"
	"testing"
)

func TestMetaLineJoinsProjectAndBranch(t *testing.T) {
	headless := Row{Kind: RowHeadless, Project: "kitmux", Branch: "feat/threads", SessionName: "droid-kitmux"}
	if got := metaLine(headless); got != "kitmux · feat/threads" {
		t.Fatalf("headless metaLine = %q", got)
	}

	ephemeral := Row{Kind: RowEphemeral, Project: "myapp", Branch: "main", SessionName: "work", WindowIndex: 1, PaneIndex: 2}
	if got := metaLine(ephemeral); got != "myapp · main · 1.2" {
		t.Fatalf("ephemeral metaLine = %q", got)
	}

	noBranch := Row{Kind: RowHeadless, Project: "kitmux", SessionName: "x"}
	if got := metaLine(noBranch); got != "kitmux" {
		t.Fatalf("noBranch metaLine = %q", got)
	}
}

func TestBadgesOmitsLabelForManagedThreads(t *testing.T) {
	if got := badges(Row{Kind: RowHeadless}); got != "" {
		t.Fatalf("managed unattached badges = %q, want empty", got)
	}
	if got := badges(Row{Kind: RowHeadless, Attached: true}); !strings.Contains(got, "●") {
		t.Fatalf("managed attached badges = %q, want ●", got)
	}
	if got := badges(Row{Kind: RowEphemeral}); !strings.Contains(got, "detected") {
		t.Fatalf("detected badges = %q, want detected", got)
	}
	if strings.Contains(badges(Row{Kind: RowHeadless}), "headless") {
		t.Fatal("managed badges should not contain headless")
	}
}
