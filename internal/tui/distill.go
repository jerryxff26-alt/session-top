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
	fmt.Fprintf(&b, "%s %d\n", padRight("sessions", 16), len(d.Sessions))
	if d.DroppedOtherProject > 0 || d.DroppedOutsideWindow > 0 || d.DroppedContinuationOnly > 0 {
		fmt.Fprintf(&b, "%s other-project %d  outside-window %d  continuation-only %d\n",
			padRight("dropped", 16), d.DroppedOtherProject, d.DroppedOutsideWindow, d.DroppedContinuationOnly)
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
		b.WriteByte('\n')
	}
	b.WriteString(mutedStyle.Render("Digest only — not raw rollout JSONL. No model was called."))
	b.WriteByte('\n')
	return b.String()
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
