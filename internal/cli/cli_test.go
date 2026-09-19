package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/archive"
	"github.com/jerryxff26-alt/session-top/internal/codex"
	"github.com/jerryxff26-alt/session-top/internal/jev"
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

func TestVersion(t *testing.T) {
	out := run(t, "--version")
	if !strings.Contains(out, "session-top") || !strings.Contains(out, Version) {
		t.Fatalf("--version: %q", out)
	}
	out = run(t, "version")
	if !strings.Contains(out, Version) {
		t.Fatalf("version: %q", out)
	}
	help := run(t, "--help")
	if !strings.Contains(help, Version) {
		t.Fatalf("help missing version: %q", help)
	}
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
	for _, want := range []string{"session-top distill", "--cwd", "--session", "--since", "--from", "--to", "--jev", "--archive-low", "--apply", "--json"} {
		if !strings.Contains(out, want) {
			t.Errorf("distill --help missing %q\n%s", want, out)
		}
	}
}

func TestDistillProjectDotIsAbs(t *testing.T) {
	now := time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC)
	runOpts, err := parseDistillArgs([]string{"--project", "."}, now)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Clean(want)
	if runOpts.Selection.Project != want {
		t.Fatalf("project %q want abs %q", runOpts.Selection.Project, want)
	}
	if !filepath.IsAbs(runOpts.Selection.Project) {
		t.Fatalf("project is not absolute: %q", runOpts.Selection.Project)
	}
}

