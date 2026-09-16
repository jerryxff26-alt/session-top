package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/session-top/session-top/internal/usage"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	hotStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	fillStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	emptyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func coloredBar(leftPct float64, width int) string {
	raw := bar(leftPct, width)
	filled := strings.Count(raw, "█")
	return fillStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", width-filled))
}

// Overview renders the first screen: 5h / Weekly, TODAY, TOP QUOTA CONSUMERS.
func Overview(a *usage.Analysis) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" SESSION TOP"))
	b.WriteByte('\n')
	b.WriteString(rule(52))
	b.WriteString("\n\n")
	writeQuotaBars(&b, a)
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("TODAY"))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "%s %s\n", padRight("Sessions", 20), fmt.Sprintf("%d", a.Today.Sessions))
	fmt.Fprintf(&b, "%s %s\n", padRight("Turns", 20), fmt.Sprintf("%d", a.Today.Turns))
	fmt.Fprintf(&b, "%s %s\n", padRight("Observed tokens", 20), formatTokens(a.Today.Tokens))
	if a.Today.HasQuota && a.Today.QuotaUsed != nil {
		fmt.Fprintf(&b, "%s %s\n", padRight("5h quota used", 20), formatPct(*a.Today.QuotaUsed))
	} else {
		fmt.Fprintf(&b, "%s %s\n", padRight("5h quota used", 20), mutedStyle.Render("unknown"))
	}
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("TOP QUOTA CONSUMERS"))
	b.WriteByte('\n')
	if len(a.Consumers) == 0 {
		b.WriteString(mutedStyle.Render("No sessions found."))
		b.WriteByte('\n')
	} else {
		for i, c := range a.Consumers {
			val := formatTokens(c.Tokens) + " observed"
			if c.QuotaDelta != nil {
				val = formatPct(*c.QuotaDelta)
			}
			fmt.Fprintf(&b, "%d. %s %s\n", i+1, padRight(c.Title, 24), val)
		}
	}
	if len(a.Ambiguous) > 0 {
		b.WriteByte('\n')
		b.WriteString(warnStyle.Render("Some intervals: Attribution: ambiguous"))
		b.WriteByte('\n')
		for _, amb := range a.Ambiguous {
			fmt.Fprintf(&b, "  %s–%s  %s  %s\n",
				formatClock(amb.Start), formatClock(amb.End),
				formatPctSigned(amb.Delta),
				strings.Join(amb.Titles, ", "))
		}
	}
	return b.String()
}

func writeQuotaBars(b *strings.Builder, a *usage.Analysis) {
	now := a.GeneratedAt
	writeBar := func(label string, w *usage.QuotaWindow) {
		if w == nil {
			fmt.Fprintf(b, "%s  %s\n", padRight(label, 8), mutedStyle.Render("quota unknown"))
			return
		}
		left := w.LeftPercent
		fmt.Fprintf(b, "%s %s  %s left     %s\n",
			padRight(label, 8),
			coloredBar(left, 10),
			formatPct(left),
			mutedStyle.Render(formatReset(w.ResetAt, now)))
	}
	writeBar("5h", a.Official.FiveHour)
	writeBar("Weekly", a.Official.Weekly)
}

// WriteOverview writes the overview to w.
func WriteOverview(w io.Writer, a *usage.Analysis) error {
	_, err := io.WriteString(w, Overview(a))
	return err
}
