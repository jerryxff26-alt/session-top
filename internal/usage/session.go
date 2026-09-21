package usage

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/codex"
)

func buildSessions(rollouts []*codex.Rollout, unique map[string]float64, ambIDs map[string]struct{}, snaps []codex.QuotaSnapshot) []SessionSummary {
	out := make([]SessionSummary, 0, len(rollouts))
	for _, r := range rollouts {
		s := summarize(r, snaps)
		if v, ok := unique[s.ID]; ok {
			s.QuotaDelta = ptrFloat(v)
		}
		if _, ok := ambIDs[s.ID]; ok {
			s.Ambiguous = true
		}
		out = append(out, s)
	}
	attachForks(out)
	finishAutopsies(out)
	sort.Slice(out, func(i, j int) bool {
		di, dj := out[i].QuotaDelta, out[j].QuotaDelta
		if di != nil && dj != nil && *di != *dj {
			return *di > *dj
		}
		if di != nil && dj == nil {
			return true
		}
		if di == nil && dj != nil {
			return false
		}
		if out[i].ObservedTokens != out[j].ObservedTokens {
			return out[i].ObservedTokens > out[j].ObservedTokens
		}
		// active sessions before archived when otherwise equal
		if out[i].Archived != out[j].Archived {
			return !out[i].Archived && out[j].Archived
		}
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}

func summarize(r *codex.Rollout, snaps []codex.QuotaSnapshot) SessionSummary {
	s := SessionSummary{
		ID:                    r.Session.ID,
		ParentID:              r.Session.ParentID,
		CWD:                   r.Session.CWD,
		StartedAt:             r.Session.StartedAt,
		EndedAt:               r.EndedAt(),
		Turns:                 len(r.Turns),
		ToolCalls:             len(r.Tools),
		Compactions:           len(r.Compactions),
		ContinuationFollowUps: countContinuations(r.Prompts),
		TopTools:              tallyTools(r.Tools),
		Conversation:          append([]codex.ConversationItem(nil), r.Conversation...),
		SkippedContextItems:   r.SkippedConversationItems,
		Archived:              r.Session.Archived,
	}
	if s.Turns == 0 {
		s.Turns = len(r.Usage)
	}
	for _, e := range r.Usage {
		s.InputTokens += e.Delta.Input
		s.CachedTokens += e.Delta.CachedInput
		s.OutputTokens += e.Delta.Output
		s.ReasoningTokens += e.Delta.Reasoning
		s.ObservedTokens += e.Delta.Total
		if e.Model != "" {
			s.Model = e.Model
		}
	}
	if s.Model == "" {
		for _, t := range r.Turns {
			if t.Model != "" {
				s.Model = t.Model
				break
			}
		}
	}
	s.Title = titleFor(r)
	s.Corrections = correctionsFor(r)
	s.Timeline = buildTimeline(r, snaps)
	s.ExpensiveTurn = expensiveIndex(s.Timeline)
	s.ContextGrowing = contextGrowing(r.Usage)
	if n := len(s.Timeline); n > 0 {
		last := s.Timeline[n-1]
		for i := n - 1; i >= 0; i-- {
			if s.Timeline[i].Kind != "compaction" {
				last = s.Timeline[i]
				break
			}
		}
		s.LastTurn = &last
	}
	return s
}

func titleFor(r *codex.Rollout) string {
	for _, p := range r.Prompts {
		if t := strings.TrimSpace(p.Text); t != "" {
			return shortTitle(t)
		}
	}
	if r.Session.CWD != "" {
		return filepath.Base(r.Session.CWD)
	}
	id := r.Session.ID
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func shortTitle(text string) string {
	text = strings.TrimSpace(text)
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = strings.TrimSpace(text[:i])
	}
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	if len(r) <= 72 {
		return text
	}
	return string(r[:71]) + "…"
}

func buildTimeline(r *codex.Rollout, snaps []codex.QuotaSnapshot) []TimelineItem {
	type mark struct {
		t    time.Time
		kind string
		u    *codex.UsageEvent
		c    *codex.Compaction
	}
	var marks []mark
	for i := range r.Usage {
		e := &r.Usage[i]
		marks = append(marks, mark{t: e.Time, kind: "usage", u: e})
	}
	for i := range r.Compactions {
		c := &r.Compactions[i]
		marks = append(marks, mark{t: c.Time, kind: "compaction", c: c})
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].t.Before(marks[j].t) })

	var items []TimelineItem
	usageN := 0
	var prevTurnStart time.Time
	if !r.Session.StartedAt.IsZero() {
		prevTurnStart = r.Session.StartedAt
	}
	turnStarts := r.Turns
	ti := 0
	var prevContext int64
	for _, m := range marks {
		for ti < len(turnStarts) && !turnStarts[ti].Time.After(m.t) {
			prevTurnStart = turnStarts[ti].Time
			ti++
		}
		switch m.kind {
		case "compaction":
			item := TimelineItem{
				Time:          m.t,
				Kind:          "compaction",
				Label:         "compaction",
				ContextBefore: prevContext,
				ContextAfter:  m.c.ContextAfter,
			}
			items = append(items, item)
		case "usage":
			usageN++
			kind := "follow-up"
			label := "agent follow-up"
			if usageN == 1 {
				kind = "prompt"
				label = "prompt"
				if len(r.Prompts) > 0 {
					label = "prompt"
				}
			}
			item := TimelineItem{
				Time:         m.t,
				Kind:         kind,
				Label:        label,
				Tokens:       m.u.Delta.Total,
				InputTokens:  m.u.Last.Input,
				OutputTokens: m.u.Delta.Output,
			}
			if item.InputTokens == 0 {
				item.InputTokens = m.u.Delta.Input
			}
			if !prevTurnStart.IsZero() && m.t.After(prevTurnStart) {
				item.Duration = m.t.Sub(prevTurnStart)
			}
			before, after := quotaAround(snaps, m.t)
			item.QuotaLeftBefore = before
			item.QuotaLeftAfter = after
			if m.u.Last.Input > 0 {
				prevContext = m.u.Last.Input
			}
			items = append(items, item)
		}
	}
	return items
}

