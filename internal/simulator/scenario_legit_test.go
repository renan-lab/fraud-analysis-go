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
		}
	}
}
