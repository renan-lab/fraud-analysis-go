package simulator

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

const merchantsPerCategory = 3

// customer is a cardholder with stable habits, so legit traffic stays coherent over time.
type customer struct {
	accountID   string
	cardID      string
	home        country
	spendMedian float64
	spendSigma  float64
	// activeFrom and activeTo bound the local hours the customer usually shops.
	activeFrom, activeTo int
	devices              []string
	ip                   string
	// deviceCountry is where the customer's devices are; fraud scenarios move
	// it abroad together with a new device, without touching home or currency.
	deviceCountry country
	merchants     []Merchant
}

// Population is the set of simulated customers and the merchants they can buy from.
type Population struct {
	customers []customer
	merchants map[string][]Merchant
}

// NewPopulation builds size customers deterministically from r.
//
//	pop, err := NewPopulation(rand.New(rand.NewPCG(42, 42)), 1000)
func NewPopulation(r *rand.Rand, size int) (*Population, error) {
	if size < 1 {
		return nil, fmt.Errorf("population size must be >= 1, got %d", size)
	}
	pop := &Population{merchants: buildMerchants(r)}
	pop.customers = make([]customer, size)
	for i := range pop.customers {
		pop.customers[i] = pop.newCustomer(r)
	}
	return pop, nil
}

// Size returns how many customers the population has.
func (p *Population) Size() int {
	return len(p.customers)
}

func buildMerchants(r *rand.Rand) map[string][]Merchant {
	byCountry := make(map[string][]Merchant, len(countries))
	for _, c := range countries {
		for _, cat := range categories {
			for range merchantsPerCategory {
				byCountry[c.code] = append(byCountry[c.code], newMerchant(r, c.code, cat))
			}
		}
	}
	return byCountry
}

func newMerchant(r *rand.Rand, countryCode string, cat category) Merchant {
	return Merchant{
		ID:      newID(r, "mer"),
		Name:    fmt.Sprintf("%s %04x", cat.label, r.IntN(0x10000)),
		MCC:     cat.mcc,
		Country: countryCode,
	}
}

func (p *Population) newCustomer(r *rand.Rand) customer {
	home := pickHomeCountry(r)
	from := 6 + r.IntN(6)
	return customer{
		accountID:     newID(r, "acc"),
		cardID:        newID(r, "card"),
		home:          home,
		spendMedian:   home.typicalSpend * math.Exp(0.5*r.NormFloat64()),
		spendSigma:    0.4 + 0.4*r.Float64(),
		activeFrom:    from,
		activeTo:      min(from+10+r.IntN(6), 23),
		devices:       newDevices(r, 1+r.IntN(3)),
		ip:            newIP(r, home),
		deviceCountry: home,
		merchants:     p.habitualMerchants(r, home.code, 5+r.IntN(8)),
	}
}

func pickHomeCountry(r *rand.Rand) country {
	total := 0
	for _, c := range countries {
		total += c.homeWeight
	}
	n := r.IntN(total)
	for _, c := range countries {
		if n < c.homeWeight {
			return c
		}
		n -= c.homeWeight
	}
	return countries[0]
}

func newDevices(r *rand.Rand, n int) []string {
	devices := make([]string, n)
	for i := range devices {
		devices[i] = newID(r, "dev")
	}
	return devices
}

func newIP(r *rand.Rand, c country) string {
	return fmt.Sprintf("%s%d.%d.%d", c.ipPrefix, r.IntN(256), r.IntN(256), 1+r.IntN(254))
}

// habitualMerchants picks non-risky merchants a customer returns to.
func (p *Population) habitualMerchants(r *rand.Rand, countryCode string, n int) []Merchant {
	picked := make([]Merchant, 0, n)
	for len(picked) < n {
		m := p.randomMerchant(r, countryCode)
		if cat, _ := categoryByMCC(m.MCC); !cat.risky {
			picked = append(picked, m)
		}
	}
	return picked
}

func (p *Population) randomMerchant(r *rand.Rand, countryCode string) Merchant {
	pool := p.merchants[countryCode]
	return pool[r.IntN(len(pool))]
}

// merchantWithMCC picks a merchant of the given category in a country.
func (p *Population) merchantWithMCC(r *rand.Rand, countryCode, mcc string) Merchant {
	for {
		if m := p.randomMerchant(r, countryCode); m.MCC == mcc {
			return m
		}
	}
}

func (p *Population) randomCustomer(r *rand.Rand) customer {
	return p.customers[r.IntN(len(p.customers))]
}

// activeCustomer prefers customers whose shopping hours include now.
func (p *Population) activeCustomer(r *rand.Rand, now time.Time) customer {
	c := p.randomCustomer(r)
	for range 3 {
		if c.isActiveAt(now) {
			return c
		}
		c = p.randomCustomer(r)
	}
	return c
}

func (c customer) isActiveAt(t time.Time) bool {
	hour := (t.UTC().Hour() + c.home.utcOffset + 24) % 24
	return hour >= c.activeFrom && hour <= c.activeTo
}

// sampleAmount draws a log-normal amount in minor units, never below 1.
func sampleAmount(r *rand.Rand, median, sigma float64) int64 {
	return max(1, int64(math.Round(median*math.Exp(sigma*r.NormFloat64()))))
}