func quotaAround(snaps []codex.QuotaSnapshot, at time.Time) (before, after *float64) {
	var prev, cur *codex.Window
	for i := range snaps {
		s := &snaps[i]
		if s.FiveHour == nil {
			continue
		}
		if s.Time.After(at) {
			break
		}
		if s.Time.Equal(at) {
			cur = s.FiveHour
			continue
		}
		prev = s.FiveHour
	}
	if prev != nil {
		v := leftFromUsed(prev.UsedPercent)
		before = &v
	}
	if cur != nil {
		v := leftFromUsed(cur.UsedPercent)
		after = &v
		if before == nil {
			// no previous: still show the reading
		}
	}
	if before != nil && after != nil && *before == *after {
		// unchanged reading is noise on the timeline
		return before, after
	}
	return before, after
}

func expensiveIndex(items []TimelineItem) *int {
	best := -1
	var bestDrop float64
	var bestTok int64
	hasDrop := false
	for i, it := range items {
		if it.Kind == "compaction" {
			continue
		}
		if it.QuotaLeftBefore != nil && it.QuotaLeftAfter != nil {
			drop := *it.QuotaLeftBefore - *it.QuotaLeftAfter
			if drop > 0 && (!hasDrop || drop > bestDrop) {
				hasDrop = true
				bestDrop = drop
				best = i
			}
		}
		if !hasDrop && it.Tokens >= bestTok {
			bestTok = it.Tokens
			best = i
		}
	}
	if best < 0 {
		return nil
	}
	return &best
}

func contextGrowing(usage []codex.UsageEvent) bool {
	if len(usage) < 2 {
		return false
	}
	start := 0
	if len(usage) > 3 {
		start = len(usage) - 3
	}
	prev := int64(-1)
	ups := 0
	var first, last int64
	for i := start; i < len(usage); i++ {
		v := usage[i].Last.Input
		if v == 0 {
			v = usage[i].Delta.Input
		}
		if prev >= 0 && v > prev {
			ups++
		}
		if first == 0 {
			first = v
		}
		last = v
		prev = v
	}
	if ups >= 2 && last > first {
		return true
	}
	if len(usage) >= 2 {
		a := usage[len(usage)-2].Last.Input
		b := usage[len(usage)-1].Last.Input
		if a == 0 {
			a = usage[len(usage)-2].Delta.Input
		}
		if b == 0 {
			b = usage[len(usage)-1].Delta.Input
		}
		if a > 0 && b > a && b >= a+a/5 {
			return true
		}
	}
	return false
}

func consumers(sessions []SessionSummary) []Consumer {
	out := make([]Consumer, 0, len(sessions))
	anyQuota := false
	for _, s := range sessions {
		if s.QuotaDelta != nil {
			anyQuota = true
			break
		}
	}
	for _, s := range sessions {
		if anyQuota && s.QuotaDelta == nil {
			continue
		}
		out = append(out, Consumer{ID: s.ID, Title: s.Title, QuotaDelta: s.QuotaDelta, Tokens: s.ObservedTokens})
	}
	if anyQuota {
		sort.Slice(out, func(i, j int) bool {
			di, dj := 0.0, 0.0
			if out[i].QuotaDelta != nil {
				di = *out[i].QuotaDelta
			}
			if out[j].QuotaDelta != nil {
				dj = *out[j].QuotaDelta
			}
			if di != dj {
				return di > dj
			}
			return out[i].Tokens > out[j].Tokens
		})
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func currentSession(sessions []SessionSummary) *SessionSummary {
	if len(sessions) == 0 {
		return nil
	}
	best := 0
	for i := 1; i < len(sessions); i++ {
		if sessions[i].EndedAt.After(sessions[best].EndedAt) {
			best = i
		}
	}
	cp := sessions[best]
	return &cp
}

func todayStats(sessions []SessionSummary, deltas []snapshotDelta, now time.Time) TodayStats {
	y, m, d := now.In(time.Local).Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	dayEnd := dayStart.Add(24 * time.Hour)
	overlaps := func(start, end time.Time) bool {
		if end.IsZero() {
			end = start
		}
		return !end.Before(dayStart) && start.Before(dayEnd)
	}
	var st TodayStats
	for _, s := range sessions {
		if overlaps(s.StartedAt, s.EndedAt) {
			st.Sessions++
			st.Turns += s.Turns
			st.Tokens += s.ObservedTokens
			st.CachedTokens += s.CachedTokens
		}
	}
	st.QuotaUsed = inferredToday(deltas, now)
	st.HasQuota = st.QuotaUsed != nil
	return st
}
