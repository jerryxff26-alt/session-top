package usage

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/session-top/session-top/internal/codex"
)

func fixtureHome(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "codex-home"))
}

func loadFixtures(t *testing.T) *Analysis {
	t.Helper()
	now := time.Date(2026, 9, 16, 14, 40, 0, 0, time.UTC)
	a, err := Load(fixtureHome(t), now)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func almost(a, b float64) bool { return math.Abs(a-b) < 0.05 }

func TestOfficialInferredObserved(t *testing.T) {
	a := loadFixtures(t)
	if !a.Official.HasQuota || a.Official.FiveHour == nil || a.Official.Weekly == nil {
		t.Fatalf("missing official quota: %+v", a.Official)
	}
	if !almost(a.Official.FiveHour.UsedPercent, 38.6) {
		t.Fatalf("5h used %v want 38.6", a.Official.FiveHour.UsedPercent)
	}
	if !almost(a.Official.FiveHour.LeftPercent, 61.4) {
		t.Fatalf("5h left %v want 61.4", a.Official.FiveHour.LeftPercent)
	}
	if !almost(a.Official.Weekly.UsedPercent, 16.0) {
		t.Fatalf("weekly used %v want 16 (duration class, not primary slot)", a.Official.Weekly.UsedPercent)
	}
	if !almost(a.Official.Weekly.LeftPercent, 84.0) {
		t.Fatalf("weekly left %v want 84", a.Official.Weekly.LeftPercent)
	}

	oauth := FindSession(a, "01970000-0000-7000-8000-000000000001")
	if oauth == nil {
		t.Fatal("oauth missing")
	}
	if oauth.ObservedTokens != 920500 {
		t.Fatalf("oauth tokens %d want 920500", oauth.ObservedTokens)
	}
	if oauth.QuotaDelta == nil || !almost(*oauth.QuotaDelta, 7.3) {
		t.Fatalf("oauth Δ %v want 7.3", oauth.QuotaDelta)
	}
	if oauth.Turns != 3 {
		t.Fatalf("oauth turns %d", oauth.Turns)
	}
	if oauth.Compactions != 2 {
		t.Fatalf("oauth compactions %d want 2", oauth.Compactions)
	}
	if oauth.ContinuationFollowUps < 1 {
		t.Fatalf("oauth continuation follow-ups %d", oauth.ContinuationFollowUps)
	}
	if oauth.Autopsy.CachedShare < 30 {
		t.Fatalf("oauth cached share %.1f, want high cache", oauth.Autopsy.CachedShare)
	}
	if oauth.Autopsy.InputPct < 90 {
		t.Fatalf("oauth input pct %.1f", oauth.Autopsy.InputPct)
	}
	if len(oauth.Autopsy.Patterns) == 0 {
		t.Fatal("oauth autopsy has no patterns")
	}
	if len(oauth.SiblingIDs) == 0 {
		t.Fatal("oauth should cluster with forked child")
	}
	if oauth.ExpensiveTurn == nil {
		t.Fatal("expensive turn unmarked")
	}
	got := oauth.Timeline[*oauth.ExpensiveTurn]
	if got.Kind == "compaction" || got.Tokens <= 0 {
		t.Fatalf("expensive turn %+v", got)
	}
	// Largest unique 5h drop on this session is the last follow-up (20%→23% is +3, 23%→27.3% is +4.3).
	if got.Tokens != 367000 {
		t.Fatalf("expensive turn tokens %d want 367000 (largest quota drop)", got.Tokens)
	}

	db := FindSession(a, "01970000-0000-7000-8000-000000000002")
	if db == nil || db.QuotaDelta == nil || !almost(*db.QuotaDelta, 5.8) {
		t.Fatalf("db Δ %v", db)
	}
	if db.ObservedTokens != 1400000 {
		t.Fatalf("db tokens %d", db.ObservedTokens)
	}
	mcp := FindSession(a, "01970000-0000-7000-8000-000000000003")
	if mcp == nil || mcp.QuotaDelta == nil || !almost(*mcp.QuotaDelta, 3.1) {
		t.Fatalf("mcp Δ %v", mcp)
	}
	readme := FindSession(a, "01970000-0000-7000-8000-000000000004")
	if readme == nil || readme.QuotaDelta == nil || !almost(*readme.QuotaDelta, 0.4) {
		t.Fatalf("readme Δ %v", readme)
	}

	nullq := FindSession(a, "01970000-0000-7000-8000-000000000005")
	if nullq == nil {
		t.Fatal("null quota session missing")
	}
	if nullq.ObservedTokens != 50000 || nullq.Turns != 2 {
		t.Fatalf("null quota observed tokens=%d turns=%d", nullq.ObservedTokens, nullq.Turns)
	}
	if nullq.QuotaDelta != nil {
		t.Fatalf("null quota session should not have unique Δ, got %v", *nullq.QuotaDelta)
	}

	alpha := FindSession(a, "01970000-0000-7000-8000-000000000006")
	beta := FindSession(a, "01970000-0000-7000-8000-000000000007")
	if alpha == nil || beta == nil {
		t.Fatal("overlap sessions missing")
	}
	if alpha.QuotaDelta != nil || beta.QuotaDelta != nil {
		t.Fatalf("overlap sessions must not receive unique Δ: alpha=%v beta=%v", alpha.QuotaDelta, beta.QuotaDelta)
	}
	if !alpha.Ambiguous || !beta.Ambiguous {
		t.Fatal("overlap sessions should be marked ambiguous")
	}
	if len(a.Ambiguous) != 1 {
		t.Fatalf("ambiguous intervals %d want 1", len(a.Ambiguous))
	}
	amb := a.Ambiguous[0]
	if !almost(amb.Delta, 2.0) {
		t.Fatalf("ambiguous Δ %v want 2.0", amb.Delta)
	}
	if len(amb.SessionIDs) != 2 {
		t.Fatalf("ambiguous sessions %v", amb.SessionIDs)
	}

	if a.Today.Sessions != 7 {
		t.Fatalf("today sessions %d", a.Today.Sessions)
	}
	if a.Today.QuotaUsed == nil || !almost(*a.Today.QuotaUsed, 18.6) {
		t.Fatalf("today 5h used %v want 18.6", a.Today.QuotaUsed)
	}

	if a.Current == nil || a.Current.ID != beta.ID {
		t.Fatalf("current session %+v want beta", a.Current)
	}
	if !a.Current.ContextGrowing {
		t.Fatal("current session should flag context growing")
	}
}

func TestWhyPotentialCausesNotProven(t *testing.T) {
	a := loadFixtures(t)
	if a.Why.QuotaUsed == nil || !almost(*a.Why.QuotaUsed, 18.6) {
		t.Fatalf("why quota used %v", a.Why.QuotaUsed)
	}
	if a.Why.Largest == nil || a.Why.Largest.Title != "Fix OAuth callback" {
		t.Fatalf("largest %+v", a.Why.Largest)
	}
	if a.Why.Observed.Compactions < 2 {
		t.Fatalf("why compactions %d", a.Why.Observed.Compactions)
	}
	if a.Why.Observed.Turns < 10 {
		t.Fatalf("why turns %d", a.Why.Observed.Turns)
	}
	if len(a.Why.Causes) == 0 {
		t.Fatal("expected potential causes")
	}
}

func TestSwappedPrimaryIsWeekly(t *testing.T) {
	now := time.Date(2026, 9, 16, 14, 20, 30, 0, time.UTC)
	home := fixtureHome(t)
	files, err := codex.Discover(home)
	if err != nil {
		t.Fatal(err)
	}
	var db *codex.Rollout
	for _, f := range files {
		r, err := codex.ParseFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if r.Session.ID == "01970000-0000-7000-8000-000000000002" {
			db = r
		}
	}
	if db == nil {
		t.Fatal("db missing")
	}
	a := Analyze([]*codex.Rollout{db}, now)
	if a.Official.FiveHour == nil || !almost(a.Official.FiveHour.UsedPercent, 33.1) {
		t.Fatalf("5h from swapped secondary: %+v", a.Official.FiveHour)
	}
	if a.Official.Weekly == nil || !almost(a.Official.Weekly.UsedPercent, 14.0) {
		t.Fatalf("weekly from swapped primary: %+v", a.Official.Weekly)
	}
}
