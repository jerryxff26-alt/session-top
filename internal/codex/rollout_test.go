package codex

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixtureHome(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "codex-home"))
}

func TestDiscoverAndParseFixtures(t *testing.T) {
	home := fixtureHome(t)
	files, err := Discover(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 7 {
		t.Fatalf("discovered %d files, want 7", len(files))
	}
	var oauth *Rollout
	var nullq *Rollout
	for _, f := range files {
		r, err := ParseFile(f)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		switch {
		case strings.HasSuffix(r.Session.ID, "000000000001"):
			oauth = r
		case strings.HasSuffix(r.Session.ID, "000000000005"):
			nullq = r
		}
	}
	if oauth == nil {
		t.Fatal("oauth session missing")
	}
	if oauth.Session.ID != "01970000-0000-7000-8000-000000000001" {
		t.Fatalf("id %s", oauth.Session.ID)
	}
	var observed int64
	for _, e := range oauth.Usage {
		observed += e.Delta.Total
	}
	if observed != 920500 {
		t.Fatalf("oauth observed tokens %d (re-emits double-counted?), want 920500", observed)
	}
	if len(oauth.Usage) != 3 {
		t.Fatalf("oauth usage events %d, want 3 (re-emit must be dropped)", len(oauth.Usage))
	}
	if len(oauth.Compactions) < 2 {
		t.Fatalf("oauth compactions %d, want >=2 (small + oversized)", len(oauth.Compactions))
	}
	skipped := 0
	for _, c := range oauth.Compactions {
		if c.SkippedPayload {
			skipped++
		}
	}
	if skipped < 1 {
		t.Fatalf("oversized compacted line was not skipped; MaxJSONLine=%d", MaxJSONLine)
	}
	if len(oauth.Tools) != 5 {
		t.Fatalf("oauth tools %d, want 5", len(oauth.Tools))
	}
	if len(oauth.Quotas) == 0 {
		t.Fatal("oauth missing quota snapshots")
	}

	if nullq == nil {
		t.Fatal("null quota session missing")
	}
	var nt int64
	for _, e := range nullq.Usage {
		nt += e.Delta.Total
	}
	if nt != 50000 {
		t.Fatalf("null-quota observed %d, want 50000 (re-emit?)", nt)
	}
	if len(nullq.Quotas) != 0 {
		t.Fatalf("null rate_limits produced %d quota snapshots", len(nullq.Quotas))
	}
	if len(nullq.Turns) != 2 {
		t.Fatalf("null-quota turns %d, want 2", len(nullq.Turns))
	}
}

func TestOversizedCompactedDoesNotCrash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-huge.jsonl")
	meta := `{"timestamp":"2026-09-16T14:00:00.000Z","type":"session_meta","payload":{"id":"huge-1","cwd":"/tmp"}}` + "\n"
	huge := `{"timestamp":"2026-09-16T14:00:01.000Z","type":"compacted","payload":{"message":"` + strings.Repeat("B", 2_000_000) + `"}}` + "\n"
	token := `{"timestamp":"2026-09-16T14:00:02.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11},"last_token_usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11},"model_context_window":1000},"rate_limits":null}}` + "\n"
	if err := os.WriteFile(path, []byte(meta+huge+token), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := ParseFile(path)
	if err != nil {
		t.Fatalf("parse oversized: %v", err)
	}
	if r.Session.ID != "huge-1" {
		t.Fatalf("id %s", r.Session.ID)
	}
	if len(r.Compactions) != 1 || !r.Compactions[0].SkippedPayload {
		t.Fatalf("compactions=%d skipped=%v", len(r.Compactions), len(r.Compactions) > 0 && r.Compactions[0].SkippedPayload)
	}
	if len(r.Usage) != 1 || r.Usage[0].Delta.Total != 11 {
		t.Fatalf("usage after oversized line: %+v", r.Usage)
	}
}

func TestClassifyWindowByDurationNotSlot(t *testing.T) {
	if ClassifyWindow(300) != WindowFiveHour {
		t.Fatalf("300 minutes: %v", ClassifyWindow(300))
	}
	if ClassifyWindow(299) != WindowFiveHour {
		t.Fatalf("299 minutes: %v", ClassifyWindow(299))
	}
	if ClassifyWindow(10080) != WindowWeekly {
		t.Fatalf("10080 minutes: %v", ClassifyWindow(10080))
	}
	if ClassifyWindow(10079) != WindowWeekly {
		t.Fatalf("10079 minutes: %v", ClassifyWindow(10079))
	}
}

func TestNoisePromptAndRuneTruncate(t *testing.T) {
	if !isNoisePrompt("<recommended_plugins>\nfoo") {
		t.Fatal("xml wrappers should be noise")
	}
	if !isNoisePrompt("# AGENTS.md instructions") {
		t.Fatal("AGENTS.md dump should be noise")
	}
	if isNoisePrompt("Fix OAuth callback") {
		t.Fatal("real user title was treated as noise")
	}
	s := truncateRunes("帮我整理一个问题列表继续", 8)
	if strings.Contains(s, "\uFFFD") {
		t.Fatalf("truncated into replacement char: %q", s)
	}
	if []rune(s)[len([]rune(s))-1] != '…' {
		t.Fatalf("got %q", s)
	}
}

func TestConversationContextIncludesAssistantAndToolEvidence(t *testing.T) {
	body := strings.Join([]string{
		`{"timestamp":"2026-09-19T01:00:00Z","type":"session_meta","payload":{"id":"ctx-1","cwd":"/workspace/demo"}}`,
		`{"timestamp":"2026-09-19T01:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Please inspect the whole conversation, not just this prompt."}]}}`,
		`{"timestamp":"2026-09-19T01:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I will verify the implementation and the test output."}]}}`,
		`{"timestamp":"2026-09-19T01:00:03Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-1","arguments":"{\"cmd\":\"go test ./...\",\"api_key\":\"super-secret\"}"}}`,
		`{"timestamp":"2026-09-19T01:00:04Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"Process exited with code 1\\nFinal output:\\nerror: assertion failed"}}`,
	}, "\n") + "\n"
	r, err := Parse("rollout-ctx-1.jsonl", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Conversation) != 3 {
		t.Fatalf("conversation items=%d want 3: %+v", len(r.Conversation), r.Conversation)
	}
	if r.Conversation[0].Kind != "user" || !strings.Contains(r.Conversation[0].Text, "whole conversation") {
		t.Fatalf("user context missing: %+v", r.Conversation[0])
	}
	if r.Conversation[1].Kind != "assistant" || !strings.Contains(r.Conversation[1].Text, "verify") {
		t.Fatalf("assistant context missing: %+v", r.Conversation[1])
	}
	tool := r.Conversation[2]
	if tool.Kind != "tool" || tool.Tool != "exec_command" || !tool.Failed || !strings.Contains(tool.Text, "go test ./...") || !strings.Contains(tool.Text, "assertion failed") {
		t.Fatalf("tool evidence missing: %+v", tool)
	}
	if strings.Contains(tool.Text, "super-secret") || !strings.Contains(tool.Text, "<redacted>") {
		t.Fatalf("secret was not redacted: %q", tool.Text)
	}
	if len(r.Prompts) != 1 || len([]rune(r.Prompts[0].Text)) <= 30 {
		t.Fatalf("full prompt was not retained: %+v", r.Prompts)
	}
}

func TestUnknownLineDoesNotCrash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-x.jsonl")
	body := strings.Join([]string{
		`{"timestamp":"2026-09-16T14:00:00.000Z","type":"session_meta","payload":{"id":"x"}}`,
		`{"timestamp":"2026-09-16T14:00:01.000Z","type":"world_state","payload":{"nope":true}}`,
		`{not json`,
		`{"timestamp":"2026-09-16T14:00:02.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":5,"total_tokens":5},"last_token_usage":{"input_tokens":5,"total_tokens":5}},"rate_limits":null}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Usage) != 1 {
		t.Fatalf("usage %d", len(r.Usage))
	}
}
