package tui

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func padLeft(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return strings.Repeat(" ", n-w) + s
}

func truncateWidth(s string, n int) string {
	if n <= 1 || lipgloss.Width(s) <= n {
		return s
	}
	const ell = "…"
	ew := lipgloss.Width(ell)
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw+ew > n {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + ell
}

func fitRight(s string, n int) string {
	return padRight(truncateWidth(s, n), n)
}

var (
	mdLinkRe  = regexp.MustCompile(`\[([^\]\n]+)\]\([^)]*\)`)
	absPathRe = regexp.MustCompile(`(?:/Users|/home)/[^\s,，]+`)
)

var leftoverBracketRe = regexp.MustCompile(`\[([^\]\n]+)\]`)
var danglingBracketRe = regexp.MustCompile(`\[[^\]\n]*$`)

func cleanSessionTitle(s string) string {
	s = strings.TrimSpace(s)
	s = mdLinkRe.ReplaceAllString(s, "$1")
	s = leftoverBracketRe.ReplaceAllString(s, "$1")
	s = danglingBracketRe.ReplaceAllString(s, "")
	s = absPathRe.ReplaceAllStringFunc(s, func(p string) string {
		p = strings.TrimRight(p, "/")
		base := filepath.Base(p)
		if base == "" || base == "/" {
			return p
		}
		return base
	})
	s = strings.TrimPrefix(s, "/goal ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func shortSessionID(id string) string {
	id = strings.TrimSpace(id)
	if i := strings.LastIndex(id, "-"); i >= 0 && i+1 < len(id) {
		last := id[i+1:]
		if len(last) >= 12 {
			return last[len(last)-12:]
		}
		if last != "" {
			return last
		}
	}
	if len(id) > 12 {
		return id[len(id)-12:]
	}
	return id
}

func rule(n int) string {
	if n < 8 {
		n = 52
	}
	return strings.Repeat("─", n)
}
