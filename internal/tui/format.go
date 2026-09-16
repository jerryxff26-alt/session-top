package tui

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		v := float64(n) / 1_000_000
		if v >= 10 {
			return fmt.Sprintf("%.1fM", v)
		}
		return fmt.Sprintf("%.2fM", v)
	case n >= 10_000:
		return fmt.Sprintf("%.0fK", math.Round(float64(n)/1000))
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatPct(p float64) string {
	if math.IsNaN(p) {
		return "?"
	}
	rounded := math.Round(p)
	if math.Abs(p-rounded) < 0.05 {
		return fmt.Sprintf("%.0f%%", rounded)
	}
	return fmt.Sprintf("%.1f%%", p)
}

func formatPctSigned(p float64) string {
	s := formatPct(p)
	if p > 0 {
		return "+" + s
	}
	return s
}

func bar(leftPct float64, width int) string {
	if width <= 0 {
		width = 10
	}
	filled := int(math.Round(leftPct / 100 * float64(width)))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func formatReset(resetAt, now time.Time) string {
	if resetAt.IsZero() {
		return ""
	}
	d := resetAt.Sub(now)
	if d < 0 {
		return "reset due"
	}
	return "reset " + formatDuration(d)
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	mins := d / time.Minute
	secs := (d - mins*time.Minute) / time.Second
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	case mins > 0:
		return fmt.Sprintf("%dm", mins)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

func formatClock(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("15:04")
}

func formatClockUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("15:04")
}

func padRight(s string, n int) string {
	w := runeLen(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func runeLen(s string) int {
	return len([]rune(s))
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if n <= 1 || len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
}

func rule(n int) string {
	if n < 8 {
		n = 52
	}
	return strings.Repeat("─", n)
}
