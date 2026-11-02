package simulator

import (
	"math/rand/v2"
	"time"
)

const habitualMerchantChance = 0.9

// ecommerceLocationChance is the share of online purchases whose device shares
// its location; the rest omit the optional block (no permission, no GPS).
const ecommerceLocationChance = 0.7

func legitScenarios(pop *Population) []Scenario {
	return []Scenario{
		scenarioSpec{
			name:        "legit/purchase",
			mode:        ModeLegit,
			description: "purchase consistent with the customer's country, spending, hours, devices and usual merchants; expects 202",
			next: func(r *rand.Rand, now time.Time) ([]Emission, error) {
				tx := pop.legitPurchase(r, pop.activeCustomer(r, now), now)
				return single(acceptedEmission(ModeLegit, "legit/purchase", LabelLegit, tx))
			},
		},
	}
}

// legitPurchase builds a purchase that matches the customer's habits.
func (p *Population) legitPurchase(r *rand.Rand, c customer, at time.Time) Transaction {
	m := c.merchants[r.IntN(len(c.merchants))]
	if r.Float64() > habitualMerchantChance {
		m = p.habitualMerchants(r, c.home.code, 1)[0]
	}
	cat, _ := categoryByMCC(m.MCC)
	amount := sampleAmount(r, c.spendMedian*cat.amountFactor, c.spendSigma)
	channel := cat.channels[r.IntN(len(cat.channels))]
	return newTransaction(r, c, m, amount, channel, at)
}

func newTransaction(r *rand.Rand, c customer, m Merchant, amount int64, channel Channel, at time.Time) Transaction {
	return Transaction{
		TransactionID: newUUID(r),
		AccountID:     c.accountID,
		CardID:        c.cardID,
		Amount:        amount,
		Currency:      c.home.currency,
		Merchant:      m,
		Channel:       channel,
		DeviceID:      c.devices[r.IntN(len(c.devices))],
		IP:            c.ip,
		OccurredAt:    at.UTC().Truncate(time.Millisecond),
		Location:      purchaseLocation(r, c, m, channel),
	}
}

// purchaseLocation is the terminal (merchant) location for in-person channels
// and the device location for ecommerce, sometimes omitted.
func purchaseLocation(r *rand.Rand, c customer, m Merchant, channel Channel) *Location {
	if channel == ChannelEcommerce {
		if r.Float64() >= ecommerceLocationChance {
			return nil
		}
		return nearbyLocation(r, c.deviceCountry)
	}
	where, ok := countryByCode(m.Country)
	if !ok {
		return nil
	}
	return nearbyLocation(r, where)
}
