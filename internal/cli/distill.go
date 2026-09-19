package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/archive"
	"github.com/jerryxff26-alt/session-top/internal/jev"
	"github.com/jerryxff26-alt/session-top/internal/tui"
	"github.com/jerryxff26-alt/session-top/internal/usage"
)

const maxJevSessions = 10

type jevAssessor interface {
	Assess(context.Context, any) (jev.Assessment, error)
}

type distillRunOptions struct {
	Selection  usage.DistillOpts
	AsJSON     bool
	UseJev     bool
	ArchiveLow bool
	Apply      bool
}

type distillDeps struct {
	newJev   func() (jevAssessor, error)
	archiver sessionArchiver
	getenv   func(string) string
}

func defaultDistillDeps() distillDeps {
	return distillDeps{
		newJev: func() (jevAssessor, error) {
			return jev.NewFromEnv()
		},
		archiver: archive.New(),
		getenv:   os.Getenv,
	}
}

func runDistill(w io.Writer, args []string, a *usage.Analysis, now time.Time) error {
	return runDistillWithDeps(w, args, a, now, defaultDistillDeps())
}

func runDistillWithDeps(w io.Writer, args []string, a *usage.Analysis, now time.Time, deps distillDeps) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			_, err := io.WriteString(w, DistillHelp())
			return err
		}
	}
	runOpts, err := parseDistillArgs(args, now)
	if err != nil {
		return err
	}
	d := usage.Distill(a, runOpts.Selection)
	if runOpts.UseJev && len(d.Sessions) > 0 {
		if len(d.Sessions) > maxJevSessions {
			return fmt.Errorf("distill --jev matched %d sessions; narrow --since/--from/--to or pass --session (maximum %d per run)", len(d.Sessions), maxJevSessions)
		}
		if deps.newJev == nil {
			return fmt.Errorf("distill: Jev assessor is unavailable")
		}
		client, err := deps.newJev()
		if err != nil {
			return err
		}
		if err := assessWithJev(context.Background(), client, &d); err != nil {
			return err
		}
	}
	if runOpts.ArchiveLow {
		currentSessionID := ""
		if deps.getenv != nil {
			currentSessionID = strings.TrimSpace(deps.getenv("CODEX_THREAD_ID"))
		}
		planLowValueArchives(&d, now, currentSessionID)
		if runOpts.Apply {
			if err := applyArchiveCandidates(context.Background(), deps.archiver, &d); err != nil {
				return err
			}
		}
	}
	if runOpts.AsJSON {
		return tui.WriteDistillJSON(w, d)
	}
	return tui.WriteDistill(w, d)
}

func assessWithJev(ctx context.Context, assessor jevAssessor, d *usage.DistillDigest) error {
	if d == nil {
		return nil
	}
	for i := range d.Sessions {
		s := &d.Sessions[i]
		state := struct {
			SessionID       string                       `json:"session_id"`
			CWD             string                       `json:"cwd"`
			Goal            string                       `json:"goal"`
			Corrections     []string                     `json:"corrections,omitempty"`
			Tools           []usage.ToolCount            `json:"tools,omitempty"`
			Context         []usage.DistillContextItem   `json:"context"`
			ContextCoverage usage.DistillContextCoverage `json:"context_coverage"`
		}{
			SessionID:       s.SessionID,
			CWD:             s.CWD,
			Goal:            s.Goal,
			Corrections:     s.Corrections,
			Tools:           s.Tools,
			Context:         s.Context,
			ContextCoverage: s.ContextCoverage,
		}
		result, err := assessor.Assess(ctx, state)
		if err != nil {
			return fmt.Errorf("distill: Jev assessment for %s: %w", s.SessionID, err)
		}
		s.Jev = &usage.DistillAssessment{
			Model:             result.Model,
			Priority:          result.DistillPriority.Choice,
			PrimaryValue:      result.PrimaryValue.Choice,
			ReusableKnowledge: result.ReusableKnowledge.Noul,
			VerifiedEvidence:  result.VerifiedEvidence.Noul,
			CorrectionValue:   result.CorrectionValue.Noul,
			InputTokens:       int64(result.Usage.InputTokens),
			ReviewRequired:    !s.ContextCoverage.Complete || result.DistillPriority.Choice == "review",
		}
	}
	return nil
}

