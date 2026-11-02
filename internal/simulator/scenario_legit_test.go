package simulator

import (
	"slices"
	"testing"
)

func TestLegitPurchaseMatchesCustomerHabits(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 5, 30)
	r := newTestRand(6)
	for range 200 {
		c := pop.randomCustomer(r)
		tx := pop.legitPurchase(r, c, testNow)
		cat, _ := categoryByMCC(tx.Merchant.MCC)
		switch {
		case tx.Merchant.Country != c.home.code:
			t.Fatalf("legit merchant country = %q, want home %q", tx.Merchant.Country, c.home.code)
		case cat.risky:
			t.Fatalf("legit merchant MCC = %q, want a non-risky category", tx.Merchant.MCC)
		case !slices.Contains(c.devices, tx.DeviceID) || tx.IP != c.ip || tx.Currency != c.home.currency:
			t.Fatalf("legit tx %+v does not use customer device/ip/currency %v %q %q", tx, c.devices, c.ip, c.home.currency)
		case tx.Channel != ChannelEcommerce && (tx.Location == nil || !nearCountry(*tx.Location, c.home)):
			t.Fatalf("in-person legit location = %+v, want terminal near home %s", tx.Location, c.home.code)
		case tx.Location != nil && !nearCountry(*tx.Location, c.home):
			t.Fatalf("legit location = %+v, want near home %s", *tx.Location, c.home.code)
		}
	}
}

func TestEcommerceLocationIsSometimesOmitted(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 7, 10)
	r := newTestRand(8)
	c := pop.randomCustomer(r)
	m := pop.merchantWithMCC(r, c.home.code, mccOnlineRetail)
	var sent, omitted int
	for range 500 {
		if newTransaction(r, c, m, 1000, ChannelEcommerce, testNow).Location == nil {
			omitted++
		} else {
			sent++
		}
	}
	if sent == 0 || omitted == 0 {
		t.Fatalf("ecommerce locations: %d sent, %d omitted; want both paths exercised", sent, omitted)
	}
}

func TestNearbyLocationIsReproducibleBySeed(t *testing.T) {
	t.Parallel()
	br, _ := countryByCode("BR")
	a, b := nearbyLocation(newTestRand(3), br), nearbyLocation(newTestRand(3), br)
	if *a != *b || !nearCountry(*a, br) {
		t.Fatalf("nearbyLocation(seed 3) = %+v and %+v, want equal and near BR", *a, *b)
	}
}
