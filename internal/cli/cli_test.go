package cli

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/tui"
	"github.com/jerryxff26-alt/session-top/internal/usage"
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
		"Token rank and official quota rank differ",
		"Continuation follow-ups",
		"Fork cluster",
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
		"QUOTA",
		"TOKENS",
		"TURNS",
		"ID",
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
		"AUTOPSY (observed)",
		"Patterns",
		"input",
		"cached",
		"output",
		"of input",
		"continuation-style follow-ups",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail missing %q\n%s", want, detail)
		}
	}
	if !strings.Contains(detail, "%") {
		t.Errorf("autopsy missing mix percents:\n%s", detail)
	}
	for _, bad := range []string{"caused by", "was caused"} {
		if strings.Contains(strings.ToLower(detail), bad) {
			t.Errorf("session autopsy asserted proven causation %q\n%s", bad, detail)
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

func TestDistillHelpExitZero(t *testing.T) {
	var buf bytes.Buffer
	err := Run(&buf, []string{"distill", "--help"}, fixtureHome(t), time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("distill --help: %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{"session-top distill", "--cwd", "--since", "--from", "--to", "--json"} {
		if !strings.Contains(out, want) {
			t.Errorf("distill --help missing %q\n%s", want, out)
		}
	}
}

func TestDistillProjectDotIsAbs(t *testing.T) {
	now := time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC)
	opts, _, err := parseDistillArgs([]string{"--project", "."}, now)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Clean(want)
	if opts.Project != want {
		t.Fatalf("project %q want abs %q", opts.Project, want)
	}
	if !filepath.IsAbs(opts.Project) {
		t.Fatalf("project is not absolute: %q", opts.Project)
	}
}

func TestDistillSameDayToIncludesFixture(t *testing.T) {
	out := run(t, "distill", "--cwd", "/workspace/oauth-app", "--from", "2026-09-16", "--to", "2026-09-16")
	if !strings.Contains(out, "Fix OAuth callback") {
		t.Fatalf("same-day --to dropped in-window session:\n%s", out)
	}
	if strings.Contains(out, "outside-window 1") && !strings.Contains(out, "Fix OAuth callback") {
		t.Fatal("oauth marked outside window")
	}
}

func TestDistillShippedPath(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	out := run(t, "distill", "--cwd", "/workspace/oauth-app", "--from", "2026-09-16", "--to", "2026-09-17")
	for _, want := range []string{
		"DISTILL (observed digest)",
		"/workspace/oauth-app",
		"Fix OAuth callback",
		"01970000-0000-7000-8000-000000000001",
		"tools",
		"observed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("distill missing %q\n%s", want, out)
		}
	}
	for _, bad := range []string{
		"Refactor database",
		"Overlap Alpha",
		"Overlap Beta",
		"codex exec batch",
		strings.Repeat("A", 200),
	} {
		if strings.Contains(out, bad) {
			t.Errorf("distill mixed unrelated project or raw payload %q\n%s", bad, out)
		}
	}
	js := run(t, "distill", "--cwd", "/workspace/oauth-app", "--from", "2026-09-16", "--to", "2026-09-17", "--json")
	if !strings.Contains(js, `"session_id"`) || !strings.Contains(js, "Fix OAuth callback") {
		t.Fatalf("json digest:\n%s", js)
	}
	if strings.Contains(js, strings.Repeat("A", 200)) {
		t.Fatal("json included compacted payload")
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
