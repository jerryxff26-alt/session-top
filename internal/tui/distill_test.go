package tui

import (
	"strings"
	"testing"

	"github.com/jerryxff26-alt/session-top/internal/usage"
)

func TestDistillRendersArchiveDecision(t *testing.T) {
	out := Distill(usage.DistillDigest{
		Project: "/workspace/demo",
		Sessions: []usage.DistillExtract{{
			SessionID: "session-1",
			Goal:      "Low-value attempt",
			Archive: &usage.DistillArchiveDecision{
				Eligible: true,
				Status:   "candidate",
				Reason:   "all value signals are low",
			},
		}},
	})
	for _, want := range []string{"archive", "candidate", "all value signals are low", "--archive-low previews", "only --apply"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
