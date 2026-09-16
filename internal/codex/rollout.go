package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// MaxJSONLine is the largest JSONL line we fully unmarshal.
// Bigger lines (typical compacted payloads) are scanned for type/timestamp only.
const MaxJSONLine = 64 * 1024

var (
	typeFieldRe = regexp.MustCompile(`"type"\s*:\s*"([^"]+)"`)
	tsFieldRe   = regexp.MustCompile(`"timestamp"\s*:\s*"([^"]+)"`)
)

type envelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	ID            string `json:"id"`
	SessionID     string `json:"session_id"`
	ForkedFromID  string `json:"forked_from_id"`
	CWD           string `json:"cwd"`
	Timestamp     string `json:"timestamp"`
	Originator    string `json:"originator"`
	CLIVersion    string `json:"cli_version"`
	Source        string `json:"source"`
	ModelProvider string `json:"model_provider"`
}

type eventMsgPayload struct {
	Type       string          `json:"type"`
	Info       *tokenInfo      `json:"info"`
	RateLimits json.RawMessage `json:"rate_limits"`
	TurnID     string          `json:"turn_id"`
	Message    string          `json:"message"`
	StartedAt  int64           `json:"started_at"`
	Item       json.RawMessage `json:"item"`
}

type tokenInfo struct {
	Last               tokenUsageFields `json:"last_token_usage"`
	Total              tokenUsageFields `json:"total_token_usage"`
	ModelContextWindow int64            `json:"model_context_window"`
}

