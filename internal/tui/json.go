package tui

import (
	"encoding/json"
	"io"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/codex"
	"github.com/jerryxff26-alt/session-top/internal/usage"
)

type overviewJSON struct {
	GeneratedAt string          `json:"generated_at"`
	Official    officialJSON    `json:"official"`
	Today       todayJSON       `json:"today"`
	Consumers   []consumerJSON  `json:"consumers"`
	Ambiguous   []ambiguousJSON `json:"ambiguous,omitempty"`
}

type officialJSON struct {
	FiveHour *windowJSON  `json:"five_hour,omitempty"`
	Weekly   *windowJSON  `json:"weekly,omitempty"`
	HasQuota bool         `json:"has_quota"`
	PlanType string       `json:"plan_type,omitempty"`
	Credits  *creditsJSON `json:"credits,omitempty"`
}

type windowJSON struct {
	UsedPercent float64 `json:"used_percent"`
	LeftPercent float64 `json:"left_percent"`
	ResetAt     string  `json:"reset_at,omitempty"`
	Minutes     int     `json:"minutes,omitempty"`
}

type creditsJSON struct {
	HasCredits bool   `json:"has_credits"`
	Unlimited  bool   `json:"unlimited"`
	Balance    string `json:"balance,omitempty"`
}

type todayJSON struct {
	Sessions     int      `json:"sessions"`
	Turns        int      `json:"turns"`
	Tokens       int64    `json:"tokens"`
	CachedTokens int64    `json:"cached_tokens,omitempty"`
	QuotaUsed    *float64 `json:"quota_used,omitempty"`
	HasQuota     bool     `json:"has_quota"`
}

type consumerJSON struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	QuotaDelta *float64 `json:"quota_delta,omitempty"`
	Tokens     int64    `json:"tokens"`
}

type ambiguousJSON struct {
	Start      string   `json:"start"`
	End        string   `json:"end"`
	Delta      float64  `json:"delta"`
	SessionIDs []string `json:"session_ids,omitempty"`
	Titles     []string `json:"titles,omitempty"`
}

type whyJSON struct {
	WindowStart string          `json:"window_start"`
	WindowEnd   string          `json:"window_end"`
	QuotaUsed   *float64        `json:"quota_used,omitempty"`
	Largest     *consumerJSON   `json:"largest,omitempty"`
	Observed    whyObservedJSON `json:"observed"`
	Causes      []causeJSON     `json:"causes,omitempty"`
	Patterns    []causeJSON     `json:"patterns,omitempty"`
}

type whyObservedJSON struct {
	InputTokens  int64 `json:"input_tokens"`
	CachedTokens int64 `json:"cached_tokens,omitempty"`
	OutputTokens int64 `json:"output_tokens"`
	Turns        int   `json:"turns"`
	Compactions  int   `json:"compactions"`
	ToolCalls    int   `json:"tool_calls"`
}

