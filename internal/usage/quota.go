package usage

import (
	"sort"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/codex"
)

type snapshotDelta struct {
	Start      time.Time
	End        time.Time
	Delta      float64
	UsedBefore float64
	UsedAfter  float64
	Active     []string
}

func collectSnapshots(rollouts []*codex.Rollout) []codex.QuotaSnapshot {
	var all []codex.QuotaSnapshot
	for _, r := range rollouts {
		all = append(all, r.Quotas...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Time.Equal(all[j].Time) {
			return all[i].SessionID < all[j].SessionID
		}
		return all[i].Time.Before(all[j].Time)
	})
	return all
}

func latestOfficial(snaps []codex.QuotaSnapshot) OfficialQuota {
	var out OfficialQuota
	for i := len(snaps) - 1; i >= 0; i-- {
		s := snaps[i]
		if out.FiveHour == nil && s.FiveHour != nil {
			out.FiveHour = cloneWindow(s.FiveHour)
		}
		if out.Weekly == nil && s.Weekly != nil {
			out.Weekly = cloneWindow(s.Weekly)
		}
		if out.PlanType == "" && s.PlanType != "" {
			out.PlanType = s.PlanType
		}
		if out.Credits == nil && s.Credits != nil {
			cp := *s.Credits
			out.Credits = &cp
		}
		if out.FiveHour != nil && out.Weekly != nil && out.PlanType != "" && out.Credits != nil {
			break
		}
	}
	out.HasQuota = out.FiveHour != nil || out.Weekly != nil
	return out
}

func fiveHourDeltas(snaps []codex.QuotaSnapshot) []snapshotDelta {
	var with5 []codex.QuotaSnapshot
	for _, s := range snaps {
		if s.FiveHour != nil {
			with5 = append(with5, s)
		}
	}
	var out []snapshotDelta
	for i := 1; i < len(with5); i++ {
		a, b := with5[i-1], with5[i]
		before, after := a.FiveHour.UsedPercent, b.FiveHour.UsedPercent
		if after+0.0001 < before {
			continue // reset
		}
		delta := after - before
		if delta <= 0 {
			continue
		}
		out = append(out, snapshotDelta{
			Start:      a.Time,
			End:        b.Time,
			Delta:      delta,
			UsedBefore: before,
			UsedAfter:  after,
		})
	}
	return out
}

func sessionIDsActive(rollouts []*codex.Rollout, start, end time.Time) []string {
	seen := map[string]struct{}{}
	in := func(t time.Time) bool {
		return t.After(start) && !t.After(end)
	}
	for _, r := range rollouts {
		id := r.Session.ID
		hit := false
		for _, e := range r.Usage {
			if in(e.Time) {
				hit = true
				break
			}
		}
		if !hit {
			for _, e := range r.Turns {
				if in(e.Time) {
					hit = true
					break
				}
			}
		}
		if !hit {
			for _, e := range r.Tools {
				if in(e.Time) {
					hit = true
					break
				}
			}
		}
		if !hit {
			for _, e := range r.Compactions {
				if in(e.Time) {
					hit = true
					break
				}
			}
		}
		if !hit {
			for _, e := range r.Prompts {
				if in(e.Time) {
					hit = true
					break
				}
			}
		}
		if !hit {
			for _, e := range r.Quotas {
				if in(e.Time) {
					hit = true
					break
				}
			}
		}
		if hit {
			seen[id] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func attribute(rollouts []*codex.Rollout, deltas []snapshotDelta, titles map[string]string) (map[string]float64, []AmbiguousInterval) {
	unique := map[string]float64{}
	var amb []AmbiguousInterval
	for i := range deltas {
		d := &deltas[i]
		d.Active = sessionIDsActive(rollouts, d.Start, d.End)
		switch len(d.Active) {
		case 1:
			unique[d.Active[0]] += d.Delta
		case 0:
			// unattributed; still a real quota move, just no session
		default:
			titlesFor := make([]string, 0, len(d.Active))
			for _, id := range d.Active {
				if t := titles[id]; t != "" {
					titlesFor = append(titlesFor, t)
				} else {
					titlesFor = append(titlesFor, id)
				}
			}
			amb = append(amb, AmbiguousInterval{
				Start:      d.Start,
				End:        d.End,
				Delta:      d.Delta,
				UsedBefore: d.UsedBefore,
				UsedAfter:  d.UsedAfter,
				SessionIDs: append([]string(nil), d.Active...),
				Titles:     titlesFor,
			})
		}
	}
	return unique, amb
}

func inferredToday(deltas []snapshotDelta, now time.Time) *float64 {
	if len(deltas) == 0 {
		return nil
	}
	y, m, d := now.In(time.Local).Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	end := start.Add(24 * time.Hour)
	var sum float64
	any := false
	for _, dlt := range deltas {
		if dlt.End.Before(start) || !dlt.Start.Before(end) {
			continue
		}
		sum += dlt.Delta
		any = true
	}
	if !any {
		return nil
	}
	return &sum
}