type tokenUsageFields struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	CacheReadInputTokens  int64 `json:"cache_read_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

func (t tokenUsageFields) asUsage() TokenUsage {
	cached := t.CachedInputTokens
	if cached == 0 && t.CacheReadInputTokens != 0 {
		cached = t.CacheReadInputTokens
	}
	total := t.TotalTokens
	if total == 0 {
		total = t.InputTokens + t.OutputTokens
	}
	return TokenUsage{
		Input:       t.InputTokens,
		CachedInput: cached,
		Output:      t.OutputTokens,
		Reasoning:   t.ReasoningOutputTokens,
		Total:       total,
	}
}

func (t tokenUsageFields) present() bool {
	return t.InputTokens != 0 || t.OutputTokens != 0 || t.TotalTokens != 0 || t.ReasoningOutputTokens != 0 || t.CachedInputTokens != 0 || t.CacheReadInputTokens != 0
}

type rateLimitsPayload struct {
	LimitID   string           `json:"limit_id"`
	Primary   *rateLimitWindow `json:"primary"`
	Secondary *rateLimitWindow `json:"secondary"`
	Credits   *creditsPayload  `json:"credits"`
	PlanType  string           `json:"plan_type"`
}

type rateLimitWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes int     `json:"window_minutes"`
	ResetsAt      int64   `json:"resets_at"`
}

type creditsPayload struct {
	HasCredits bool            `json:"has_credits"`
	Unlimited  bool            `json:"unlimited"`
	Balance    json.RawMessage `json:"balance"`
}

type responseItemPayload struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Name      string          `json:"name"`
	Content   json.RawMessage `json:"content"`
	Arguments string          `json:"arguments"`
	CallID    string          `json:"call_id"`
}

type turnContextPayload struct {
	Model  string `json:"model"`
	TurnID string `json:"turn_id"`
}

type compactedPayload struct {
	Message                string          `json:"message"`
	WindowNumber           int             `json:"window_number"`
	LatestTokenUsageRecord json.RawMessage `json:"latest_token_usage_record"`
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type parseState struct {
	ro        *Rollout
	model     string
	turnID    string
	prevTotal *TokenUsage
	prevLast  *TokenUsage
	haveTotal bool
}

// ParseFile reads a Codex rollout JSONL file. Unknown types and oversized
// compacted lines are skipped without failing the rest of the file.
func ParseFile(path string) (*Rollout, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(path, f)
}

// Parse reads a rollout from r.
func Parse(path string, r io.Reader) (*Rollout, error) {
	br := bufio.NewReaderSize(r, 256*1024)
	st := &parseState{ro: &Rollout{Session: Session{File: path}}}
	for {
		line, truncated, err := readLineLimited(br, MaxJSONLine)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if truncated {
			st.ingestTruncated(line)
			continue
		}
		st.ingestLine(line)
	}
	if st.ro.Session.ID == "" {
		st.ro.Session.ID = sessionIDFromFilename(path)
	}
	return st.ro, nil
}

func (st *parseState) ingestTruncated(prefix []byte) {
	typ := firstCapture(typeFieldRe, prefix)
	ts := ParseTime(firstCapture(tsFieldRe, prefix))
	switch typ {
	case "compacted":
		st.ro.Compactions = append(st.ro.Compactions, Compaction{
			SessionID:      st.ro.Session.ID,
			Time:           ts,
			SkippedPayload: true,
		})
	default:
		// unknown / oversized non-compaction: ignore
	}
}

func (st *parseState) ingestLine(line []byte) {
	var env envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return
	}
	ts := ParseTime(env.Timestamp)
	switch env.Type {
	case "session_meta":
		st.ingestSessionMeta(ts, env.Payload)
	case "event_msg":
		st.ingestEventMsg(ts, env.Payload)
	case "response_item":
		st.ingestResponseItem(ts, env.Payload)
	case "turn_context":
		st.ingestTurnContext(ts, env.Payload)
	case "compacted":
		st.ingestCompacted(ts, env.Payload)
	}
}

func (st *parseState) ingestSessionMeta(ts time.Time, raw json.RawMessage) {
	var p sessionMetaPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	id := p.ID
	if id == "" {
		id = p.SessionID
	}
	started := ParseTime(p.Timestamp)
	if started.IsZero() {
		started = ts
	}
	st.ro.Session.ID = id
	st.ro.Session.ParentID = p.ForkedFromID
	st.ro.Session.CWD = p.CWD
	st.ro.Session.Originator = p.Originator
	st.ro.Session.Source = p.Source
	st.ro.Session.Provider = p.ModelProvider
	st.ro.Session.StartedAt = started
}

func (st *parseState) ingestEventMsg(ts time.Time, raw json.RawMessage) {
	var p eventMsgPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	switch p.Type {
	case "token_count":
		st.ingestTokenCount(ts, p)
	case "task_started":
		if p.TurnID != "" {
			st.turnID = p.TurnID
		}
		st.ro.Turns = append(st.ro.Turns, TurnStart{
			SessionID: st.ro.Session.ID,
			Time:      ts,
			TurnID:    st.turnID,
			Model:     st.model,
		})
	case "user_message":
		if text := strings.TrimSpace(p.Message); text != "" && !isNoisePrompt(text) {
			st.ro.Prompts = append(st.ro.Prompts, Prompt{
				SessionID: st.ro.Session.ID,
				Time:      ts,
				TurnID:    st.turnID,
				Text:      firstLine(text),
			})
		}
	}
}

func (st *parseState) ingestTokenCount(ts time.Time, p eventMsgPayload) {
	if snap := parseQuotaSnapshot(st.ro.Session.ID, ts, p.RateLimits); snap != nil {
		st.ro.Quotas = append(st.ro.Quotas, *snap)
	}
	if p.Info == nil {
		return
	}
	total := p.Info.Total.asUsage()
	last := p.Info.Last.asUsage()
	hasTotal := p.Info.Total.present()
	hasLast := p.Info.Last.present()

	var delta TokenUsage
	emitted := false
	if hasTotal {
		if st.haveTotal && st.prevTotal != nil && total.Equal(*st.prevTotal) {
			// re-emit: quota-only, do not double-count
		} else {
			if st.haveTotal && st.prevTotal != nil {
				delta = total.Sub(*st.prevTotal)
			} else {
				delta = total
			}
			if !delta.IsZero() {
				emitted = true
			}
			cp := total
			st.prevTotal = &cp
			st.haveTotal = true
		}
	} else if hasLast {
		if st.prevLast != nil && last.Equal(*st.prevLast) {
			// re-emit of last_token_usage
		} else {
			delta = last
			if !delta.IsZero() {
				emitted = true
			}
			cp := last
			st.prevLast = &cp
		}
	}
	if hasLast {
		cp := last
		st.prevLast = &cp
	}
	if !emitted {
		return
	}
	st.ro.Usage = append(st.ro.Usage, UsageEvent{
		SessionID:     st.ro.Session.ID,
		Time:          ts,
		TurnID:        st.turnID,
		Model:         st.model,
		Delta:         delta,
		Last:          last,
		ContextWindow: p.Info.ModelContextWindow,
	})
}

func (st *parseState) ingestResponseItem(ts time.Time, raw json.RawMessage) {
	var p responseItemPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	switch p.Type {
	case "function_call", "custom_tool_call":
		name := p.Name
		if name == "" {
			name = p.Type
		}
		st.ro.Tools = append(st.ro.Tools, ToolCall{
			SessionID: st.ro.Session.ID,
			Time:      ts,
			TurnID:    st.turnID,
			Name:      name,
		})
	case "message":
		if !strings.EqualFold(p.Role, "user") {
			return
		}
		text := extractText(p.Content)
		if text == "" || isNoisePrompt(text) {
			return
		}
		st.ro.Prompts = append(st.ro.Prompts, Prompt{
			SessionID: st.ro.Session.ID,
			Time:      ts,
			TurnID:    st.turnID,
			Text:      firstLine(text),
		})
	}
}

func (st *parseState) ingestTurnContext(ts time.Time, raw json.RawMessage) {
	var p turnContextPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	if p.Model != "" {
		st.model = p.Model
	}
	if p.TurnID != "" {
		st.turnID = p.TurnID
	}
}

func (st *parseState) ingestCompacted(ts time.Time, raw json.RawMessage) {
	c := Compaction{SessionID: st.ro.Session.ID, Time: ts}
	var p compactedPayload
	if err := json.Unmarshal(raw, &p); err == nil && len(p.LatestTokenUsageRecord) > 0 {
		var rec struct {
			Usage tokenUsageFields `json:"usage"`
		}
		if json.Unmarshal(p.LatestTokenUsageRecord, &rec) == nil {
			c.ContextAfter = rec.Usage.InputTokens
			if c.ContextAfter == 0 {
				c.ContextAfter = rec.Usage.TotalTokens
			}
		}
	}
	st.ro.Compactions = append(st.ro.Compactions, c)
}

func parseQuotaSnapshot(sessionID string, ts time.Time, raw json.RawMessage) *QuotaSnapshot {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var p rateLimitsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	snap := &QuotaSnapshot{SessionID: sessionID, Time: ts, PlanType: p.PlanType}
	assignWindow := func(w *rateLimitWindow) {
		if w == nil {
			return
		}
		win := &Window{
			UsedPercent: w.UsedPercent,
			Minutes:     w.WindowMinutes,
		}
		if w.ResetsAt > 0 {
			win.ResetAt = time.Unix(w.ResetsAt, 0).UTC()
		}
		switch ClassifyWindow(w.WindowMinutes) {
		case WindowFiveHour:
			snap.FiveHour = win
		case WindowWeekly:
			snap.Weekly = win
		}
	}
	assignWindow(p.Primary)
	assignWindow(p.Secondary)
	if p.Credits != nil {
		bal := strings.TrimSpace(string(p.Credits.Balance))
		bal = strings.Trim(bal, `"`)
		if bal == "null" {
			bal = ""
		}
		snap.Credits = &CreditsInfo{
			HasCredits: p.Credits.HasCredits,
			Unlimited:  p.Credits.Unlimited,
			Balance:    bal,
		}
	}
	if snap.FiveHour == nil && snap.Weekly == nil && snap.Credits == nil {
		return nil
	}
	return snap
}

