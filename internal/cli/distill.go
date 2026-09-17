package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/tui"
	"github.com/jerryxff26-alt/session-top/internal/usage"
)

func runDistill(w io.Writer, args []string, a *usage.Analysis, now time.Time) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			_, err := io.WriteString(w, DistillHelp())
			return err
		}
	}
	opts, asJSON, err := parseDistillArgs(args, now)
	if err != nil {
		return err
	}
	d := usage.Distill(a, opts)
	if asJSON {
		return tui.WriteDistillJSON(w, d)
	}
	return tui.WriteDistill(w, d)
}

// DistillHelp is the distill subcommand usage (exit 0 via --help).
func DistillHelp() string {
	return "usage: session-top distill [--cwd PATH] [--since 7d] [--from DATE] [--to DATE] [--json]\n"
}

func parseDistillArgs(args []string, now time.Time) (usage.DistillOpts, bool, error) {
	opts := usage.DistillOpts{}
	asJSON := false
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
			asJSON = true
		case "--cwd", "--project":
			v, err := need()
			if err != nil {
				return opts, false, err
			}
			opts.Project = v
		case "--since":
			v, err := need()
			if err != nil {
				return opts, false, err
			}
			since = v
		case "--from":
			v, err := need()
			if err != nil {
				return opts, false, err
			}
			fromS = v
		case "--to":
			v, err := need()
			if err != nil {
				return opts, false, err
			}
			toS = v
		default:
			if strings.HasPrefix(a, "-") {
				return opts, false, fmt.Errorf("distill: unknown flag %s", a)
			}
			return opts, false, fmt.Errorf("distill: unexpected argument %s", a)
		}
	}
	proj, err := resolveProject(opts.Project)
	if err != nil {
		return opts, false, err
	}
	opts.Project = proj
	if since != "" {
		d, err := parseSince(since)
		if err != nil {
			return opts, false, err
		}
		opts.From = now.Add(-d)
		opts.To = now
	}
	if fromS != "" {
		t, err := parseTimeArg(fromS, false)
		if err != nil {
			return opts, false, fmt.Errorf("distill --from: %w", err)
		}
		opts.From = t
	}
	if toS != "" {
		t, err := parseTimeArg(toS, true)
		if err != nil {
			return opts, false, fmt.Errorf("distill --to: %w", err)
		}
		opts.To = t
	}
	return opts, asJSON, nil
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