type causeJSON struct {
	Warn   bool   `json:"warn"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

type sessionsJSON struct {
	Sessions []sessionJSON `json:"sessions"`
}

type sessionJSON struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	CWD            string   `json:"cwd,omitempty"`
	Model          string   `json:"model,omitempty"`
	StartedAt      string   `json:"started_at,omitempty"`
	EndedAt        string   `json:"ended_at,omitempty"`
	Turns          int      `json:"turns"`
	InputTokens    int64    `json:"input_tokens"`
	CachedTokens   int64    `json:"cached_tokens,omitempty"`
	OutputTokens   int64    `json:"output_tokens"`
	ObservedTokens int64    `json:"observed_tokens"`
	QuotaDelta     *float64 `json:"quota_delta,omitempty"`
	Ambiguous      bool     `json:"ambiguous,omitempty"`
	Archived       bool     `json:"archived,omitempty"`
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func windowToJSON(w *usage.QuotaWindow) *windowJSON {
	if w == nil {
		return nil
	}
	return &windowJSON{
		UsedPercent: w.UsedPercent,
		LeftPercent: w.LeftPercent,
		ResetAt:     rfc3339(w.ResetAt),
		Minutes:     w.Minutes,
	}
}

func creditsToJSON(c *codex.CreditsInfo) *creditsJSON {
	if c == nil {
		return nil
	}
	return &creditsJSON{HasCredits: c.HasCredits, Unlimited: c.Unlimited, Balance: c.Balance}
}

func consumerToJSON(c *usage.Consumer) *consumerJSON {
	if c == nil {
		return nil
	}
	return &consumerJSON{ID: c.ID, Title: c.Title, QuotaDelta: c.QuotaDelta, Tokens: c.Tokens}
}

func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// WriteOverviewJSON writes the overview as JSON (same shape family as distill --json).
func WriteOverviewJSON(w io.Writer, a *usage.Analysis) error {
	out := overviewJSON{
		GeneratedAt: rfc3339(a.GeneratedAt),
		Official: officialJSON{
			FiveHour: windowToJSON(a.Official.FiveHour),
			Weekly:   windowToJSON(a.Official.Weekly),
			HasQuota: a.Official.HasQuota,
			PlanType: a.Official.PlanType,
			Credits:  creditsToJSON(a.Official.Credits),
		},
		Today: todayJSON{
			Sessions:     a.Today.Sessions,
			Turns:        a.Today.Turns,
			Tokens:       a.Today.Tokens,
			CachedTokens: a.Today.CachedTokens,
			QuotaUsed:    a.Today.QuotaUsed,
			HasQuota:     a.Today.HasQuota,
		},
	}
	for _, c := range a.Consumers {
		cp := c
		out.Consumers = append(out.Consumers, *consumerToJSON(&cp))
	}
	for _, amb := range a.Ambiguous {
		out.Ambiguous = append(out.Ambiguous, ambiguousJSON{
			Start: rfc3339(amb.Start), End: rfc3339(amb.End), Delta: amb.Delta,
			SessionIDs: amb.SessionIDs, Titles: amb.Titles,
		})
	}
	return encodeJSON(w, out)
}

// WriteWhyJSON writes the why report as JSON.
func WriteWhyJSON(w io.Writer, a *usage.Analysis) error {
	wr := a.Why
	out := whyJSON{
		WindowStart: rfc3339(wr.WindowStart),
		WindowEnd:   rfc3339(wr.WindowEnd),
		QuotaUsed:   wr.QuotaUsed,
		Largest:     consumerToJSON(wr.Largest),
		Observed: whyObservedJSON{
			InputTokens: wr.Observed.InputTokens, CachedTokens: wr.Observed.CachedTokens,
			OutputTokens: wr.Observed.OutputTokens, Turns: wr.Observed.Turns,
			Compactions: wr.Observed.Compactions, ToolCalls: wr.Observed.ToolCalls,
		},
	}
	for _, c := range wr.Causes {
		out.Causes = append(out.Causes, causeJSON{Warn: c.Warn, Title: c.Title, Detail: c.Detail})
	}
	for _, c := range wr.Patterns {
		out.Patterns = append(out.Patterns, causeJSON{Warn: c.Warn, Title: c.Title, Detail: c.Detail})
	}
	return encodeJSON(w, out)
}

// WriteSessionsJSON writes the ranked session list as JSON.
func WriteSessionsJSON(w io.Writer, a *usage.Analysis) error {
	out := sessionsJSON{}
	for _, s := range a.Sessions {
		out.Sessions = append(out.Sessions, sessionJSON{
			ID: s.ID, Title: s.Title, CWD: s.CWD, Model: s.Model,
			StartedAt: rfc3339(s.StartedAt), EndedAt: rfc3339(s.EndedAt),
			Turns: s.Turns, InputTokens: s.InputTokens, CachedTokens: s.CachedTokens,
			OutputTokens: s.OutputTokens, ObservedTokens: s.ObservedTokens,
			QuotaDelta: s.QuotaDelta, Ambiguous: s.Ambiguous, Archived: s.Archived,
		})
	}
	return encodeJSON(w, out)
}
