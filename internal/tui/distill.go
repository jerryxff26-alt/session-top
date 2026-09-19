package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jerryxff26-alt/session-top/internal/usage"
)

// Distill renders a bounded project digest (not a transcript dump).
func Distill(d usage.DistillDigest) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("DISTILL (observed digest)"))
	b.WriteByte('\n')
	b.WriteString(rule(52))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "%s %s\n", padRight("project", 16), d.Project)
	if d.From != "" || d.To != "" {
		fmt.Fprintf(&b, "%s %s → %s\n", padRight("window", 16), d.From, d.To)
	}
	if d.SessionID != "" {
		fmt.Fprintf(&b, "%s %s\n", padRight("session", 16), d.SessionID)
	}
	fmt.Fprintf(&b, "%s %d\n", padRight("sessions", 16), len(d.Sessions))
	if d.DroppedOtherProject > 0 || d.DroppedOtherSession > 0 || d.DroppedOutsideWindow > 0 || d.DroppedContinuationOnly > 0 {
		fmt.Fprintf(&b, "%s other-project %d  other-session %d  outside-window %d  continuation-only %d\n",
			padRight("dropped", 16), d.DroppedOtherProject, d.DroppedOtherSession, d.DroppedOutsideWindow, d.DroppedContinuationOnly)
	}
	b.WriteByte('\n')
	if len(d.Sessions) == 0 {
		b.WriteString(mutedStyle.Render("No sessions matched this project/window."))
		b.WriteByte('\n')
		return b.String()
	}
	for _, s := range d.Sessions {
		fmt.Fprintf(&b, "%s  %s\n", s.SessionID, s.Goal)
		fmt.Fprintf(&b, "  %s %s\n", padRight("cwd", 14), s.CWD)
		if s.ParentID != "" {
			fmt.Fprintf(&b, "  %s %s\n", padRight("parent", 14), s.ParentID)
		}
		fmt.Fprintf(&b, "  %s %s observed  input %s  cached %s of input  output %s  turns %d\n",
			padRight("mix", 14),
			formatTokens(s.ObservedTokens),
			formatTokens(s.InputTokens),
			formatPct(s.CachedShare),
			formatTokens(s.OutputTokens),
			s.Turns)
		if len(s.Tools) > 0 {
			parts := make([]string, 0, len(s.Tools))
			for _, t := range s.Tools {
				parts = append(parts, fmt.Sprintf("%s %d", t.Name, t.Count))
			}
			fmt.Fprintf(&b, "  %s %s\n", padRight("tools", 14), strings.Join(parts, " · "))
		}
		if s.ContinuationFollowUps > 0 {
			fmt.Fprintf(&b, "  %s %d\n", padRight("continuation", 14), s.ContinuationFollowUps)
		}
		for _, c := range s.Corrections {
			fmt.Fprintf(&b, "  %s %s\n", padRight("correction", 14), c)
		}
		if s.ContextCoverage.SourceItems > 0 {
			fmt.Fprintf(&b, "  %s included %d/%d  user %d  assistant %d  tool %d  omitted %d  truncated %d  oversized %d  complete %t\n",
				padRight("context", 14),
				s.ContextCoverage.IncludedItems,
				s.ContextCoverage.SourceItems,
				s.ContextCoverage.UserMessages,
				s.ContextCoverage.AssistantMessages,
				s.ContextCoverage.ToolEvents,
				s.ContextCoverage.OmittedItems,
				s.ContextCoverage.TruncatedItems,
				s.ContextCoverage.OversizedItems,
				s.ContextCoverage.Complete)
		}
		if s.Jev != nil {
			fmt.Fprintf(&b, "  %s %s · %s  reusable %.2f  verified %.2f  correction %.2f  review %t\n",
				padRight("jev", 14),
				s.Jev.Priority,
				s.Jev.PrimaryValue,
				s.Jev.ReusableKnowledge,
				s.Jev.VerifiedEvidence,
				s.Jev.CorrectionValue,
				s.Jev.ReviewRequired)
		}
		if s.Archive != nil {
			fmt.Fprintf(&b, "  %s %s · %s\n", padRight("archive", 14), s.Archive.Status, s.Archive.Reason)
		}
		for _, item := range s.Context {
			label := strings.ToUpper(item.Kind)
			if item.Tool != "" {
				label += " " + item.Tool
			}
			if item.Failed {
				label += " [failed]"
			}
			if item.Truncated {
				label += " [truncated]"
			}
			fmt.Fprintf(&b, "  %s %s\n", padRight(label, 14), indentMultiline(item.Text, 18))
		}
		b.WriteByte('\n')
	}
	b.WriteString(mutedStyle.Render("Bounded, redacted digest — never raw rollout JSONL. --archive-low previews; only --apply runs Codex native archive."))
	b.WriteByte('\n')
	return b.String()
}

func indentMultiline(text string, spaces int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	indent := "\n" + strings.Repeat(" ", spaces)
	return strings.ReplaceAll(text, "\n", indent)
}

func WriteDistill(w io.Writer, d usage.DistillDigest) error {
	_, err := io.WriteString(w, Distill(d))
	return err
}

func WriteDistillJSON(w io.Writer, d usage.DistillDigest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}
