package usage

import (
	"time"

	"github.com/session-top/session-top/internal/codex"
)

// Analyze merges parsed rollouts into OFFICIAL / OBSERVED / INFERRED views.
func Analyze(rollouts []*codex.Rollout, now time.Time) *Analysis {
	if now.IsZero() {
		now = time.Now()
	}
	snaps := collectSnapshots(rollouts)
	official := latestOfficial(snaps)
	deltas := fiveHourDeltas(snaps)

	titles := map[string]string{}
	for _, r := range rollouts {
		titles[r.Session.ID] = titleFor(r)
	}
	unique, amb := attribute(rollouts, deltas, titles)
	ambIDs := map[string]struct{}{}
	for _, a := range amb {
		for _, id := range a.SessionIDs {
			ambIDs[id] = struct{}{}
		}
	}
	sessions := buildSessions(rollouts, unique, ambIDs, snaps)
	a := &Analysis{
		Official:    official,
		Today:       todayStats(sessions, deltas, now),
		Sessions:    sessions,
		Consumers:   consumers(sessions),
		Ambiguous:   amb,
		Why:         buildWhy(rollouts, sessions, deltas, amb, now),
		Current:     currentSession(sessions),
		GeneratedAt: now,
	}
	return a
}

// FindSession returns the unique session matching id (exact, prefix, or suffix).
func FindSession(a *Analysis, id string) *SessionSummary {
	if a == nil || id == "" {
		return nil
	}
	var matches []SessionSummary
	for _, s := range a.Sessions {
		if s.ID == id {
			cp := s
			return &cp
		}
		if len(id) >= 4 && (hasPrefix(s.ID, id) || hasSuffix(s.ID, id)) {
			matches = append(matches, s)
		}
	}
	if len(matches) == 1 {
		cp := matches[0]
		return &cp
	}
	return nil
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func hasSuffix(s, p string) bool {
	return len(s) >= len(p) && s[len(s)-len(p):] == p
}

// Load discovers and parses rollouts under a Codex home directory, then analyzes them.
func Load(home string, now time.Time) (*Analysis, error) {
	files, err := codex.Discover(home)
	if err != nil {
		return nil, err
	}
	var rollouts []*codex.Rollout
	for _, f := range files {
		r, err := codex.ParseFile(f)
		if err != nil {
			continue
		}
		if r.Session.ID == "" {
			continue
		}
		rollouts = append(rollouts, r)
	}
	return Analyze(rollouts, now), nil
}
