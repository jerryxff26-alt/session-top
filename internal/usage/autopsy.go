package usage

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jerryxff26-alt/session-top/internal/codex"
)

func buildAutopsy(s *SessionSummary) Autopsy {
	a := Autopsy{
		Duration:        s.Duration(),
		Turns:           s.Turns,
		ObservedTokens:  s.ObservedTokens,
		InputTokens:     s.InputTokens,
		CachedTokens:    s.CachedTokens,
		OutputTokens:    s.OutputTokens,
		ReasoningTokens: s.ReasoningTokens,
		ToolCalls:       s.ToolCalls,
		TopTools:        s.TopTools,
		Compactions:     s.Compactions,
	}
	if s.ObservedTokens > 0 {
		a.InputPct = 100 * float64(s.InputTokens) / float64(s.ObservedTokens)
		a.OutputPct = 100 * float64(s.OutputTokens) / float64(s.ObservedTokens)
		a.ReasoningPct = 100 * float64(s.ReasoningTokens) / float64(s.ObservedTokens)
	}
	if s.InputTokens > 0 {
		a.CachedShare = 100 * float64(s.CachedTokens) / float64(s.InputTokens)
	}
	a.Patterns = sessionPatterns(s, a)
	return a
}

func sessionPatterns(s *SessionSummary, mix Autopsy) []Cause {
	var out []Cause
	if s.ContinuationFollowUps > 0 {
		out = append(out, Cause{
			Warn:   true,
			Title:  "continuation-style follow-ups",
			Detail: fmt.Sprintf("user follow-up × %d (keep going / 继续)", s.ContinuationFollowUps),
		})
	}
	if s.ContextGrowing {
		out = append(out, Cause{
			Warn:   true,
			Title:  "context grew across turns",
			Detail: "input size rose on later turns",
		})
	}
	if s.Compactions > 0 {
		out = append(out, Cause{
			Warn:   true,
			Title:  "context compaction",
			Detail: fmt.Sprintf("%d compaction(s) detected", s.Compactions),
		})
	}
	if mix.CachedShare >= 30 {
		out = append(out, Cause{
			Warn:   true,
			Title:  "large cached share of input",
			Detail: fmt.Sprintf("%.0f%% of input was cached (context already seen)", mix.CachedShare),
		})
	}
	if len(s.SiblingIDs) > 0 {
		n := len(s.SiblingIDs) + 1
		detail := fmt.Sprintf("%d related sessions", n)
		if s.ParentID != "" {
			detail += " (forked from " + shortID(s.ParentID) + ")"
		}
		if len(s.SiblingTitles) > 0 {
			detail += ": " + joinTitles(s.SiblingTitles, 3)
		}
		out = append(out, Cause{
			Warn:   true,
			Title:  "sibling/fork cluster",
			Detail: detail,
		})
	}
	if s.ObservedTokens > 0 && s.OutputTokens*10 < s.InputTokens {
		out = append(out, Cause{
			Warn:   false,
			Title:  "output was not the bulk of observed tokens",
			Detail: fmt.Sprintf("%s output vs %s input", formatRawTokens(s.OutputTokens), formatRawTokens(s.InputTokens)),
		})
	}
	return out
}

