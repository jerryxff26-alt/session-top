package usage

import (
	"path/filepath"
	"strings"
	"time"
)

const (
	maxGoalRunes       = 500
	maxCorrectionRunes = 240
	maxCorrections     = 8
	maxTools           = 8
)

// DistillOpts selects a project and optional time window.
type DistillOpts struct {
	Project string    // required: session cwd equals or is inside this path
	From    time.Time // inclusive; zero = unbounded
	To      time.Time // exclusive; zero = unbounded
}

// DistillExtract is one bounded session extract. Never a transcript dump.
type DistillExtract struct {
	SessionID             string      `json:"session_id"`
	CWD                   string      `json:"cwd"`
	ParentID              string      `json:"parent_id,omitempty"`
	Goal                  string      `json:"goal"`
	Corrections           []string    `json:"corrections,omitempty"`
	Tools                 []ToolCount `json:"tools,omitempty"`
	ObservedTokens        int64       `json:"observed_tokens"`
	InputTokens           int64       `json:"input_tokens"`
	CachedTokens          int64       `json:"cached_tokens"`
	OutputTokens          int64       `json:"output_tokens"`
	ReasoningTokens       int64       `json:"reasoning_tokens"`
	CachedShare           float64     `json:"cached_share_of_input"`
	Turns                 int         `json:"turns"`
	ContinuationFollowUps int         `json:"continuation_follow_ups"`
}

// DistillDigest is the project-level OBSERVED digest for distill / the orchestrator skill.
type DistillDigest struct {
	Project                 string           `json:"project"`
	From                    string           `json:"from,omitempty"`
	To                      string           `json:"to,omitempty"`
	Sessions                []DistillExtract `json:"sessions"`
	DroppedContinuationOnly int              `json:"dropped_continuation_only"`
	DroppedOtherProject     int              `json:"dropped_other_project"`
	DroppedOutsideWindow    int              `json:"dropped_outside_window"`
}

// Distill filters analysis sessions into a bounded project digest. No model.
func Distill(a *Analysis, opts DistillOpts) DistillDigest {
	project := filepath.Clean(opts.Project)
	d := DistillDigest{Project: project}
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
	return e
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
	r := []rune(strings.TrimSpace(s))
	if n <= 1 || len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}
