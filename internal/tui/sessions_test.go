package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/jerryxff26-alt/session-top/internal/usage"
)

func TestCleanSessionTitle(t *testing.T) {
	got := cleanSessionTitle("[ai-bak](ai-bak/) 恢复相关内容")
	if got != "ai-bak 恢复相关内容" {
		t.Fatalf("markdown: %q", got)
	}
	got = cleanSessionTitle("目标：只读评估 /Users/anyi.su/git-repo/quota extra")
	if strings.Contains(got, "/Users/") {
		t.Fatalf("path remains: %q", got)
	}
	if !strings.Contains(got, "quota") {
		t.Fatalf("basename lost: %q", got)
	}
	got = cleanSessionTitle("/goal 输出最终版的方案报告")
	if got != "输出最终版的方案报告" {
		t.Fatalf("/goal: %q", got)
	}
}

func TestSessionsTableAligned(t *testing.T) {
	d := 11.0
	a := &usage.Analysis{
		Sessions: []usage.SessionSummary{
			{ID: "01970000-0000-7000-8000-000000000001", Title: "Fix OAuth callback", ObservedTokens: 921000, Turns: 3, QuotaDelta: &d},
			{ID: "01970000-0000-7000-8000-000000000002", Title: "帮我整理一个问题列表，用于跟客户沟通，基于之前的需求，现在发现还有很多", ObservedTokens: 71000000, Turns: 5},
			{ID: "01970000-0000-7000-8000-000000000003", Title: "[ai-bak](ai-bak/) 恢复相关内容", ObservedTokens: 1690000, Turns: 1},
			{ID: "01970000-0000-7000-8000-000000000004", Title: "目标：只读评估 /Users/anyi.su/git-repo/quota", ObservedTokens: 0, Turns: 1, Ambiguous: true},
		},
	}
	out := Sessions(a)
	if strings.Contains(out, "](") {
		t.Fatalf("markdown survived:\n%s", out)
	}
	if strings.Contains(out, "/Users/") {
		t.Fatalf("abs path survived:\n%s", out)
	}
	if !strings.Contains(out, "000000000001") {
		t.Fatalf("short id missing:\n%s", out)
	}
	if !strings.Contains(out, "Attribution: ambiguous") {
		t.Fatalf("ambiguous footnote missing:\n%s", out)
	}
	var widths []int
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "~") {
			continue
		}
		if strings.Contains(line, "──") {
			continue
		}
		widths = append(widths, lipgloss.Width(line))
	}
	if len(widths) < 3 {
		t.Fatalf("too few table rows:\n%s", out)
	}
	for i := 1; i < len(widths); i++ {
		if widths[i] != widths[0] {
			t.Fatalf("row %d width %d != header %d\n%s", i, widths[i], widths[0], out)
		}
	}
}

func TestTruncateWidthCJK(t *testing.T) {
	s := truncateWidth("帮我整理一个问题列表继续往后写", 8)
	if lipgloss.Width(s) > 8 {
		t.Fatalf("width %d > 8: %q", lipgloss.Width(s), s)
	}
	if strings.ContainsRune(s, '\uFFFD') {
		t.Fatalf("split rune: %q", s)
	}
}