func TestParseDistillJevAndSession(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	runOpts, err := parseDistillArgs([]string{"--session", "0001", "--jev", "--json"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if runOpts.Selection.SessionID != "0001" || !runOpts.AsJSON || !runOpts.UseJev {
		t.Fatalf("opts=%+v", runOpts)
	}
}

func TestParseDistillArchiveFlagDependencies(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	if _, err := parseDistillArgs([]string{"--archive-low"}, now); err == nil || !strings.Contains(err.Error(), "requires --jev") {
		t.Fatalf("--archive-low error=%v", err)
	}
	if _, err := parseDistillArgs([]string{"--apply"}, now); err == nil || !strings.Contains(err.Error(), "requires --archive-low") {
		t.Fatalf("--apply error=%v", err)
	}
	runOpts, err := parseDistillArgs([]string{"--jev", "--archive-low", "--apply"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !runOpts.UseJev || !runOpts.ArchiveLow || !runOpts.Apply {
		t.Fatalf("opts=%+v", runOpts)
	}
}

type fakeJevAssessor struct {
	state any
}

func (f *fakeJevAssessor) Assess(_ context.Context, state any) (jev.Assessment, error) {
	f.state = state
	return jev.Assessment{
		Model:             "jev-latest",
		DistillPriority:   jev.ChoiceAnswer{Choice: "high"},
		PrimaryValue:      jev.ChoiceAnswer{Choice: "correction_value"},
		ReusableKnowledge: jev.NoulAnswer{Noul: 0.8},
		VerifiedEvidence:  jev.NoulAnswer{Noul: 0.7},
		CorrectionValue:   jev.NoulAnswer{Noul: 0.9},
		Usage:             jev.Usage{InputTokens: 123},
	}, nil
}

func TestAssessWithJevAnnotatesDigestAndRequiresReviewForIncompleteContext(t *testing.T) {
	d := usage.DistillDigest{Sessions: []usage.DistillExtract{{
		SessionID: "session-1",
		CWD:       "/workspace/demo",
		Goal:      "Fix parser",
		Context: []usage.DistillContextItem{
			{Kind: "user", Text: "read the whole conversation"},
			{Kind: "assistant", Text: "implemented and tested"},
		},
		ContextCoverage: usage.DistillContextCoverage{SourceItems: 3, IncludedItems: 2, OmittedItems: 1},
	}}}
	fake := &fakeJevAssessor{}
	if err := assessWithJev(context.Background(), fake, &d); err != nil {
		t.Fatal(err)
	}
	got := d.Sessions[0].Jev
	if got == nil || got.Priority != "high" || got.PrimaryValue != "correction_value" || !got.ReviewRequired || got.InputTokens != 123 {
		t.Fatalf("assessment=%+v", got)
	}
	if fake.state == nil {
		t.Fatal("structured state was not sent")
	}
}

func TestDistillJevRequiresKey(t *testing.T) {
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	now := time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC)
	var buf bytes.Buffer
	err := Run(&buf, []string{"distill", "--cwd", "/workspace/oauth-app", "--session", "0001", "--jev"}, fixtureHome(t), now)
	if !errors.Is(err, jev.ErrMissingAPIKey) {
		t.Fatalf("error=%v want ErrMissingAPIKey", err)
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

type lowJevAssessor struct{}

func (lowJevAssessor) Assess(context.Context, any) (jev.Assessment, error) {
	return jev.Assessment{
		Model:             "jev-test",
		DistillPriority:   jev.ChoiceAnswer{Choice: "low"},
		PrimaryValue:      jev.ChoiceAnswer{Choice: "none"},
		ReusableKnowledge: jev.NoulAnswer{Noul: 0.1},
		VerifiedEvidence:  jev.NoulAnswer{Noul: 0.2},
		CorrectionValue:   jev.NoulAnswer{Noul: 0.05},
	}, nil
}

type recordingArchiver struct {
	ids []string
	err error
}

func (r *recordingArchiver) Archive(_ context.Context, id string) (*archive.Result, error) {
	r.ids = append(r.ids, id)
	if r.err != nil {
		return nil, r.err
	}
	return &archive.Result{SessionID: id}, nil
}

func archiveTestAnalysis(now time.Time, project string) *usage.Analysis {
	conversation := []codex.ConversationItem{
		{Kind: "user", Text: "check the result"},
		{Kind: "assistant", Text: "nothing reusable was produced"},
	}
	return &usage.Analysis{Sessions: []usage.SessionSummary{
		{ID: "old-low", Title: "Low-value attempt", CWD: project, StartedAt: now.Add(-8 * 24 * time.Hour), Conversation: conversation},
		{ID: "recent-low", Title: "Recent attempt", CWD: project, StartedAt: now.Add(-24 * time.Hour), Conversation: conversation},
	}}
}

func TestRunDistillArchiveLowDryRunDoesNotArchive(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	project := t.TempDir()
	archiver := &recordingArchiver{}
	deps := distillDeps{
		newJev:   func() (jevAssessor, error) { return lowJevAssessor{}, nil },
		archiver: archiver,
		getenv:   func(string) string { return "" },
	}
	var buf bytes.Buffer
	err := runDistillWithDeps(&buf, []string{"--cwd", project, "--jev", "--archive-low", "--json"}, archiveTestAnalysis(now, project), now, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(archiver.ids) != 0 {
		t.Fatalf("dry-run archived %v", archiver.ids)
	}
	out := buf.String()
	if !strings.Contains(out, `"status": "candidate"`) || !strings.Contains(out, `"status": "protected"`) {
		t.Fatalf("archive decisions missing:\n%s", out)
	}
}

func TestRunDistillArchiveLowApplyArchivesOnlyEligible(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	project := t.TempDir()
	archiver := &recordingArchiver{}
	deps := distillDeps{
		newJev:   func() (jevAssessor, error) { return lowJevAssessor{}, nil },
		archiver: archiver,
		getenv:   func(string) string { return "" },
	}
	var buf bytes.Buffer
	err := runDistillWithDeps(&buf, []string{"--cwd", project, "--jev", "--archive-low", "--apply", "--json"}, archiveTestAnalysis(now, project), now, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(archiver.ids) != 1 || archiver.ids[0] != "old-low" {
		t.Fatalf("archived ids=%v", archiver.ids)
	}
	out := buf.String()
	if !strings.Contains(out, `"status": "archived"`) || !strings.Contains(out, `"status": "protected"`) {
		t.Fatalf("archive results missing:\n%s", out)
	}
}
