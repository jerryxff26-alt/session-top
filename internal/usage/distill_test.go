package usage

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDistillProjectFilterAndBounds(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	home := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "codex-home"))
	now := time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC)
	a, err := Load(home, now)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	d := Distill(a, DistillOpts{Project: "/workspace/oauth-app", From: from, To: to})
	if d.DroppedOtherProject < 1 {
		t.Fatalf("expected other-project drops, got %+v", d)
	}
	if len(d.Sessions) != 1 {
		t.Fatalf("sessions=%d want 1 (oauth only): %+v", len(d.Sessions), d.Sessions)
	}
	s := d.Sessions[0]
	if s.Goal != "Fix OAuth callback" {
		t.Fatalf("goal %q", s.Goal)
	}
	if s.SessionID != "01970000-0000-7000-8000-000000000001" {
		t.Fatalf("id %s", s.SessionID)
	}
	if s.ContinuationFollowUps < 1 {
		t.Fatalf("continuation %d", s.ContinuationFollowUps)
	}
	if s.CachedShare < 30 {
		t.Fatalf("cached share %.1f", s.CachedShare)
	}
	if len(s.Tools) == 0 {
		t.Fatal("expected tools")
	}
	if s.ParentID != "" {
		t.Fatalf("oauth should not be a fork, parent=%s", s.ParentID)
	}
	for _, x := range d.Sessions {
		if strings.Contains(x.Goal, "Overlap") || strings.Contains(x.CWD, "overlap") {
			t.Fatalf("mixed other project into digest: %+v", x)
		}
		if strings.Contains(x.Goal, strings.Repeat("A", 100)) {
			t.Fatal("oversized compacted payload leaked into digest")
		}
	}
}

func TestCwdInProject(t *testing.T) {
	if !cwdInProject("/workspace/oauth-app", "/workspace/oauth-app") {
		t.Fatal("exact")
	}
	if !cwdInProject("/workspace/oauth-app/cmd", "/workspace/oauth-app") {
		t.Fatal("nested")
	}
	if cwdInProject("/workspace/db-app", "/workspace/oauth-app") {
		t.Fatal("sibling must not match")
	}
	if cwdInProject("/workspace/oauth-app", "/workspace/oauth") {
		t.Fatal("prefix sibling oauth vs oauth-app")
	}
}
