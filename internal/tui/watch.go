package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jerryxff26-alt/session-top/internal/usage"
)

const watchTick = 2 * time.Second

type tickMsg time.Time
type analysisMsg struct {
	a   *usage.Analysis
	err error
}

// Watch is the live Bubble Tea model. View() is the shipped watch surface.
type Watch struct {
	load     func() (*usage.Analysis, error)
	a        *usage.Analysis
	err      error
	width    int
	quitting bool
}

// NewWatch builds a live model that reloads via load on each tick.
func NewWatch(load func() (*usage.Analysis, error)) Watch {
	return Watch{load: load, width: 72}
}

// NewWatchForTest returns a model already holding an analysis (no IO).
func NewWatchForTest(a *usage.Analysis) Watch {
	return Watch{a: a, width: 72}
}

func (m Watch) Init() tea.Cmd {
	return tea.Batch(m.reload(), tickCmd())
}

func (m Watch) reload() tea.Cmd {
	if m.load == nil {
		return nil
	}
	load := m.load
	return func() tea.Msg {
		a, err := load()
		return analysisMsg{a: a, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(watchTick, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Watch) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case analysisMsg:
		m.a = msg.a
		m.err = msg.err
	case tickMsg:
		return m, tea.Batch(m.reload(), tickCmd())
	}
	return m, nil
}

func (m Watch) View() string {
	if m.quitting {
		return ""
	}
	if m.err != nil && m.a == nil {
		return "session-top watch: " + m.err.Error() + "\n"
	}
	if m.a == nil {
		return "SESSION TOP                            LIVE\n\nLoading…\n"
	}
	return RenderWatch(m.a)
}

// RenderWatch is the shipped watch surface: 5h/Weekly, current session, last turn, warnings.
func RenderWatch(a *usage.Analysis) string {
	var b strings.Builder
	header := titleStyle.Render("SESSION TOP") + strings.Repeat(" ", 28) + hotStyle.Render("LIVE")
	b.WriteString(header)
	b.WriteString("\n\n")
	now := a.GeneratedAt
	writeLiveBar := func(label string, w *usage.QuotaWindow) {
		if w == nil {
			fmt.Fprintf(&b, "%s  %s\n", padRight(label, 8), mutedStyle.Render("quota unknown"))
			return
		}
		drop := ""
		fmt.Fprintf(&b, "%s %s %s %s %s\n",
			padRight(label, 8),
			formatPct(w.LeftPercent),
			coloredBar(w.LeftPercent, 10),
			drop,
			mutedStyle.Render(formatReset(w.ResetAt, now)),
		)
	}
	if rate := dropRate(a); rate != "" {
		if a.Official.FiveHour != nil {
			fmt.Fprintf(&b, "%s %s %s     %s\n",
				padRight("5h", 8),
				formatPct(a.Official.FiveHour.LeftPercent),
				coloredBar(a.Official.FiveHour.LeftPercent, 10),
				warnStyle.Render(rate),
			)
		} else {
			writeLiveBar("5h", a.Official.FiveHour)
		}
	} else {
		writeLiveBar("5h", a.Official.FiveHour)
	}
	writeLiveBar("Weekly", a.Official.Weekly)
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("CURRENT SESSION"))
	b.WriteByte('\n')
	if a.Current == nil {
		b.WriteString(mutedStyle.Render("No sessions."))
		b.WriteByte('\n')
		return b.String()
	}
	s := a.Current
	b.WriteString(s.Title)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s %s\n", padRight("Duration", 16), formatDuration(s.Duration()))
	fmt.Fprintf(&b, "%s %d\n", padRight("Turns", 16), s.Turns)
	fmt.Fprintf(&b, "%s %s\n", padRight("Tokens", 16), formatTokens(s.ObservedTokens))
	delta := "—"
	if s.QuotaDelta != nil {
		delta = "-" + formatPct(*s.QuotaDelta)
	} else if s.Ambiguous {
		delta = "ambiguous"
	}
	fmt.Fprintf(&b, "%s %s\n", padRight("Quota delta", 16), delta)
	b.WriteByte('\n')
	b.WriteString("Last turn:\n\n")
	if s.LastTurn != nil {
		fmt.Fprintf(&b, "%s %s\n", padRight("input", 16), formatTokens(s.LastTurn.InputTokens))
		fmt.Fprintf(&b, "%s %s\n", padRight("output", 16), formatTokens(s.LastTurn.OutputTokens))
		dur := s.LastTurn.Duration
		if dur > 0 {
			fmt.Fprintf(&b, "%s %s\n", padRight("duration", 16), formatDuration(dur))
		}
	} else {
		b.WriteString(mutedStyle.Render("none yet"))
		b.WriteByte('\n')
	}
	if s.ContextGrowing {
		b.WriteByte('\n')
		b.WriteString(warnStyle.Render("⚠ context growing quickly"))
		b.WriteByte('\n')
	}
	return b.String()
}

func dropRate(a *usage.Analysis) string {
	if a.Official.FiveHour == nil || a.Why.Expensive == nil || a.Why.Expensive.Delta <= 0 {
		return ""
	}
	d := a.Why.Expensive.End.Sub(a.Why.Expensive.Start)
	if d <= 0 {
		return ""
	}
	return fmt.Sprintf("↓ %s / %s", formatPct(a.Why.Expensive.Delta), formatDuration(d))
}
