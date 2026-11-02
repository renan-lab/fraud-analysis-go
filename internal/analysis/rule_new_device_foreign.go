package analysis

import (
	"slices"
	"strings"
)

// DeviceCountryParams configures NewDeviceForeignRule. Each signal adds its
// own score: a device the account never used, a merchant country the account
// never bought from, and an MCC in RiskyMCCs (e.g. 7995 betting, 6051 crypto).
type DeviceCountryParams struct {
	RiskyMCCs       []string
	NewDeviceScore  int
	NewCountryScore int
	RiskyMCCScore   int
}

// NewDeviceForeignRule flags account-takeover patterns. One signal alone is
// common in legit traffic (first purchase, travel), so scores are meant to
// reach review only when signals stack.
type NewDeviceForeignRule struct {
	p DeviceCountryParams
}

// NewNewDeviceForeignRule builds a NewDeviceForeignRule.
//
//	analysis.NewNewDeviceForeignRule(analysis.DeviceCountryParams{RiskyMCCs: []string{"7995"}, NewDeviceScore: 20, NewCountryScore: 25, RiskyMCCScore: 25})
func NewNewDeviceForeignRule(p DeviceCountryParams) NewDeviceForeignRule {
	return NewDeviceForeignRule{p: p}
}

// Name implements Rule.
func (NewDeviceForeignRule) Name() string { return "new_device_foreign" }

// Evaluate implements Rule.
func (r NewDeviceForeignRule) Evaluate(tx Transaction, f Features) RuleResult {
	score := 0
	var signals []string
	add := func(on bool, points int, signal string) {
		if on && points > 0 {
			score += points
			signals = append(signals, signal)
		}
	}
	add(!f.DeviceKnown, r.p.NewDeviceScore, "new device "+tx.DeviceID)
	add(!f.CountryKnown, r.p.NewCountryScore, "first purchase in "+tx.MerchantCountry)
	add(slices.Contains(r.p.RiskyMCCs, tx.MerchantMCC), r.p.RiskyMCCScore, "risky mcc "+tx.MerchantMCC)
	return RuleResult{Rule: r.Name(), Score: score, Reason: strings.Join(signals, ", ")}
}
