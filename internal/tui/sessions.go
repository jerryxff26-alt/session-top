package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/session-top/session-top/internal/usage"
)

const (
	sessIDW    = 12
	sessTitleW = 38
	sessQuotaW = 6
	sessTokW   = 7
	sessTurnW  = 5
)

func sessionsRow(id, title, quota, tokens, turns string) string {
	return fitRight(id, sessIDW) + " │ " +
		fitRight(title, sessTitleW) + " │ " +
		padLeft(quota, sessQuotaW) + " │ " +
		padLeft(tokens, sessTokW) + " │ " +
		padLeft(turns, sessTurnW)
}

func sessionsRule() string {
	return strings.Repeat("─", sessIDW) + "─┼─" +
		strings.Repeat("─", sessTitleW) + "─┼─" +
		strings.Repeat("─", sessQuotaW) + "─┼─" +
		strings.Repeat("─", sessTokW) + "─┼─" +
		strings.Repeat("─", sessTurnW)
}

// Sessions renders the ranked session table.
func Sessions(a *usage.Analysis) string {
	var b strings.Builder
	header := sessionsRow("ID", "SESSION", "QUOTA", "TOKENS", "TURNS")
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(sessionsRule())
	b.WriteByte('\n')
	if len(a.Sessions) == 0 {
		b.WriteString(mutedStyle.Render("No sessions found."))
		b.WriteByte('\n')
		return b.String()
	}
	anyAmbiguous := false
	for _, s := range a.Sessions {
		delta := "—"
		if s.QuotaDelta != nil {
			delta = "-" + formatPct(*s.QuotaDelta)
		} else if s.Ambiguous {
			delta = "~"
			anyAmbiguous = true
		}
		title := cleanSessionTitle(s.Title)
		if title == "" {
			title = s.ID
		}
		id := shortSessionID(s.ID)
		b.WriteString(sessionsRow(
			id,
			title,
			delta,
			formatTokens(s.ObservedTokens),
			fmt.Sprintf("%d", s.Turns),
		))
		b.WriteByte('\n')
	}
	if anyAmbiguous {
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("~  Attribution: ambiguous (overlapping sessions)"))
		b.WriteByte('\n')
	}
	return b.String()
}

// SessionDetail renders a single session timeline.
func SessionDetail(s *usage.SessionSummary) string {
	if s == nil {
		return "session not found\n"
	}
	var b strings.Builder
	title := cleanSessionTitle(s.Title)
	if title == "" {
		title = s.Title
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render(s.ID))
	b.WriteByte('\n')
	if s.QuotaDelta != nil {
		fmt.Fprintf(&b, "Quota Δ  -%s\n", formatPct(*s.QuotaDelta))
	} else if s.Ambiguous {
		b.WriteString("Quota Δ  Attribution: ambiguous\n")
	} else {
		b.WriteString("Quota Δ  unknown\n")
	}
	fmt.Fprintf(&b, "Observed %s tokens  %d turns\n", formatTokens(s.ObservedTokens), s.Turns)
	b.WriteByte('\n')
	for i, it := range s.Timeline {
		fmt.Fprintf(&b, "%s  %s\n", formatClock(it.Time), it.Label)
		switch it.Kind {
		case "compaction":
			switch {
			case it.ContextBefore > 0 && it.ContextAfter > 0:
				fmt.Fprintf(&b, "       context %s → %s\n", formatTokens(it.ContextBefore), formatTokens(it.ContextAfter))
			case it.ContextBefore > 0:
				fmt.Fprintf(&b, "       context %s compacted\n", formatTokens(it.ContextBefore))
			default:
				b.WriteString("       context compacted\n")
			}
		default:
			if it.Tokens > 0 {
				fmt.Fprintf(&b, "       %s tokens\n", formatTokens(it.Tokens))
			}
			if it.QuotaLeftBefore != nil && it.QuotaLeftAfter != nil {
				fmt.Fprintf(&b, "       quota %s → %s\n", formatPct(*it.QuotaLeftBefore), formatPct(*it.QuotaLeftAfter))
			} else if it.QuotaLeftAfter != nil {
				fmt.Fprintf(&b, "       quota %s\n", formatPct(*it.QuotaLeftAfter))
			}
		}
		if s.ExpensiveTurn != nil && *s.ExpensiveTurn == i {
			b.WriteByte('\n')
			b.WriteString(hotStyle.Render("🔥 Expensive turn"))
			b.WriteByte('\n')
			fmt.Fprintf(&b, "       %s\n", formatClock(it.Time))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func WriteSessions(w io.Writer, a *usage.Analysis) error {
	_, err := io.WriteString(w, Sessions(a))
	return err
}

func WriteSessionDetail(w io.Writer, s *usage.SessionSummary) error {
	_, err := io.WriteString(w, SessionDetail(s))
	return err
}
