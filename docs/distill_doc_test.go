package docs

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed distill.md
var distillDesign string

func TestDistillDesignContract(t *testing.T) {
	if distillDesign == "" {
		t.Fatal("docs/distill.md is empty")
	}
	required := []string{
		"## Value goals",
		"CLI vs skill split",
		"cwd",
		"project",
		"time range",
		"Bounded extracts",
		"raw rollout",
		"opt-in",
		"human review",
		"autopsy/why stay local",
		"--since",
		"--cwd",
		"model-written",
		"session ids",
	}
	lower := strings.ToLower(distillDesign)
	for _, want := range required {
		if !strings.Contains(distillDesign, want) && !strings.Contains(lower, strings.ToLower(want)) {
			t.Errorf("docs/distill.md missing required phrase %q", want)
		}
	}
	forbidden := []string{
		"default-on model",
		"auto-enable",
		"auto-enabling a skill without",
	}
	// Contract: the design must not *propose* default-on model calls or
	// auto-enabling without review. Mentions in a prohibition are required.
	if !strings.Contains(lower, "no default") && !strings.Contains(lower, "never auto") && !strings.Contains(distillDesign, "never auto-installs") {
		t.Error("docs/distill.md must forbid default-on model calls / auto-install")
	}
	if strings.Contains(lower, "by default call a model") {
		t.Error("docs/distill.md must not propose calling a model by default")
	}
	_ = forbidden
	if !strings.Contains(distillDesign, "100% local") {
		t.Error("docs/distill.md must keep the 100% local core")
	}
	if !strings.Contains(distillDesign, "Human review") && !strings.Contains(lower, "human review") {
		t.Error("docs/distill.md must require human review")
	}
}
