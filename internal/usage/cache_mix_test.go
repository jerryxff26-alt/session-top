package usage

import (
	"testing"
	"time"
)

func TestObservedCacheMixSurfaced(t *testing.T) {
	a := loadFixtures(t)
	oauth := FindSession(a, "01970000-0000-7000-8000-000000000001")
	if oauth == nil {
		t.Fatal("oauth missing")
	}
	if oauth.CachedTokens <= 0 {
		t.Fatalf("oauth cached tokens %d, want > 0 from fixtures", oauth.CachedTokens)
	}
	if oauth.Autopsy.CachedShare <= 0 {
		t.Fatalf("oauth autopsy cached share %.1f", oauth.Autopsy.CachedShare)
	}
	if a.Why.Observed.CachedTokens <= 0 {
		t.Fatalf("why observed cached tokens %d", a.Why.Observed.CachedTokens)
	}
	if a.Official.PlanType == "" {
		t.Fatal("expected plan_type from fixture rate_limits")
	}
	if a.Official.Credits == nil {
		t.Fatal("expected credits blob from fixture rate_limits")
	}
	_ = time.Time{}
}
