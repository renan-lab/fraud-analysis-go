package simulator

import (
	"reflect"
	"testing"
	"time"
)

func TestNewPopulationRejectsInvalidSize(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, -1} {
		if _, err := NewPopulation(newTestRand(1), size); err == nil {
			t.Errorf("NewPopulation(%d) error = nil, want error", size)
		}
	}
}

func TestNewPopulationIsDeterministicBySeed(t *testing.T) {
	t.Parallel()
	a := newTestPopulation(t, 7, 50)
	b := newTestPopulation(t, 7, 50)
	if !reflect.DeepEqual(a, b) {
		t.Error("NewPopulation with the same seed produced different populations")
	}
	if c := newTestPopulation(t, 8, 50); reflect.DeepEqual(a, c) {
		t.Error("NewPopulation with different seeds produced the same population")
	}
}

func TestCustomersHaveValidProfiles(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 9, 200)
	if pop.Size() != 200 {
		t.Fatalf("Size() = %d, want 200", pop.Size())
	}
	for _, c := range pop.customers {
		switch {
		case !idPattern.MatchString(c.accountID) || !idPattern.MatchString(c.cardID):
			t.Fatalf("customer ids %q %q do not match %s", c.accountID, c.cardID, idPattern)
		case c.home.homeWeight == 0:
			t.Fatalf("customer lives in %s, a fraud-origin-only country", c.home.code)
		case c.activeFrom > c.activeTo || c.activeTo > 23:
			t.Fatalf("active hours %d-%d, want ordered within 0-23", c.activeFrom, c.activeTo)
		case len(c.devices) == 0 || len(c.merchants) == 0 || c.spendMedian <= 0:
			t.Fatalf("customer %+v missing devices, merchants or spend", c)
		}
	}
}

func TestCustomerIsActiveAtUsesLocalHour(t *testing.T) {
	t.Parallel()
	c := customer{home: country{utcOffset: -3}, activeFrom: 8, activeTo: 18}
	tests := []struct {
		utcHour int
		want    bool
	}{
		{utcHour: 11, want: true},  // 08 local
		{utcHour: 21, want: true},  // 18 local
		{utcHour: 22, want: false}, // 19 local
		{utcHour: 2, want: false},  // 23 local, previous day
	}
	for _, tt := range tests {
		at := time.Date(2026, 11, 2, tt.utcHour, 0, 0, 0, time.UTC)
		if got := c.isActiveAt(at); got != tt.want {
			t.Errorf("isActiveAt(%02d:00 UTC) = %v, want %v", tt.utcHour, got, tt.want)
		}
	}
}

func TestSampleAmountIsPositive(t *testing.T) {
	t.Parallel()
	r := newTestRand(1)
	for range 1000 {
		if got := sampleAmount(r, 0.1, 2); got < 1 {
			t.Fatalf("sampleAmount(0.1, 2) = %d, want >= 1", got)
		}
	}
}

func TestMerchantWithMCCReturnsRequestedCategory(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 2, 1)
	m := pop.merchantWithMCC(newTestRand(3), "NG", "6051")
	if m.Country != "NG" || m.MCC != "6051" {
		t.Errorf("merchantWithMCC(NG, 6051) = %+v, want NG/6051", m)
	}
}
