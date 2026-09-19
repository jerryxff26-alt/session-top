package usage

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/codex"
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

func TestDistillContextKeepsOpeningAndClosingEvidence(t *testing.T) {
	items := make([]codex.ConversationItem, 20)
	for i := range items {
		items[i] = codex.ConversationItem{Kind: "tool", Tool: "exec_command", Text: "step"}
	}
	items[0] = codex.ConversationItem{Kind: "user", Text: "Initial multi-turn request"}
	items[19] = codex.ConversationItem{Kind: "assistant", Text: "Final verified conclusion"}
	context, coverage := distillContext(items, 1)
	if len(context) != maxContextItems {
		t.Fatalf("context=%d want %d", len(context), maxContextItems)
	}
	if context[0].Kind != "user" || !strings.Contains(context[0].Text, "Initial") {
		t.Fatalf("opening request missing: %+v", context[0])
	}
	if last := context[len(context)-1]; last.Kind != "assistant" || !strings.Contains(last.Text, "verified") {
		t.Fatalf("closing answer missing: %+v", last)
	}
	if coverage.OmittedItems != 11 || coverage.OversizedItems != 1 || coverage.Complete {
		t.Fatalf("coverage=%+v", coverage)
	}
}

func TestDistillSessionFilter(t *testing.T) {
	a := &Analysis{Sessions: []SessionSummary{
		{ID: "01970000-0000-7000-8000-000000000001", CWD: "/workspace/oauth-app", Title: "keep"},
		{ID: "01970000-0000-7000-8000-000000000002", CWD: "/workspace/oauth-app", Title: "drop"},
	}}
	d := Distill(a, DistillOpts{Project: "/workspace/oauth-app", SessionID: "0001"})
	if len(d.Sessions) != 1 || d.Sessions[0].Goal != "keep" || d.DroppedOtherSession != 1 {
		t.Fatalf("filtered digest: %+v", d)
	}
}

func TestEmptyDistillContextRequiresReview(t *testing.T) {
	context, coverage := distillContext(nil, 0)
	if len(context) != 0 || coverage.Complete {
		t.Fatalf("context=%+v coverage=%+v", context, coverage)
	}
}
