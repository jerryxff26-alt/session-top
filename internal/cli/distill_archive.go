package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/archive"
	"github.com/jerryxff26-alt/session-top/internal/usage"
)

const (
	archiveMinAge       = 7 * 24 * time.Hour
	archiveScoreCeiling = 0.35
)

type sessionArchiver interface {
	Archive(context.Context, string) (*archive.Result, error)
}

func applyArchiveCandidates(ctx context.Context, archiver sessionArchiver, d *usage.DistillDigest) error {
	if d == nil {
		return nil
	}
	if archiver == nil {
		return fmt.Errorf("distill: archive runner is unavailable")
	}
	for i := range d.Sessions {
		s := &d.Sessions[i]
		if s.Archive == nil || !s.Archive.Eligible || s.Archive.Status != "candidate" {
			continue
		}
		if _, err := archiver.Archive(ctx, s.SessionID); err != nil {
			s.Archive.Status = "failed"
			s.Archive.Reason = "Codex archive command failed"
			return fmt.Errorf("distill: archive session %s: %w", s.SessionID, err)
		}
		s.Archive.Status = "archived"
		s.Archive.Reason = "archived with Codex native archive command"
	}
	return nil
}

func planLowValueArchives(d *usage.DistillDigest, now time.Time, currentSessionID string) int {
	if d == nil {
		return 0
	}
	eligible := 0
	for i := range d.Sessions {
		s := &d.Sessions[i]
		decision := lowValueArchiveDecision(*s, now, currentSessionID)
		s.Archive = &decision
		if decision.Eligible {
			eligible++
		}
	}
	return eligible
}

func lowValueArchiveDecision(s usage.DistillExtract, now time.Time, currentSessionID string) usage.DistillArchiveDecision {
	protect := func(reason string) usage.DistillArchiveDecision {
		return usage.DistillArchiveDecision{Status: "protected", Reason: reason}
	}
	if s.Jev == nil {
		return protect("Jev assessment is required")
	}
	if currentSessionID != "" && s.SessionID == currentSessionID {
		return protect("current session is never auto-archived")
	}
	if !s.ContextCoverage.Complete || s.Jev.ReviewRequired {
		return protect("context is incomplete or requires review")
	}
	if strings.TrimSpace(s.StartedAt) == "" {
		return protect("session start time is unknown")
	}
	started, err := time.Parse(time.RFC3339, s.StartedAt)
	if err != nil {
		return protect("session start time is invalid")
	}
	if now.IsZero() {
		now = time.Now()
	}
	age := now.Sub(started)
	if age < archiveMinAge {
		return protect(fmt.Sprintf("session is newer than %s", archiveMinAge))
	}
	priority := strings.ToLower(strings.TrimSpace(s.Jev.Priority))
	if priority != "low" && priority != "none" {
		return protect("Jev priority is not low or none")
	}
	if s.Jev.ReusableKnowledge >= archiveScoreCeiling {
		return protect("reusable-knowledge score is not low")
	}
	if s.Jev.VerifiedEvidence >= archiveScoreCeiling {
		return protect("verified-evidence score is not low")
	}
	if s.Jev.CorrectionValue >= archiveScoreCeiling {
		return protect("correction-value score is not low")
	}
	return usage.DistillArchiveDecision{
		Eligible: true,
		Status:   "candidate",
		Reason:   "complete context, old enough, and all Jev value signals are low",
	}
}