// WindowKind is the duration class of a rate-limit window.
type WindowKind int

const (
	WindowUnknown WindowKind = iota
	WindowFiveHour
	WindowWeekly
)

// ClassifyWindow maps window_minutes onto 5h vs weekly by duration, not slot.
func ClassifyWindow(minutes int) WindowKind {
	if minutes <= 0 {
		return WindowUnknown
	}
	const fiveH = 300
	const week = 7 * 24 * 60
	d5 := absInt(minutes - fiveH)
	dW := absInt(minutes - week)
	if minutes >= 48*60 && dW <= d5 {
		return WindowWeekly
	}
	if minutes <= 12*60 {
		return WindowFiveHour
	}
	if dW < d5 {
		return WindowWeekly
	}
	return WindowFiveHour
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var parts []contentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(p.Text)
			}
		}
		return strings.TrimSpace(b.String())
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func isNoisePrompt(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	lower := strings.ToLower(t)
	switch {
	case strings.HasPrefix(t, "<"):
		return true
	case strings.HasPrefix(lower, "you are codex"):
		return true
	case strings.HasPrefix(t, "# AGENTS.md"):
		return true
	case strings.HasPrefix(t, "# Files mentioned"):
		return true
	case strings.Contains(lower, "the following is the codex agent history"):
		return true
	default:
		return false
	}
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = strings.TrimSpace(text[:i])
	}
	text = strings.Join(strings.Fields(text), " ")
	return truncateRunes(text, 72)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if n <= 1 || len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func firstCapture(re *regexp.Regexp, b []byte) string {
	m := re.FindSubmatch(b)
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}

// ParseTime parses RFC3339 / RFC3339Nano timestamps; zero on failure.
func ParseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func sessionIDFromFilename(path string) string {
	base := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		base = path[i+1:]
	}
	base = strings.TrimPrefix(base, "rollout-")
	base = strings.TrimSuffix(base, ".jsonl")
	if i := strings.LastIndex(base, "-"); i >= 0 && i+1 < len(base) {
		rest := base[i+1:]
		if len(rest) >= 8 {
			return rest
		}
	}
	return base
}

func readLineLimited(r *bufio.Reader, limit int) (line []byte, truncated bool, err error) {
	const keep = 4096
	for {
		chunk, isPrefix, readErr := r.ReadLine()
		if len(chunk) > 0 {
			if !truncated && len(line)+len(chunk) <= limit {
				line = append(line, chunk...)
			} else {
				if !truncated {
					remain := keep - len(line)
					if remain > 0 {
						if remain > len(chunk) {
							remain = len(chunk)
						}
						line = append(line, chunk[:remain]...)
					}
				}
				truncated = true
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				if len(line) == 0 && !truncated {
					return nil, false, io.EOF
				}
				return line, truncated, nil
			}
			return line, truncated, readErr
		}
		if !isPrefix {
			return line, truncated, nil
		}
	}
}
