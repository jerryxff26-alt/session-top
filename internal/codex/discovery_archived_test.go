package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverIncludesArchivedPreferringActive(t *testing.T) {
	home := t.TempDir()
	activeDir := filepath.Join(home, "sessions", "2026", "09", "16")
	archDir := filepath.Join(home, "archived_sessions")
	if err := os.MkdirAll(activeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}

	activeName := "rollout-2026-09-16T10-00-00-aaaaaaaa-bbbb-cccc-dddd-000000000001.jsonl"
	onlyArchName := "rollout-2026-09-16T11-00-00-aaaaaaaa-bbbb-cccc-dddd-000000000002.jsonl"
	dupName := "rollout-2026-09-16T12-00-00-aaaaaaaa-bbbb-cccc-dddd-000000000003.jsonl"

	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(activeDir, activeName), "{}\n")
	write(filepath.Join(archDir, onlyArchName), "{}\n")
	write(filepath.Join(activeDir, dupName), "{}\n")
	write(filepath.Join(archDir, dupName), "{}\n") // duplicate basename — active wins

	files, err := Discover(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("got %d files %#v, want 3", len(files), files)
	}

	var sawActive, sawArchOnly, sawDupActive bool
	for _, f := range files {
		base := filepath.Base(f)
		switch base {
		case activeName:
			sawActive = true
			if IsArchivedPath(f) {
				t.Fatalf("active file marked archived: %s", f)
			}
		case onlyArchName:
			sawArchOnly = true
			if !IsArchivedPath(f) {
				t.Fatalf("archived-only file not under archived_sessions: %s", f)
			}
		case dupName:
			sawDupActive = true
			if IsArchivedPath(f) {
				t.Fatalf("duplicate preferred archived copy: %s", f)
			}
			if !strings.Contains(f, string(filepath.Separator)+"sessions"+string(filepath.Separator)) {
				t.Fatalf("duplicate not from sessions/: %s", f)
			}
		}
	}
	if !sawActive || !sawArchOnly || !sawDupActive {
		t.Fatalf("missing expected files: active=%v archOnly=%v dupActive=%v files=%v", sawActive, sawArchOnly, sawDupActive, files)
	}

	// Ordering: all active paths before archived-only paths.
	var sawArchived bool
	for _, f := range files {
		if IsArchivedPath(f) {
			sawArchived = true
			continue
		}
		if sawArchived {
			t.Fatalf("active path after archived path: %s in %#v", f, files)
		}
	}
}
