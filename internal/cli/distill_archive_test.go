package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/usage"
)

func lowValueSession(now time.Time) usage.DistillExtract {
	return usage.DistillExtract{
		SessionID: "old-low",
		StartedAt: now.Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339),
		ContextCoverage: usage.DistillContextCoverage{
			SourceItems:   3,
			IncludedItems: 3,
			Complete:      true,
		},
		Jev: &usage.DistillAssessment{
			Priority:          "low",
			PrimaryValue:      "none",
			ReusableKnowledge: 0.1,
			VerifiedEvidence:  0.2,
			CorrectionValue:   0.05,
		},
	}
}

func TestLowValueArchiveDecisionEligible(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	got := lowValueArchiveDecision(lowValueSession(now), now, "current")
	if !got.Eligible || got.Status != "candidate" {
		t.Fatalf("decision=%+v", got)
	}
}

func TestLowValueArchiveDecisionProtections(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		edit func(*usage.DistillExtract)
		want string
	}{
		{name: "current", edit: func(s *usage.DistillExtract) { s.SessionID = "current" }, want: "current"},
		{name: "incomplete", edit: func(s *usage.DistillExtract) { s.ContextCoverage.Complete = false }, want: "incomplete"},
		{name: "review", edit: func(s *usage.DistillExtract) { s.Jev.ReviewRequired = true }, want: "review"},
		{name: "recent", edit: func(s *usage.DistillExtract) { s.StartedAt = now.Add(-24 * time.Hour).Format(time.RFC3339) }, want: "newer"},
		{name: "priority", edit: func(s *usage.DistillExtract) { s.Jev.Priority = "medium" }, want: "priority"},
		{name: "knowledge", edit: func(s *usage.DistillExtract) { s.Jev.ReusableKnowledge = archiveScoreCeiling }, want: "reusable"},
		{name: "evidence", edit: func(s *usage.DistillExtract) { s.Jev.VerifiedEvidence = archiveScoreCeiling }, want: "verified"},
		{name: "correction", edit: func(s *usage.DistillExtract) { s.Jev.CorrectionValue = archiveScoreCeiling }, want: "correction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := lowValueSession(now)
			tt.edit(&s)
			got := lowValueArchiveDecision(s, now, "current")
			if got.Eligible || got.Status != "protected" || !strings.Contains(got.Reason, tt.want) {
				t.Fatalf("decision=%+v", got)
			}
		})
	}
}

func TestPlanLowValueArchivesAnnotatesAllSessions(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	eligible := lowValueSession(now)
	protected := lowValueSession(now)
	protected.SessionID = "valuable"
	protected.Jev.ReusableKnowledge = 0.9
	d := usage.DistillDigest{Sessions: []usage.DistillExtract{eligible, protected}}
	if got := planLowValueArchives(&d, now, ""); got != 1 {
		t.Fatalf("eligible=%d", got)
	}
	if d.Sessions[0].Archive == nil || !d.Sessions[0].Archive.Eligible {
		t.Fatalf("candidate=%+v", d.Sessions[0].Archive)
	}
	if d.Sessions[1].Archive == nil || d.Sessions[1].Archive.Eligible {
		t.Fatalf("protected=%+v", d.Sessions[1].Archive)
	}
}

func TestApplyArchiveCandidatesUpdatesStatusAndSkipsProtected(t *testing.T) {
	d := usage.DistillDigest{Sessions: []usage.DistillExtract{
		{SessionID: "eligible", Archive: &usage.DistillArchiveDecision{Eligible: true, Status: "candidate"}},
		{SessionID: "protected", Archive: &usage.DistillArchiveDecision{Status: "protected"}},
		{SessionID: "unplanned"},
	}}
	archiver := &recordingArchiver{}
	if err := applyArchiveCandidates(context.Background(), archiver, &d); err != nil {
		t.Fatal(err)
	}
	if len(archiver.ids) != 1 || archiver.ids[0] != "eligible" {
		t.Fatalf("archived=%v", archiver.ids)
	}
	if got := d.Sessions[0].Archive; got.Status != "archived" || !got.Eligible {
		t.Fatalf("decision=%+v", got)
	}
	if got := d.Sessions[1].Archive; got.Status != "protected" {
		t.Fatalf("protected decision=%+v", got)
	}
}

func TestApplyArchiveCandidatesMarksFailure(t *testing.T) {
	d := usage.DistillDigest{Sessions: []usage.DistillExtract{{
		SessionID: "eligible",
		Archive:   &usage.DistillArchiveDecision{Eligible: true, Status: "candidate"},
	}}}
	archiver := &recordingArchiver{err: errors.New("boom")}
	err := applyArchiveCandidates(context.Background(), archiver, &d)
	if err == nil || !strings.Contains(err.Error(), "eligible") {
		t.Fatalf("error=%v", err)
	}
	if got := d.Sessions[0].Archive; got.Status != "failed" || got.Reason != "Codex archive command failed" {
		t.Fatalf("decision=%+v", got)
	}
}

var _ sessionArchiver = (*recordingArchiver)(nil)
