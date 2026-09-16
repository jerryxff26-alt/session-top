package usage

import (
	"time"

	"github.com/session-top/session-top/internal/codex"
)

// Analysis is the full OFFICIAL / OBSERVED / INFERRED picture for the CLI.
type Analysis struct {
	Official    OfficialQuota
	Today       TodayStats
	Sessions    []SessionSummary
	Consumers   []Consumer
	Ambiguous   []AmbiguousInterval
	Why         WhyReport
	Current     *SessionSummary
	GeneratedAt time.Time
}

// OfficialQuota is the latest OFFICIAL 5h / weekly snapshot.
type OfficialQuota struct {
	FiveHour *QuotaWindow
	Weekly   *QuotaWindow
	HasQuota bool
}

// QuotaWindow is one OFFICIAL remaining bar.
type QuotaWindow struct {
	UsedPercent float64
	LeftPercent float64
	ResetAt     time.Time
	Minutes     int
}

// TodayStats is OBSERVED activity on the local calendar day of GeneratedAt,
// plus INFERRED 5h quota consumed across today's snapshots.
type TodayStats struct {
	Sessions  int
	Turns     int
	Tokens    int64
	QuotaUsed *float64 // INFERRED 5h used today (last-first, resets skipped)
	HasQuota  bool
}

// SessionSummary is one session's OBSERVED totals plus unique INFERRED 5h Δ.
type SessionSummary struct {
	ID              string
	Title           string
	CWD             string
	Model           string
	StartedAt       time.Time
	EndedAt         time.Time
	Turns           int
	InputTokens     int64
	CachedTokens    int64
	OutputTokens    int64
	ReasoningTokens int64
	ObservedTokens  int64
	ToolCalls       int
	Compactions     int
	QuotaDelta      *float64 // unique 5h attribution only
	Ambiguous       bool
	Timeline        []TimelineItem
	ExpensiveTurn   *int // index into Timeline
	ContextGrowing  bool
	LastTurn        *TimelineItem
}

// Consumer is a top-quota (or top-token) row for the overview.
type Consumer struct {
	ID         string
	Title      string
	QuotaDelta *float64
	Tokens     int64
}

// AmbiguousInterval is an INFERRED Δ that cannot be assigned to one session.
type AmbiguousInterval struct {
	Start      time.Time
	End        time.Time
	Delta      float64
	UsedBefore float64
	UsedAfter  float64
	SessionIDs []string
	Titles     []string
}

// TimelineItem is one prompt / follow-up / compaction row.
type TimelineItem struct {
	Time            time.Time
	Kind            string // prompt, follow-up, compaction
	Label           string
	Tokens          int64
	InputTokens     int64
	OutputTokens    int64
	Duration        time.Duration
	QuotaLeftBefore *float64
	QuotaLeftAfter  *float64
	ContextBefore   int64
	ContextAfter    int64
}

// WhyReport is the last-window explanation. Wording is "Potential causes", never proven causation.
type WhyReport struct {
	WindowStart time.Time
	WindowEnd   time.Time
	QuotaUsed   *float64
	Largest     *Consumer
	Observed    WhyObserved
	Causes      []Cause
	Expensive   *AmbiguousInterval // most expensive snapshot interval (may be unique)
}

// WhyObserved is OBSERVED activity inside the why window.
type WhyObserved struct {
	InputTokens  int64
	OutputTokens int64
	Turns        int
	Compactions  int
	ToolCalls    int
}

// Cause is a potential (or notably absent) driver. Never a proven attribution.
type Cause struct {
	Warn   bool
	Title  string
	Detail string
}

func (s SessionSummary) Duration() time.Duration {
	if s.EndedAt.IsZero() || s.StartedAt.IsZero() {
		return 0
	}
	d := s.EndedAt.Sub(s.StartedAt)
	if d < 0 {
		return 0
	}
	return d
}

func leftFromUsed(used float64) float64 {
	v := 100 - used
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func ptrFloat(v float64) *float64 { return &v }

func cloneWindow(w *codex.Window) *QuotaWindow {
	if w == nil {
		return nil
	}
	return &QuotaWindow{
		UsedPercent: w.UsedPercent,
		LeftPercent: leftFromUsed(w.UsedPercent),
		ResetAt:     w.ResetAt,
		Minutes:     w.Minutes,
	}
}
