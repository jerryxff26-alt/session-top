package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/session-top/session-top/internal/usage"
)

func TestWatchViewFromAnalyzerSnapshot(t *testing.T) {
	left5 := 67.0
	leftW := 83.0
	delta := 4.0
	in := int64(181000)
	out := int64(8000)
	a := &usage.Analysis{
		Official: usage.OfficialQuota{
			HasQuota: true,
			FiveHour: &usage.QuotaWindow{UsedPercent: 33, LeftPercent: left5, Minutes: 300},
			Weekly:   &usage.QuotaWindow{UsedPercent: 17, LeftPercent: leftW, Minutes: 10080},
		},
		GeneratedAt: time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC),
		Current: &usage.SessionSummary{
			ID:             "sess-1",
			Title:          "Fix OAuth callback",
			StartedAt:      time.Date(2026, 9, 16, 14, 22, 0, 0, time.UTC),
			EndedAt:        time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC),
			Turns:          14,
			ObservedTokens: 921000,
			QuotaDelta:     &delta,
			ContextGrowing: true,
			LastTurn: &usage.TimelineItem{
				Time:         time.Date(2026, 9, 16, 14, 39, 0, 0, time.UTC),
				Kind:         "follow-up",
				InputTokens:  in,
				OutputTokens: out,
				Duration:     47 * time.Second,
			},
		},
	}
	view := NewWatchForTest(a).View()
	for _, want := range []string{
		"SESSION TOP",
		"5h",
		"Weekly",
		"67%",
		"83%",
		"CURRENT SESSION",
		"Fix OAuth callback",
		"Last turn",
		"181K",
		"8.0K",
		"context growing quickly",
		"LIVE",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("watch view missing %q\n%s", want, view)
		}
	}
}

func TestRenderWatchBlankIsDefect(t *testing.T) {
	a := &usage.Analysis{
		Official: usage.OfficialQuota{
			HasQuota: true,
			FiveHour: &usage.QuotaWindow{LeftPercent: 50, Minutes: 300},
			Weekly:   &usage.QuotaWindow{LeftPercent: 80, Minutes: 10080},
		},
		Current: &usage.SessionSummary{Title: "x", Turns: 1, LastTurn: &usage.TimelineItem{InputTokens: 10}},
	}
	got := RenderWatch(a)
	if strings.TrimSpace(got) == "" {
		t.Fatal("RenderWatch returned a blank surface")
	}
	if !strings.Contains(got, "5h") || !strings.Contains(got, "Weekly") {
		t.Fatalf("watch surface missing bars:\n%s", got)
	}
}
