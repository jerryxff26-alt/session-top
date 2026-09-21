package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jerryxff26-alt/session-top/internal/tui"
	"github.com/jerryxff26-alt/session-top/internal/usage"
)

// Load is the shipped discover → parse → analyze path.
func Load(home string, now time.Time) (*usage.Analysis, error) {
	return usage.Load(home, now)
}

func stripJSONFlag(args []string) (rest []string, asJSON bool) {
	rest = make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" {
			asJSON = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, asJSON
}

// Run dispatches a CLI command against a Codex home directory.
func Run(w io.Writer, args []string, home string, now time.Time) error {
	args, asJSON := stripJSONFlag(args)
	cmd := ""
	rest := []string{}
	if len(args) > 0 {
		cmd = args[0]
		rest = args[1:]
	}
	switch cmd {
	case "help", "-h", "--help":
		_, err := io.WriteString(w, Help())
		return err
	case "version", "--version":
		_, err := fmt.Fprintf(w, "session-top %s\n", Version)
		return err
	case "watch":
		if asJSON {
			return fmt.Errorf("watch does not support --json")
		}
		return RunWatch(home, now)
	}

	a, err := Load(home, now)
	if err != nil {
		return err
	}
	if len(a.Sessions) == 0 && cmd != "watch" {
		fmt.Fprintf(w, "No sessions found under %s/sessions (or archived_sessions)\n", home)
		if cmd == "" {
			return nil
		}
	}

	switch cmd {
	case "", "overview":
		if asJSON {
			return tui.WriteOverviewJSON(w, a)
		}
		return tui.WriteOverview(w, a)
	case "why":
		if asJSON {
			return tui.WriteWhyJSON(w, a)
		}
		return tui.WriteWhy(w, a)
	case "sessions":
		if asJSON {
			return tui.WriteSessionsJSON(w, a)
		}
		return tui.WriteSessions(w, a)
	case "session":
		if asJSON {
			return fmt.Errorf("session detail --json is not supported yet; use sessions --json")
		}
		if len(rest) < 1 {
			return fmt.Errorf("usage: session-top session <id>")
		}
		s := usage.FindSession(a, rest[0])
		if s == nil {
			return fmt.Errorf("session not found: %s", rest[0])
		}
		return tui.WriteSessionDetail(w, s)
	case "distill":
		if asJSON {
			rest = append(rest, "--json")
		}
		return runDistill(w, rest, a, now)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, Help())
	}
}

// RunWatch starts the live Bubble Tea view.
func RunWatch(home string, _ time.Time) error {
	load := func() (*usage.Analysis, error) {
		return Load(home, time.Now())
	}
	m := tui.NewWatch(load)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Help is the CLI usage text.
func Help() string {
	var b strings.Builder
	fmt.Fprintf(&b, "session-top %s — htop for your agent sessions\n\n", Version)
	b.WriteString("Usage:\n")
	b.WriteString("  session-top [--json]              Usage overview\n")
	b.WriteString("  session-top why [--json]          Why did quota drop?\n")
	b.WriteString("  session-top sessions [--json]     Rank sessions\n")
	b.WriteString("  session-top session <id>          Session timeline\n")
	b.WriteString("  session-top watch                 Live view\n")
	b.WriteString("  session-top distill               Bounded project digest (local by default; optional --jev)\n")
	b.WriteString("  session-top version               Print version\n")
	return b.String()
}
