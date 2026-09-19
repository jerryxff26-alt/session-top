package skill

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestDontLetYourTokenDieSkill(t *testing.T) {
	root := repoRoot(t)
	paths := []string{
		filepath.Join(root, "skills", "dont-let-your-token-die", "SKILL.md"),
	}
	required := []string{
		"name: dont-let-your-token-die",
		"session-top",
		"dependency",
		"cwd",
		"time range",
		"raw rollout",
		"auto-install",
		"distill",
		"token",
		"--jev",
		"JEV_API_KEY",
		"context_coverage",
		"assistant",
		"--archive-low",
		"--apply",
		"codex unarchive",
		"not deletion",
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		body := string(b)
		if !strings.HasPrefix(strings.TrimSpace(body), "---") {
			t.Errorf("%s missing YAML front matter", p)
		}
		lower := strings.ToLower(body)
		if !strings.Contains(lower, "distill") || !strings.Contains(lower, "token") {
			t.Errorf("%s description/body should mention distillation / token waste", p)
		}
		for _, want := range required {
			if !strings.Contains(body, want) && !strings.Contains(lower, strings.ToLower(want)) {
				t.Errorf("%s missing %q", p, want)
			}
		}
		if !strings.Contains(lower, "do not auto-install") && !strings.Contains(body, "Do **not auto-install**") && !strings.Contains(lower, "not auto-install") {
			t.Errorf("%s must forbid auto-install", p)
		}
		if strings.Contains(lower, "cat ~/.codex/sessions") {
			t.Errorf("%s must not instruct cat of rollouts", p)
		}
	}
}
