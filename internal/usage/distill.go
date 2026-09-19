package usage

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/codex"
)

const (
	maxGoalRunes           = 500
	maxCorrectionRunes     = 600
	maxCorrections         = 8
	maxTools               = 8
	maxContextItems        = 9
	maxContextMessageRunes = 1600
	maxContextToolRunes    = 320
	maxContextTotalRunes   = 3200
)

// DistillOpts selects a project, optional session, and optional time window.
type DistillOpts struct {
	Project   string    // required: session cwd equals or is inside this path
	SessionID string    // optional: exact, prefix, or suffix match
	From      time.Time // inclusive; zero = unbounded
	To        time.Time // exclusive; zero = unbounded
}

// DistillContextItem is one bounded user/assistant/tool excerpt. It never
// contains model reasoning or an unbounded raw tool payload.
type DistillContextItem struct {
	Kind      string `json:"kind"`
	Text      string `json:"text,omitempty"`
	Tool      string `json:"tool,omitempty"`
	Failed    bool   `json:"failed,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// DistillContextCoverage makes missing context visible to the model and user.
type DistillContextCoverage struct {
	SourceItems       int  `json:"source_items"`
	IncludedItems     int  `json:"included_items"`
	UserMessages      int  `json:"user_messages"`
	AssistantMessages int  `json:"assistant_messages"`
	ToolEvents        int  `json:"tool_events"`
	OmittedItems      int  `json:"omitted_items"`
	TruncatedItems    int  `json:"truncated_items"`
	OversizedItems    int  `json:"oversized_items"`
	Complete          bool `json:"complete"`
}

// DistillAssessment is an optional Jev classification. Jev ranks and gates
// candidates; it does not write the resulting skill prose.
type DistillAssessment struct {
	Model             string  `json:"model"`
	Priority          string  `json:"priority"`
	PrimaryValue      string  `json:"primary_value"`
	ReusableKnowledge float64 `json:"reusable_knowledge"`
	VerifiedEvidence  float64 `json:"verified_evidence"`
	CorrectionValue   float64 `json:"correction_value"`
	InputTokens       int64   `json:"input_tokens,omitempty"`
	ReviewRequired    bool    `json:"review_required"`
}

// DistillArchiveDecision records the safe archive gate and optional result.
// It is emitted only when --archive-low is requested.
type DistillArchiveDecision struct {
	Eligible bool   `json:"eligible"`
	Status   string `json:"status"` // candidate, protected, archived, failed
	Reason   string `json:"reason"`
}

// DistillExtract is one bounded session extract. Never a transcript dump.
type DistillExtract struct {
	SessionID             string                  `json:"session_id"`
	CWD                   string                  `json:"cwd"`
	ParentID              string                  `json:"parent_id,omitempty"`
	StartedAt             string                  `json:"started_at,omitempty"`
	Goal                  string                  `json:"goal"`
	Corrections           []string                `json:"corrections,omitempty"`
	Tools                 []ToolCount             `json:"tools,omitempty"`
	Context               []DistillContextItem    `json:"context,omitempty"`
	ContextCoverage       DistillContextCoverage  `json:"context_coverage"`
	Jev                   *DistillAssessment      `json:"jev,omitempty"`
	Archive               *DistillArchiveDecision `json:"archive,omitempty"`
	ObservedTokens        int64                   `json:"observed_tokens"`
	InputTokens           int64                   `json:"input_tokens"`
	CachedTokens          int64                   `json:"cached_tokens"`
	OutputTokens          int64                   `json:"output_tokens"`
	ReasoningTokens       int64                   `json:"reasoning_tokens"`
	CachedShare           float64                 `json:"cached_share_of_input"`
	Turns                 int                     `json:"turns"`
	ContinuationFollowUps int                     `json:"continuation_follow_ups"`
}

// DistillDigest is the project-level OBSERVED digest for distill / the orchestrator skill.
type DistillDigest struct {
	Project                 string           `json:"project"`
	SessionID               string           `json:"session_id,omitempty"`
	From                    string           `json:"from,omitempty"`
	To                      string           `json:"to,omitempty"`
	Sessions                []DistillExtract `json:"sessions"`
	DroppedContinuationOnly int              `json:"dropped_continuation_only"`
	DroppedOtherProject     int              `json:"dropped_other_project"`
	DroppedOtherSession     int              `json:"dropped_other_session"`
	DroppedOutsideWindow    int              `json:"dropped_outside_window"`
}

// Distill filters analysis sessions into a bounded project digest. No model.
func Distill(a *Analysis, opts DistillOpts) DistillDigest {
	project := filepath.Clean(opts.Project)
	d := DistillDigest{Project: project, SessionID: opts.SessionID}
	if !opts.From.IsZero() {
		d.From = opts.From.UTC().Format(time.RFC3339)
	}
	if !opts.To.IsZero() {
		d.To = opts.To.UTC().Format(time.RFC3339)
	}
	if a == nil {
		return d
	}
	for _, s := range a.Sessions {
		if !cwdInProject(s.CWD, project) {
			d.DroppedOtherProject++
			continue
		}
		if opts.SessionID != "" && !sessionIDMatches(s.ID, opts.SessionID) {
			d.DroppedOtherSession++
			continue
		}
		if !inWindow(s, opts.From, opts.To) {
			d.DroppedOutsideWindow++
			continue
		}
		if s.Title == "" && s.ContinuationFollowUps > 0 {
			d.DroppedContinuationOnly++
			continue
		}
		d.Sessions = append(d.Sessions, extractSession(s))
	}
	return d
}

func extractSession(s SessionSummary) DistillExtract {
	e := DistillExtract{
		SessionID:             s.ID,
		CWD:                   s.CWD,
		ParentID:              s.ParentID,
		Goal:                  capRunes(s.Title, maxGoalRunes),
		ObservedTokens:        s.ObservedTokens,
		InputTokens:           s.InputTokens,
		CachedTokens:          s.CachedTokens,
		OutputTokens:          s.OutputTokens,
		ReasoningTokens:       s.ReasoningTokens,
		Turns:                 s.Turns,
		ContinuationFollowUps: s.ContinuationFollowUps,
	}
	if !s.StartedAt.IsZero() {
		e.StartedAt = s.StartedAt.UTC().Format(time.RFC3339)
	}
	if s.InputTokens > 0 {
		e.CachedShare = 100 * float64(s.CachedTokens) / float64(s.InputTokens)
	}
	for i, c := range s.Corrections {
		if i >= maxCorrections {
			break
		}
		e.Corrections = append(e.Corrections, capRunes(c, maxCorrectionRunes))
	}
	tools := s.TopTools
	if len(tools) > maxTools {
		tools = tools[:maxTools]
	}
	e.Tools = tools
	e.Context, e.ContextCoverage = distillContext(s.Conversation, s.SkippedContextItems)
	return e
}

func distillContext(items []codex.ConversationItem, oversized int) ([]DistillContextItem, DistillContextCoverage) {
	coverage := DistillContextCoverage{
		SourceItems:    len(items) + oversized,
		OversizedItems: oversized,
	}
	indices := contextIndices(items, maxContextItems)
	coverage.OmittedItems = len(items) - len(indices)
	messageLimit, toolLimits := contextRuneLimits(items, indices)
	out := make([]DistillContextItem, 0, len(indices))
	for _, idx := range indices {
		item := items[idx]
		limit := messageLimit
		if item.Kind == "tool" {
			limit = toolLimits[idx]
		}
		text, clipped := capRunesFlag(item.Text, limit)
		ci := DistillContextItem{
			Kind:      item.Kind,
			Text:      text,
			Tool:      item.Tool,
			Failed:    item.Failed,
			Truncated: item.Truncated || clipped,
		}
		if ci.Truncated {
			coverage.TruncatedItems++
		}
		switch item.Kind {
		case "user":
			coverage.UserMessages++
		case "assistant":
			coverage.AssistantMessages++
		case "tool":
			coverage.ToolEvents++
		}
		out = append(out, ci)
	}
	coverage.IncludedItems = len(out)
	coverage.Complete = coverage.SourceItems > 0 && coverage.OmittedItems == 0 && coverage.TruncatedItems == 0 && coverage.OversizedItems == 0
	return out, coverage
}

func contextRuneLimits(items []codex.ConversationItem, indices []int) (int, map[int]int) {
	toolLimits := make(map[int]int)
	toolRunes := 0
	messages := 0
	for _, idx := range indices {
		item := items[idx]
		if item.Kind != "tool" {
			messages++
			continue
		}
		limit := 160
		if item.Failed {
			limit = maxContextToolRunes
		}
		toolLimits[idx] = limit
		n := len([]rune(strings.TrimSpace(item.Text)))
		if n > limit {
			n = limit
		}
		toolRunes += n
	}
	if messages == 0 {
		if len(indices) == 0 {
			return maxContextMessageRunes, toolLimits
		}
		perTool := maxContextTotalRunes / len(indices)
		if perTool > maxContextToolRunes {
			perTool = maxContextToolRunes
		}
		for _, idx := range indices {
			if items[idx].Kind == "tool" {
				toolLimits[idx] = perTool
			}
		}
		return maxContextMessageRunes, toolLimits
	}
	remaining := maxContextTotalRunes - toolRunes
	if remaining < messages {
		remaining = messages
	}
	messageLimit := remaining / messages
	if messageLimit > maxContextMessageRunes {
		messageLimit = maxContextMessageRunes
	}
	return messageLimit, toolLimits
}

func contextIndices(items []codex.ConversationItem, limit int) []int {
	total := len(items)
	if total <= 0 || limit <= 0 {
		return nil
	}
	if total <= limit {
		out := make([]int, total)
		for i := range out {
			out[i] = i
		}
		return out
	}
	selected := make(map[int]struct{}, limit)
	for i, item := range items {
		if item.Kind == "user" || item.Kind == "assistant" || item.Failed {
			selected[i] = struct{}{}
		}
	}
	if len(selected) > limit {
		messageIndices := make([]int, 0, len(selected))
		for i := range selected {
			messageIndices = append(messageIndices, i)
		}
		sort.Ints(messageIndices)
		return headTailIndices(messageIndices, limit)
	}
	for i := 0; i < total && len(selected) < limit; i++ {
		if items[i].Kind == "tool" {
			selected[i] = struct{}{}
			break
		}
	}
	for i := total - 1; i >= 0 && len(selected) < limit; i-- {
		selected[i] = struct{}{}
	}
	out := make([]int, 0, len(selected))
	for i := range selected {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

func headTailIndices(indices []int, limit int) []int {
	if len(indices) <= limit {
		return indices
	}
	head := limit / 3
	if head < 1 {
		head = 1
	}
	tail := limit - head
	out := append([]int(nil), indices[:head]...)
	out = append(out, indices[len(indices)-tail:]...)
	return out
}

func sessionIDMatches(sessionID, query string) bool {
	if sessionID == query {
		return true
	}
	return len(query) >= 4 && (strings.HasPrefix(sessionID, query) || strings.HasSuffix(sessionID, query))
}

func cwdInProject(sessionCWD, project string) bool {
	s := filepath.Clean(sessionCWD)
	p := filepath.Clean(project)
	if s == "." || p == "." {
		return s == p
	}
	if s == p {
		return true
	}
	sep := string(filepath.Separator)
	prefix := p
	if !strings.HasSuffix(prefix, sep) {
		prefix += sep
	}
	return strings.HasPrefix(s, prefix)
}

func inWindow(s SessionSummary, from, to time.Time) bool {
	start, end := s.StartedAt, s.EndedAt
	if end.IsZero() {
		end = start
	}
	if !from.IsZero() && end.Before(from) {
		return false
	}
	if !to.IsZero() && !start.Before(to) {
		return false
	}
	return true
}

func capRunes(s string, n int) string {
	out, _ := capRunesFlag(s, n)
	return out
}

func capRunesFlag(s string, n int) (string, bool) {
	r := []rune(strings.TrimSpace(s))
	if n <= 1 || len(r) <= n {
		return string(r), false
	}
	return string(r[:n-1]) + "…", true
}
