package usage

import (
	"fmt"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/codex"
)

const whyWindow = 60 * time.Minute

func buildWhy(rollouts []*codex.Rollout, sessions []SessionSummary, deltas []snapshotDelta, amb []AmbiguousInterval, now time.Time) WhyReport {
	end := now
	var latest time.Time
	for _, r := range rollouts {
		if t := r.EndedAt(); t.After(latest) {
			latest = t
		}
	}
	if !latest.IsZero() {
		// Stale logs, or fixtures timestamped later than wall clock: explain
		// the last hour of recorded activity rather than an empty live window.
		if latest.Before(now.Add(-whyWindow)) || latest.After(now) {
			end = latest
		}
	}
	if end.IsZero() {
		end = now
	}
	start := end.Add(-whyWindow)
	w := WhyReport{WindowStart: start, WindowEnd: end}

	inWin := func(t time.Time) bool {
		return !t.Before(start) && !t.After(end)
	}

	var qSum float64
	var qAny bool
	for _, d := range deltas {
		if d.End.Before(start) || d.End.After(end) {
			continue
		}
		qSum += d.Delta
		qAny = true
	}
	if qAny {
		w.QuotaUsed = &qSum
	}

	obs := WhyObserved{}
	for _, r := range rollouts {
		for _, e := range r.Usage {
			if inWin(e.Time) {
				obs.InputTokens += e.Delta.Input
				obs.CachedTokens += e.Delta.CachedInput
				obs.OutputTokens += e.Delta.Output
			}
		}
		for _, t := range r.Turns {
			if inWin(t.Time) {
				obs.Turns++
			}
		}
		for _, c := range r.Compactions {
			if inWin(c.Time) {
				obs.Compactions++
			}
		}
		for _, t := range r.Tools {
			if inWin(t.Time) {
				obs.ToolCalls++
			}
		}
	}
	if obs.Turns == 0 {
		for _, r := range rollouts {
			for _, e := range r.Usage {
				if inWin(e.Time) {
					obs.Turns++
				}
			}
		}
	}
	w.Observed = obs

	var largest *Consumer
	for _, s := range sessions {
		active := false
		if !s.EndedAt.Before(start) && !s.StartedAt.After(end) {
			active = true
		}
		if !active {
			continue
		}
		c := Consumer{ID: s.ID, Title: s.Title, QuotaDelta: s.QuotaDelta, Tokens: s.ObservedTokens}
		if largest == nil {
			cp := c
			largest = &cp
			continue
		}
		if c.QuotaDelta != nil && (largest.QuotaDelta == nil || *c.QuotaDelta > *largest.QuotaDelta) {
			cp := c
			largest = &cp
			continue
		}
		if c.QuotaDelta == nil && largest.QuotaDelta == nil && c.Tokens > largest.Tokens {
			cp := c
			largest = &cp
		}
	}
	w.Largest = largest
	w.Causes = potentialCauses(obs)
	w.Expensive = mostExpensive(deltas, amb, start, end)
	w.Patterns, w.TokenLeader, w.QuotaLeader = crossSessionPatterns(sessions)
	return w
}

func potentialCauses(obs WhyObserved) []Cause {
	var causes []Cause
	if obs.InputTokens >= 200_000 && obs.Turns >= 2 {
		causes = append(causes, Cause{
			Warn:   true,
			Title:  "Very large repeated context",
			Detail: fmt.Sprintf("%s input tokens across %d turns", formatRawTokens(obs.InputTokens), obs.Turns),
		})
	} else if obs.InputTokens > 0 && obs.Turns > 0 {
		causes = append(causes, Cause{
			Warn:   false,
			Title:  "Input context was not unusually large",
			Detail: fmt.Sprintf("%s input tokens across %d turns", formatRawTokens(obs.InputTokens), obs.Turns),
		})
	}
	if obs.Turns >= 8 {
		causes = append(causes, Cause{
			Warn:   true,
			Title:  "High turn count",
			Detail: fmt.Sprintf("Agent continued for %d turns", obs.Turns),
		})
	} else if obs.Turns > 0 {
		causes = append(causes, Cause{
			Warn:   false,
			Title:  "Turn count was not high",
			Detail: fmt.Sprintf("%d turns in the window", obs.Turns),
		})
	}
	if obs.Compactions > 0 {
		causes = append(causes, Cause{
			Warn:   true,
			Title:  "Context compaction",
			Detail: fmt.Sprintf("%d compaction(s) detected", obs.Compactions),
		})
	} else {
		causes = append(causes, Cause{
			Warn:   false,
			Title:  "No context compaction detected",
			Detail: "",
		})
	}
	if obs.ToolCalls >= 20 {
		causes = append(causes, Cause{
			Warn:   true,
			Title:  "Many tool calls",
			Detail: fmt.Sprintf("%d tool calls", obs.ToolCalls),
		})
	}
	if obs.OutputTokens*10 < obs.InputTokens || obs.OutputTokens < 50_000 {
		causes = append(causes, Cause{
			Warn:   false,
			Title:  "Output generation was not significant",
			Detail: formatRawTokens(obs.OutputTokens) + " output tokens",
		})
	} else {
		causes = append(causes, Cause{
			Warn:   true,
			Title:  "Large output generation",
			Detail: formatRawTokens(obs.OutputTokens) + " output tokens",
		})
	}
	return causes
}

func mostExpensive(deltas []snapshotDelta, amb []AmbiguousInterval, start, end time.Time) *AmbiguousInterval {
	var best *AmbiguousInterval
	for _, d := range deltas {
		if d.End.Before(start) || d.End.After(end) {
			continue
		}
		item := AmbiguousInterval{
			Start:      d.Start,
			End:        d.End,
			Delta:      d.Delta,
			UsedBefore: d.UsedBefore,
			UsedAfter:  d.UsedAfter,
			SessionIDs: d.Active,
		}
		if len(d.Active) > 1 {
			item.Titles = append([]string(nil), d.Active...)
		}
		if best == nil || d.Delta > best.Delta {
			cp := item
			best = &cp
		}
	}
	return best
}

func formatRawTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 10_000:
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
