package codex

import "time"

// TokenUsage is a bag of token counters as recorded by Codex.
// CachedInput is a subset of Input (OpenAI includes cached tokens inside input_tokens).
type TokenUsage struct {
	Input       int64
	CachedInput int64
	Output      int64
	Reasoning   int64
	Total       int64
}

func (t TokenUsage) Equal(o TokenUsage) bool {
	return t.Input == o.Input && t.CachedInput == o.CachedInput && t.Output == o.Output && t.Reasoning == o.Reasoning && t.Total == o.Total
}

func (t TokenUsage) Sub(prev TokenUsage) TokenUsage {
	if t.Total < prev.Total {
		return t
	}
	return TokenUsage{
		Input:       t.Input - prev.Input,
		CachedInput: t.CachedInput - prev.CachedInput,
		Output:      t.Output - prev.Output,
		Reasoning:   t.Reasoning - prev.Reasoning,
		Total:       t.Total - prev.Total,
	}
}

func (t TokenUsage) IsZero() bool {
	return t.Input == 0 && t.CachedInput == 0 && t.Output == 0 && t.Reasoning == 0 && t.Total == 0
}

// Session is metadata from a rollout session_meta envelope.
type Session struct {
	ID         string
	ParentID   string
	CWD        string
	Originator string
	Source     string
	Provider   string
	StartedAt  time.Time
	File       string
}

// UsageEvent is one OBSERVED token delta (re-emits with unchanged totals are dropped).
type UsageEvent struct {
	SessionID     string
	Time          time.Time
	TurnID        string
	Model         string
	Delta         TokenUsage
	Last          TokenUsage
	ContextWindow int64
}

// Window is one OFFICIAL rate-limit window (5h or weekly), classified by duration.
type Window struct {
	UsedPercent float64
	Minutes     int
	ResetAt     time.Time
}

// CreditsInfo is the optional credits blob on a rate_limits snapshot.
type CreditsInfo struct {
	HasCredits bool
	Unlimited  bool
	Balance    string
}

// QuotaSnapshot is an OFFICIAL quota reading attached to a token_count event.
type QuotaSnapshot struct {
	SessionID string
	Time      time.Time
	FiveHour  *Window
	Weekly    *Window
	Credits   *CreditsInfo
	PlanType  string
}

// TurnStart is a task_started / turn_context marker.
type TurnStart struct {
	SessionID string
	Time      time.Time
	TurnID    string
	Model     string
}

// ToolCall is a function_call / custom_tool_call.
type ToolCall struct {
	SessionID string
	Time      time.Time
	TurnID    string
	Name      string
}

// ConversationItem is a bounded, redacted piece of user/assistant/tool context.
// It deliberately excludes reasoning and raw rollout payloads.
type ConversationItem struct {
	Time      time.Time
	TurnID    string
	Kind      string // user, assistant, tool
	Text      string
	Tool      string
	CallID    string
	Failed    bool
	Truncated bool
}

// Compaction is a compacted event (payload may have been skipped if oversized).
type Compaction struct {
	SessionID      string
	Time           time.Time
	ContextAfter   int64
	SkippedPayload bool
}

// Prompt is a user-visible message used for titles and timeline labels.
type Prompt struct {
	SessionID string
	Time      time.Time
	TurnID    string
	Text      string
}

// Rollout is one parsed rollout-*.jsonl file.
type Rollout struct {
	Session                  Session
	Usage                    []UsageEvent
	Quotas                   []QuotaSnapshot
	Turns                    []TurnStart
	Tools                    []ToolCall
	Compactions              []Compaction
	Prompts                  []Prompt
	Conversation             []ConversationItem
	SkippedConversationItems int
}

func (r *Rollout) EndedAt() time.Time {
	end := r.Session.StartedAt
	bump := func(t time.Time) {
		if t.After(end) {
			end = t
		}
	}
	for _, e := range r.Usage {
		bump(e.Time)
	}
	for _, e := range r.Quotas {
		bump(e.Time)
	}
	for _, e := range r.Turns {
		bump(e.Time)
	}
	for _, e := range r.Tools {
		bump(e.Time)
	}
	for _, e := range r.Compactions {
		bump(e.Time)
	}
	for _, e := range r.Prompts {
		bump(e.Time)
	}
	for _, e := range r.Conversation {
		bump(e.Time)
	}
	return end
}