func correctionsFor(r *codex.Rollout) []string {
	var out []string
	for i, p := range r.Prompts {
		if i == 0 {
			continue
		}
		if isContinuationPrompt(p.Text) {
			continue
		}
		if !isCorrectionPrompt(p.Text) {
			continue
		}
		out = append(out, p.Text)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func isCorrectionPrompt(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	keys := []string{
		"don't", "do not", "stop changing", "never ",
		"不要", "别", "谁让你", "不对", "错了", "不是这样", "改为", "改成",
		"只读取了", "没有考虑", "遗漏", "漏了", "重新",
	}
	for _, k := range keys {
		if strings.Contains(t, k) {
			return true
		}
	}
	return false
}

func countContinuations(prompts []codex.Prompt) int {
	n := 0
	for i, p := range prompts {
		if i == 0 {
			continue
		}
		if isContinuationPrompt(p.Text) {
			n++
		}
	}
	return n
}

func isContinuationPrompt(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.TrimRight(t, ".!！。")
	if t == "" {
		return false
	}
	if t == "继续" || t == "continue" || t == "go on" || t == "keep going" {
		return true
	}
	if strings.HasPrefix(t, "继续") || strings.HasPrefix(t, "keep going") || strings.HasPrefix(t, "continue ") {
		return true
	}
	return false
}

func tallyTools(tools []codex.ToolCall) []ToolCount {
	if len(tools) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, t := range tools {
		name := t.Name
		if name == "" {
			name = "tool"
		}
		counts[name]++
	}
	out := make([]ToolCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, ToolCount{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func attachForks(sessions []SessionSummary) {
	byID := map[string]int{}
	for i, s := range sessions {
		byID[s.ID] = i
	}
	children := map[string][]int{}
	for i, s := range sessions {
		if s.ParentID == "" {
			continue
		}
		children[s.ParentID] = append(children[s.ParentID], i)
	}
	for parent, kids := range children {
		cluster := append([]int(nil), kids...)
		if pi, ok := byID[parent]; ok {
			cluster = append([]int{pi}, cluster...)
		}
		if len(cluster) < 2 {
			continue
		}
		for _, i := range cluster {
			var ids, titles []string
			for _, j := range cluster {
				if sessions[j].ID == sessions[i].ID {
					continue
				}
				ids = append(ids, sessions[j].ID)
				titles = append(titles, sessions[j].Title)
			}
			sessions[i].SiblingIDs = ids
			sessions[i].SiblingTitles = titles
		}
	}
}

func finishAutopsies(sessions []SessionSummary) {
	for i := range sessions {
		sessions[i].Autopsy = buildAutopsy(&sessions[i])
	}
}

func crossSessionPatterns(sessions []SessionSummary) (patterns []Cause, tokenLeader, quotaLeader *Consumer) {
	if len(sessions) == 0 {
		return nil, nil, nil
	}
	var tok, quo *SessionSummary
	for i := range sessions {
		s := &sessions[i]
		if tok == nil || s.ObservedTokens > tok.ObservedTokens {
			tok = s
		}
		if s.QuotaDelta != nil {
			if quo == nil || *s.QuotaDelta > *quo.QuotaDelta {
				quo = s
			}
		}
	}
	if tok != nil {
		c := Consumer{ID: tok.ID, Title: tok.Title, QuotaDelta: tok.QuotaDelta, Tokens: tok.ObservedTokens}
		tokenLeader = &c
	}
	if quo != nil {
		c := Consumer{ID: quo.ID, Title: quo.Title, QuotaDelta: quo.QuotaDelta, Tokens: quo.ObservedTokens}
		quotaLeader = &c
	}
	if tokenLeader != nil && quotaLeader != nil && tokenLeader.ID != quotaLeader.ID {
		q := "unknown"
		if quotaLeader.QuotaDelta != nil {
			q = fmt.Sprintf("%.1f%% inferred 5h Δ", *quotaLeader.QuotaDelta)
		}
		patterns = append(patterns, Cause{
			Warn:  true,
			Title: "Token rank and official quota rank differ",
			Detail: fmt.Sprintf("most observed tokens: %s (%s); most inferred 5h quota: %s (%s)",
				tokenLeader.Title, formatRawTokens(tokenLeader.Tokens),
				quotaLeader.Title, q),
		})
	}
	cont := 0
	for _, s := range sessions {
		if s.ContinuationFollowUps > 0 {
			cont++
		}
	}
	if cont > 0 {
		patterns = append(patterns, Cause{
			Warn:   true,
			Title:  "Continuation follow-ups",
			Detail: fmt.Sprintf("%d session(s) with keep-going / 继续 style prompts", cont),
		})
	}
	seenParent := map[string]int{}
	for _, s := range sessions {
		if s.ParentID != "" {
			seenParent[s.ParentID]++
		}
	}
	forkSessions := 0
	for _, n := range seenParent {
		forkSessions += n
	}
	if forkSessions > 0 {
		patterns = append(patterns, Cause{
			Warn:   true,
			Title:  "Fork cluster",
			Detail: fmt.Sprintf("%d session(s) encode a parent/fork link", forkSessions),
		})
	}
	return patterns, tokenLeader, quotaLeader
}

func shortID(id string) string {
	if i := strings.LastIndex(id, "-"); i >= 0 && i+1 < len(id) {
		last := id[i+1:]
		if len(last) > 12 {
			return last[len(last)-12:]
		}
		return last
	}
	if len(id) > 12 {
		return id[len(id)-12:]
	}
	return id
}

func joinTitles(titles []string, n int) string {
	if n > 0 && len(titles) > n {
		titles = titles[:n]
	}
	return strings.Join(titles, ", ")
}
