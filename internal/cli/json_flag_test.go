package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fixtureHomeJSON(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "codex-home"))
}

func TestOverviewWhySessionsJSON(t *testing.T) {
	now := time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC)
	home := fixtureHomeJSON(t)

	for _, args := range [][]string{
		{"--json"},
		{"overview", "--json"},
		{"why", "--json"},
		{"sessions", "--json"},
	} {
		var buf bytes.Buffer
		if err := Run(&buf, args, home, now); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, buf.String())
		}
		out := buf.String()
		if !json.Valid([]byte(out)) {
			t.Fatalf("%v: invalid JSON:\n%s", args, out)
		}
		switch args[0] {
		case "--json", "overview":
			if !strings.Contains(out, `"official"`) || !strings.Contains(out, `"today"`) {
				t.Fatalf("overview json missing keys:\n%s", out)
			}
			if !strings.Contains(out, `"plan_type"`) {
				t.Fatalf("overview json missing plan_type from fixtures:\n%s", out)
			}
		case "why":
			if !strings.Contains(out, `"observed"`) || !strings.Contains(out, `"cached_tokens"`) {
				t.Fatalf("why json missing observed/cached:\n%s", out)
			}
		case "sessions":
			if !strings.Contains(out, `"sessions"`) || !strings.Contains(out, `"cached_tokens"`) {
				t.Fatalf("sessions json missing keys:\n%s", out)
			}
		}
	}
}
