package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/tui"
	"github.com/jerryxff26-alt/session-top/internal/usage"
)

func runDistill(w io.Writer, args []string, a *usage.Analysis, now time.Time) error {
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
		case "-h", "--help":
			return opts, false, fmt.Errorf("usage: session-top distill [--cwd PATH] [--since 7d] [--from DATE] [--to DATE] [--json]")
		default:
			if strings.HasPrefix(a, "-") {
				return opts, false, fmt.Errorf("distill: unknown flag %s", a)
			}
			return opts, false, fmt.Errorf("distill: unexpected argument %s", a)
		}
	}
	if opts.Project == "" {
		wd, err := os.Getwd()
		if err != nil {
			return opts, false, err
		}
		opts.Project = wd
	}
	if since != "" {
		d, err := parseSince(since)
		if err != nil {
			return opts, false, err
		}
		opts.From = now.Add(-d)
		opts.To = now
	}
	if fromS != "" {
		t, err := parseTimeArg(fromS)
		if err != nil {
			return opts, false, fmt.Errorf("distill --from: %w", err)
		}
		opts.From = t
	}
	if toS != "" {
		t, err := parseTimeArg(toS)
		if err != nil {
			return opts, false, fmt.Errorf("distill --to: %w", err)
		}
		opts.To = t
	}
	return opts, asJSON, nil
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

func parseTimeArg(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("want RFC3339 or YYYY-MM-DD, got %q", s)
}
