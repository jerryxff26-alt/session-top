package codex

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HomeDir returns $CODEX_HOME, or ~/.codex if unset.
func HomeDir() (string, error) {
	if h := strings.TrimSpace(os.Getenv("CODEX_HOME")); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

// IsArchivedPath reports whether path sits under CODEX_HOME/archived_sessions.
func IsArchivedPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, p := range parts {
		if p == "archived_sessions" {
			return true
		}
	}
	return false
}

func collectRollouts(root string) ([]string, error) {
	st, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "rollout-") && strings.HasSuffix(name, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// Discover lists rollout JSONL files under sessions/ and archived_sessions/.
// Active sessions are listed first; if the same basename appears in both trees,
// the active copy wins.
func Discover(home string) ([]string, error) {
	active, err := collectRollouts(filepath.Join(home, "sessions"))
	if err != nil {
		return nil, err
	}
	archived, err := collectRollouts(filepath.Join(home, "archived_sessions"))
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(active)+len(archived))
	for _, f := range active {
		base := filepath.Base(f)
		seen[base] = struct{}{}
		out = append(out, f)
	}
	var archivedOnly []string
	for _, f := range archived {
		base := filepath.Base(f)
		if _, ok := seen[base]; ok {
			continue
		}
		seen[base] = struct{}{}
		archivedOnly = append(archivedOnly, f)
	}
	sort.Strings(archivedOnly)
	// Keep active order stable (sorted), then archived.
	sort.Strings(out)
	out = append(out, archivedOnly...)
	return out, nil
}
