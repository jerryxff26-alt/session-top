package app

import (
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/session-top/session-top/internal/tui"
	"github.com/session-top/session-top/internal/usage"
)

// Load is the shipped discover → parse → analyze path.
func Load(home string, now time.Time) (*usage.Analysis, error) {
	return usage.Load(home, now)
}

// Run dispatches a CLI command against a Codex home directory.
func Run(w io.Writer, args []string, home string, now time.Time) error {
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
	case "watch":
		return RunWatch(home, now)
	}

	a, err := Load(home, now)
	if err != nil {
		return err
	}
	if len(a.Sessions) == 0 && cmd != "watch" {
		fmt.Fprintf(w, "No sessions found under %s/sessions\n", home)
		if cmd == "" {
			return nil
		}
	}

	switch cmd {
	case "", "overview":
		return tui.WriteOverview(w, a)
	case "why":
		return tui.WriteWhy(w, a)
	case "sessions":
		return tui.WriteSessions(w, a)
	case "session":
		if len(rest) < 1 {
			return fmt.Errorf("usage: session-top session <id>")
		}
		s := usage.FindSession(a, rest[0])
		if s == nil {
			return fmt.Errorf("session not found: %s", rest[0])
		}
		return tui.WriteSessionDetail(w, s)
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
	b.WriteString("session-top — htop for your agent sessions\n\n")
	b.WriteString("Usage:\n")
	b.WriteString("  session-top              Usage overview\n")
	b.WriteString("  session-top why          Why did quota drop?\n")
	b.WriteString("  session-top sessions     Rank sessions\n")
	b.WriteString("  session-top session <id> Session timeline\n")
	b.WriteString("  session-top watch        Live view\n")
	return b.String()
}
