package simulator

import (
	"math/rand/v2"
	"time"
)

const impossibleTravelMinKM = 5000

func fraudScenarios(pop *Population) []Scenario {
	return []Scenario{
		fraudSpec("fraud/velocity", "8-15 purchases on the same card within ~3 minutes; expects 202", pop.velocity),
		fraudSpec("fraud/card_testing", "5-10 tiny online charges abroad from a new device, then a large one; expects 202", pop.cardTesting),
		fraudSpec("fraud/impossible_travel", "legit purchase at home, then a card-present purchase >5000 km away minutes later; expects 202", pop.impossibleTravel),
		fraudSpec("fraud/amount_outlier", "electronics purchase 15-40x the customer's median spend; expects 202", pop.amountOutlier),
		fraudSpec("fraud/foreign_new_device", "online betting/crypto purchase from a fraud-origin country, new device and foreign IP; expects 202", pop.foreignNewDevice),
	}
}

// fraudSpec turns a transaction sequence builder into a Scenario labeled fraud.
func fraudSpec(name, description string, build func(r *rand.Rand, now time.Time) []labeledTx) Scenario {
	return scenarioSpec{
		name:        name,
		mode:        ModeFraud,
		description: description,
		next: func(r *rand.Rand, now time.Time) ([]Emission, error) {
			return encodeSequence(ModeFraud, name, build(r, now))
		},
	}
}

type labeledTx struct {
	tx       Transaction
	label    Label
	parallel bool
}

func encodeSequence(mode Mode, scenario string, seq []labeledTx) ([]Emission, error) {
	out := make([]Emission, 0, len(seq))
	for _, lt := range seq {
		e, err := acceptedEmission(mode, scenario, lt.label, lt.tx)
		if err != nil {
			return nil, err
		}
		e.Parallel = lt.parallel
		out = append(out, e)
	}
	return out, nil
}

// backwardTimes returns n increasing timestamps ending at now, spaced by gap(r).
func backwardTimes(r *rand.Rand, now time.Time, n int, gap func(*rand.Rand) time.Duration) []time.Time {
	times := make([]time.Time, n)
	at := now
	for i := n - 1; i >= 0; i-- {
		times[i] = at
		at = at.Add(-gap(r))
	}
	return times
}

func secondsBetween(lo, hi int) func(*rand.Rand) time.Duration {
	return func(r *rand.Rand) time.Duration {
		return time.Duration(lo+r.IntN(hi-lo+1)) * time.Second
	}
}

func (p *Population) velocity(r *rand.Rand, now time.Time) []labeledTx {
	c := p.randomCustomer(r)
	times := backwardTimes(r, now, 8+r.IntN(8), secondsBetween(5, 12))
	seq := make([]labeledTx, len(times))
	for i, at := range times {
		seq[i] = labeledTx{tx: p.legitPurchase(r, c, at), label: LabelFraud}
	}
	return seq
}

func (p *Population) cardTesting(r *rand.Rand, now time.Time) []labeledTx {
	c := p.randomCustomer(r)
	origin := fraudOrigins[r.IntN(len(fraudOrigins))]
	c.devices = newDevices(r, 1)
	c.deviceCountry, _ = countryByCode(origin)
	times := backwardTimes(r, now, 6+r.IntN(6), secondsBetween(3, 20))
	seq := make([]labeledTx, len(times))
	for i, at := range times[:len(times)-1] {
		m := p.merchantWithMCC(r, origin, mccOnlineRetail)
		seq[i] = labeledTx{tx: newTransaction(r, c, m, int64(100+r.IntN(401)), ChannelEcommerce, at), label: LabelFraud}
	}
	big := p.merchantWithMCC(r, origin, mccElectronics)
	amount := int64(c.spendMedian * float64(20+r.IntN(11)))
	seq[len(seq)-1] = labeledTx{tx: newTransaction(r, c, big, amount, ChannelEcommerce, now), label: LabelFraud}
	return seq
}

func (p *Population) impossibleTravel(r *rand.Rand, now time.Time) []labeledTx {
	c := p.randomCustomer(r)
	// The first purchase is in person so both legs carry a terminal location.
	local := p.merchantWithMCC(r, c.home.code, mccGrocery)
	first := newTransaction(r, c, local, sampleAmount(r, c.spendMedian, c.spendSigma), ChannelCardPresent, now.Add(-time.Duration(10+r.IntN(31))*time.Minute))
	far := countriesFartherThan(c.home, impossibleTravelMinKM)
	dest := far[r.IntN(len(far))]
	m := p.merchantWithMCC(r, dest.code, mccGrocery)
	second := newTransaction(r, c, m, sampleAmount(r, c.spendMedian, c.spendSigma), ChannelCardPresent, now)
	second.IP = newIP(r, dest)
	return []labeledTx{{tx: first, label: LabelLegit}, {tx: second, label: LabelFraud}}
}

func (p *Population) amountOutlier(r *rand.Rand, now time.Time) []labeledTx {
	c := p.randomCustomer(r)
	m := p.merchantWithMCC(r, c.home.code, mccElectronics)
	amount := int64(c.spendMedian * float64(15+r.IntN(26)))
	return []labeledTx{{tx: newTransaction(r, c, m, amount, ChannelEcommerce, now), label: LabelFraud}}
}

func (p *Population) foreignNewDevice(r *rand.Rand, now time.Time) []labeledTx {
	c := p.randomCustomer(r)
	originCode := fraudOrigins[r.IntN(len(fraudOrigins))]
	origin, _ := countryByCode(originCode)
	risky := []string{"7995", "6051"}
	m := p.merchantWithMCC(r, originCode, risky[r.IntN(len(risky))])
	c.devices = newDevices(r, 1)
	c.ip = newIP(r, origin)
	c.deviceCountry = origin
	amount := sampleAmount(r, c.spendMedian*3, c.spendSigma)
	return []labeledTx{{tx: newTransaction(r, c, m, amount, ChannelEcommerce, now), label: LabelFraud}}
}