// DistillHelp is the distill subcommand usage (exit 0 via --help).
func DistillHelp() string {
	return `usage: session-top distill [--cwd PATH] [--session ID] [--since 7d] [--from DATE] [--to DATE] [--jev] [--archive-low [--apply]] [--json]

  --jev          rank bounded session context with Jev (maximum 10 sessions)
  --archive-low  preview candidate/protected decisions; requires --jev
  --apply        archive candidates with Codex native archive; requires --archive-low
`
}

func parseDistillArgs(args []string, now time.Time) (distillRunOptions, error) {
	runOpts := distillRunOptions{}
	var since string
	var fromS, toS string
	for i := 0; i < len(args); i++ {
		a := args[i]
		need := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("distill: %s requires a value", a)
			}
			i++
			return args[i], nil
		}
		switch a {
		case "--json":
			runOpts.AsJSON = true
		case "--jev":
			runOpts.UseJev = true
		case "--archive-low":
			runOpts.ArchiveLow = true
		case "--apply":
			runOpts.Apply = true
		case "--cwd", "--project":
			v, err := need()
			if err != nil {
				return runOpts, err
			}
			runOpts.Selection.Project = v
		case "--session":
			v, err := need()
			if err != nil {
				return runOpts, err
			}
			runOpts.Selection.SessionID = strings.TrimSpace(v)
			if runOpts.Selection.SessionID == "" {
				return runOpts, fmt.Errorf("distill: --session requires a non-empty id")
			}
		case "--since":
			v, err := need()
			if err != nil {
				return runOpts, err
			}
			since = v
		case "--from":
			v, err := need()
			if err != nil {
				return runOpts, err
			}
			fromS = v
		case "--to":
			v, err := need()
			if err != nil {
				return runOpts, err
			}
			toS = v
		default:
			if strings.HasPrefix(a, "-") {
				return runOpts, fmt.Errorf("distill: unknown flag %s", a)
			}
			return runOpts, fmt.Errorf("distill: unexpected argument %s", a)
		}
	}
	if runOpts.ArchiveLow && !runOpts.UseJev {
		return runOpts, fmt.Errorf("distill: --archive-low requires --jev")
	}
	if runOpts.Apply && !runOpts.ArchiveLow {
		return runOpts, fmt.Errorf("distill: --apply requires --archive-low")
	}
	proj, err := resolveProject(runOpts.Selection.Project)
	if err != nil {
		return runOpts, err
	}
	runOpts.Selection.Project = proj
	if since != "" {
		d, err := parseSince(since)
		if err != nil {
			return runOpts, err
		}
		runOpts.Selection.From = now.Add(-d)
		runOpts.Selection.To = now
	}
	if fromS != "" {
		t, err := parseTimeArg(fromS, false)
		if err != nil {
			return runOpts, fmt.Errorf("distill --from: %w", err)
		}
		runOpts.Selection.From = t
	}
	if toS != "" {
		t, err := parseTimeArg(toS, true)
		if err != nil {
			return runOpts, fmt.Errorf("distill --to: %w", err)
		}
		runOpts.Selection.To = t
	}
	return runOpts, nil
}

func resolveProject(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		p = "."
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty --since")
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid --since %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("invalid --since %q (use 7d, 24h, 60m)", s)
}

func parseTimeArg(s string, asEnd bool) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		t = t.UTC()
		if asEnd {
			// Date-only --to is exclusive next midnight (end of that UTC day).
			return t.Add(24 * time.Hour), nil
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("want RFC3339 or YYYY-MM-DD, got %q", s)
}
