package simulator

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestVelocityKeepsSameCardInShortWindow(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 10, 20)
	r := newTestRand(11)
	for range 50 {
		seq := pop.velocity(r, testNow)
		first, last := seq[0].tx, seq[len(seq)-1].tx
		if len(seq) < 8 || last.OccurredAt.Sub(first.OccurredAt) > 3*time.Minute {
			t.Fatalf("velocity: %d txs over %v, want >= 8 within 3m", len(seq), last.OccurredAt.Sub(first.OccurredAt))
		}
		for _, lt := range seq {
			if lt.tx.CardID != first.CardID || lt.label != LabelFraud {
				t.Fatalf("velocity tx card %q label %q, want card %q label fraud", lt.tx.CardID, lt.label, first.CardID)
			}
		}
	}
}

func TestCardTestingEndsWithLargeChargeFromNewDevice(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 12, 20)
	r := newTestRand(13)
	for range 50 {
		seq := pop.cardTesting(r, testNow)
		big := seq[len(seq)-1].tx
		c := pop.customerByAccount(t, big.AccountID)
		if slices.Contains(c.devices, big.DeviceID) {
			t.Fatalf("card testing device %q is one of the customer's %v, want a new one", big.DeviceID, c.devices)
		}
		origin, _ := countryByCode(big.Merchant.Country)
		for _, lt := range seq {
			if l := lt.tx.Location; l != nil && !nearCountry(*l, origin) {
				t.Fatalf("card testing device location %+v, want near fraud origin %s", *l, origin.code)
			}
		}
		for _, lt := range seq[:len(seq)-1] {
			if lt.tx.Amount > 500 || lt.tx.Amount >= big.Amount || lt.tx.DeviceID != big.DeviceID {
				t.Fatalf("probe amount %d device %q, want <= 500, < final %d and device %q", lt.tx.Amount, lt.tx.DeviceID, big.Amount, big.DeviceID)
			}
		}
	}
}

func TestCardTestingFiresProbesInParallelThenCashout(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 12, 20)
	seq := pop.cardTesting(newTestRand(13), testNow)
	emissions, err := encodeSequence(ModeFraud, "fraud/card_testing", seq)
	if err != nil {
		t.Fatalf("encodeSequence: %v", err)
	}
	for i, e := range emissions {
		if wantParallel := i < len(emissions)-1; e.Parallel != wantParallel {
			t.Fatalf("emission %d of %d Parallel = %v, want %v (probes parallel, cash-out sequential)", i, len(emissions), e.Parallel, wantParallel)
		}
	}
}

func TestClonedCardConcurrentIsSimultaneousAndFar(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 20, 20)
	r := newTestRand(21)
	for range 50 {
		seq := pop.clonedCardConcurrent(r, testNow)
		home, far := seq[0], seq[1]
		gap := far.tx.OccurredAt.Sub(home.tx.OccurredAt)
		a, b := home.tx.Location, far.tx.Location
		switch {
		case home.tx.CardID != far.tx.CardID:
			t.Fatalf("cards %q and %q, want the same card", home.tx.CardID, far.tx.CardID)
		case gap <= 0 || gap > 3*time.Second:
			t.Fatalf("far purchase %v after home, want within (0, 3s]", gap)
		case a == nil || b == nil:
			t.Fatalf("locations = %+v, %+v, want both set", a, b)
		case haversineKM(a.Lat, a.Lon, b.Lat, b.Lon) < impossibleTravelMinKM-100:
			t.Fatalf("locations %.0f km apart, want about >= %d", haversineKM(a.Lat, a.Lon, b.Lat, b.Lon), impossibleTravelMinKM)
		case !home.parallel || !far.parallel:
			t.Fatalf("parallel = %v, %v, want both sent at the same time", home.parallel, far.parallel)
		case home.label != LabelLegit || far.label != LabelFraud:
			t.Fatalf("labels = %q, %q, want legit then fraud", home.label, far.label)
		}
	}
}

func TestImpossibleTravelExceedsAirlinerSpeed(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 14, 20)
	r := newTestRand(15)
	for range 50 {
		seq := pop.impossibleTravel(r, testNow)
		home, _ := countryByCode(seq[0].tx.Merchant.Country)
		dest, _ := countryByCode(seq[1].tx.Merchant.Country)
		hours := seq[1].tx.OccurredAt.Sub(seq[0].tx.OccurredAt).Hours()
		if speed := distanceKM(home, dest) / hours; speed < 1000 {
			t.Fatalf("impossible travel %s->%s at %.0f km/h, want >= 1000", home.code, dest.code, speed)
		}
		a, b := seq[0].tx.Location, seq[1].tx.Location
		if a == nil || b == nil {
			t.Fatalf("impossible travel locations = %+v, %+v, want both set", a, b)
		}
		if km := haversineKM(a.Lat, a.Lon, b.Lat, b.Lon); km < impossibleTravelMinKM-100 {
			t.Fatalf("impossible travel locations %.0f km apart, want about >= %d", km, impossibleTravelMinKM)
		}
		if seq[0].label != LabelLegit || seq[1].label != LabelFraud {
			t.Fatalf("labels = %q, %q, want legit then fraud", seq[0].label, seq[1].label)
		}
	}
}

func TestAmountOutlierIsFarAboveMedian(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 16, 20)
	r := newTestRand(17)
	for range 50 {
		tx := pop.amountOutlier(r, testNow)[0].tx
		c := pop.customerByAccount(t, tx.AccountID)
		if min := int64(c.spendMedian * 15); tx.Amount < min {
			t.Fatalf("outlier amount %d, want >= 15x median (%d)", tx.Amount, min)
		}
	}
}

func TestForeignNewDeviceComesFromFraudOrigin(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 18, 20)
	r := newTestRand(19)
	for range 50 {
		tx := pop.foreignNewDevice(r, testNow)[0].tx
		c := pop.customerByAccount(t, tx.AccountID)
		origin, _ := countryByCode(tx.Merchant.Country)
		switch {
		case !slices.Contains(fraudOrigins, origin.code):
			t.Fatalf("merchant country %q, want one of %v", origin.code, fraudOrigins)
		case slices.Contains(c.devices, tx.DeviceID):
			t.Fatalf("device %q is known to the customer, want a new one", tx.DeviceID)
		case !strings.HasPrefix(tx.IP, origin.ipPrefix):
			t.Fatalf("ip %q, want prefix %q of %s", tx.IP, origin.ipPrefix, origin.code)
		case tx.Location != nil && !nearCountry(*tx.Location, origin):
			t.Fatalf("device location %+v, want near %s", *tx.Location, origin.code)
		}
	}
}

func TestBackwardTimesEndAtNowAndIncrease(t *testing.T) {
	t.Parallel()
	times := backwardTimes(newTestRand(1), testNow, 5, secondsBetween(1, 3))
	if !times[len(times)-1].Equal(testNow) || !slices.IsSortedFunc(times, func(a, b time.Time) int { return a.Compare(b) }) {
		t.Errorf("backwardTimes = %v, want increasing and ending at %v", times, testNow)
	}
}
