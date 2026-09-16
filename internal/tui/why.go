package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/session-top/session-top/internal/usage"
)

// Why renders the last-window explanation. Labels are "Potential causes".
func Why(a *usage.Analysis) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Why did my quota drop?"))
	b.WriteByte('\n')
	b.WriteByte('\n')
	w := a.Why
	fmt.Fprintf(&b, "Last 60 minutes\n")
	b.WriteString(rule(40))
	b.WriteString("\n\n")
	if w.QuotaUsed != nil {
		fmt.Fprintf(&b, "%s %s\n", padRight("Quota used", 28), formatPct(*w.QuotaUsed))
	} else {
		fmt.Fprintf(&b, "%s %s\n", padRight("Quota used", 28), mutedStyle.Render("unknown (no official snapshots)"))
	}
	b.WriteByte('\n')
	if w.Largest != nil {
		b.WriteString("Largest session:\n")
		val := formatTokens(w.Largest.Tokens) + " observed"
		if w.Largest.QuotaDelta != nil {
			val = formatPct(*w.Largest.QuotaDelta)
		}
		fmt.Fprintf(&b, "%s %s\n", padRight(w.Largest.Title, 28), val)
	}
	b.WriteByte('\n')
	b.WriteString("Observed:\n")
	fmt.Fprintf(&b, "%s %s\n", padRight("Input tokens", 28), formatTokens(w.Observed.InputTokens))
	fmt.Fprintf(&b, "%s %s\n", padRight("Output tokens", 28), formatTokens(w.Observed.OutputTokens))
	fmt.Fprintf(&b, "%s %d\n", padRight("Turns", 28), w.Observed.Turns)
	fmt.Fprintf(&b, "%s %d\n", padRight("Context compactions", 28), w.Observed.Compactions)
	fmt.Fprintf(&b, "%s %d\n", padRight("Tool calls", 28), w.Observed.ToolCalls)
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("Potential causes:"))
	b.WriteByte('\n')
	b.WriteByte('\n')
	if len(w.Causes) == 0 {
		b.WriteString(mutedStyle.Render("Not enough activity in this window to guess."))
		b.WriteByte('\n')
	}
	for _, c := range w.Causes {
		mark := okStyle.Render("✓")
		if c.Warn {
			mark = warnStyle.Render("⚠")
		}
		fmt.Fprintf(&b, "%s %s\n", mark, c.Title)
		if c.Detail != "" {
			fmt.Fprintf(&b, "  %s\n", mutedStyle.Render(c.Detail))
		}
		b.WriteByte('\n')
	}
	if len(w.Patterns) > 0 {
		b.WriteString(titleStyle.Render("Observed patterns"))
		b.WriteByte('\n')
		b.WriteByte('\n')
		for _, p := range w.Patterns {
			mark := okStyle.Render("✓")
			if p.Warn {
				mark = warnStyle.Render("⚠")
			}
			fmt.Fprintf(&b, "%s %s\n", mark, p.Title)
			if p.Detail != "" {
				fmt.Fprintf(&b, "  %s\n", mutedStyle.Render(p.Detail))
			}
			b.WriteByte('\n')
		}
	}
	if w.Expensive != nil {
		b.WriteString("Most expensive interval:\n")
		fmt.Fprintf(&b, "%s–%s\n", formatClock(w.Expensive.Start), formatClock(w.Expensive.End))
		if w.Expensive.UsedAfter > 0 || w.Expensive.UsedBefore > 0 {
			fmt.Fprintf(&b, "Quota: %s → %s\n",
				formatPct(100-w.Expensive.UsedBefore),
				formatPct(100-w.Expensive.UsedAfter))
		} else {
			fmt.Fprintf(&b, "Quota: %s this interval\n", formatPctSigned(w.Expensive.Delta))
		}
		if len(w.Expensive.SessionIDs) > 1 {
			b.WriteString(warnStyle.Render("Attribution: ambiguous"))
			b.WriteByte('\n')
			b.WriteString("Active sessions:\n")
			for _, t := range w.Expensive.Titles {
				fmt.Fprintf(&b, "  %s\n", t)
			}
		}
	}
	if len(a.Ambiguous) > 0 {
		b.WriteByte('\n')
		for _, amb := range a.Ambiguous {
			if amb.End.Before(w.WindowStart) || amb.End.After(w.WindowEnd) {
				continue
			}
			b.WriteString(warnStyle.Render("Attribution: ambiguous"))
			b.WriteByte('\n')
			fmt.Fprintf(&b, "%s–%s\n", formatClock(amb.Start), formatClock(amb.End))
			fmt.Fprintf(&b, "Quota delta      %s\n", formatPctSigned(amb.Delta))
			b.WriteString("Active sessions:\n")
			for _, t := range amb.Titles {
				fmt.Fprintf(&b, "  %s\n", t)
			}
		}
	}
	return b.String()
}

// WriteWhy writes the why report to w.
func WriteWhy(w io.Writer, a *usage.Analysis) error {
	_, err := io.WriteString(w, Why(a))
	return err
}
