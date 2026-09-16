package app

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/session-top/session-top/internal/tui"
	"github.com/session-top/session-top/internal/usage"
)

func fixtureHome(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "codex-home"))
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	now := time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC)
	var buf bytes.Buffer
	if err := Run(&buf, args, fixtureHome(t), now); err != nil {
		t.Fatalf("run %v: %v\n%s", args, err, buf.String())
	}
	return buf.String()
}

func TestOverviewWhySessionsDetail(t *testing.T) {
	overview := run(t)
	for _, want := range []string{
		"SESSION TOP",
		"5h",
		"Weekly",
		"61.4%",
		"84%",
		"TODAY",
		"TOP QUOTA CONSUMERS",
		"Fix OAuth callback",
		"7.3%",
		"Refactor database",
		"5.8%",
		"Research MCP auth",
		"3.1%",
		"Update README",
		"0.4%",
		"Attribution: ambiguous",
	} {
		if !strings.Contains(overview, want) {
			t.Errorf("overview missing %q\n%s", want, overview)
		}
	}

	why := run(t, "why")
	for _, want := range []string{
		"Quota used",
		"18.6%",
		"Observed",
		"Potential causes",
		"Fix OAuth callback",
	} {
		if !strings.Contains(why, want) {
			t.Errorf("why missing %q\n%s", want, why)
		}
	}
	for _, bad := range []string{
		"caused by",
		"was caused",
		"due to compaction",
		"your quota drop was",
	} {
		if strings.Contains(strings.ToLower(why), strings.ToLower(bad)) {
			t.Errorf("why asserted proven causation %q\n%s", bad, why)
		}
	}

	sessions := run(t, "sessions")
	for _, want := range []string{
		"QUOTA Δ",
		"TOKENS",
		"TURNS",
		"Fix OAuth callback",
		"-7.3%",
		"921K",
		"Refactor database",
		"-5.8%",
		"1.40M",
		"codex exec batch",
		"50K",
		"ambiguous",
	} {
		if !strings.Contains(sessions, want) {
			t.Errorf("sessions missing %q\n%s", want, sessions)
		}
	}

	detail := run(t, "session", "01970000-0000-7000-8000-000000000001")
	for _, want := range []string{
		"Fix OAuth callback",
		"prompt",
		"agent follow-up",
		"compaction",
		"Expensive turn",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail missing %q\n%s", want, detail)
		}
	}

	a, err := Load(fixtureHome(t), time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tui.Overview(a), "Fix OAuth callback") {
		t.Fatal("shipped Overview renderer missed consumer")
	}
	if usage.FindSession(a, "000000000001") == nil {
		t.Fatal("suffix lookup failed")
	}
}

func TestNoOpenAIKeyRequired(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	out := run(t)
	if !strings.Contains(out, "SESSION TOP") {
		t.Fatalf("ran without API key but got:\n%s", out)
	}
}
