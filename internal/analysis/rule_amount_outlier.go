package analysis

import "fmt"

// OutlierParams configures AmountOutlierRule: an amount at least Ratio times
// the account median, once MinSamples past amounts exist, adds Score.
type OutlierParams struct {
	Ratio      float64
	MinSamples int
	Score      int
}

// AmountOutlierRule compares the amount with the account's own baseline
// instead of a global limit, so big spenders are not flagged for being big.
type AmountOutlierRule struct {
	p OutlierParams
}

// NewAmountOutlierRule builds an AmountOutlierRule.
//
//	analysis.NewAmountOutlierRule(analysis.OutlierParams{Ratio: 10, MinSamples: 5, Score: 60})
func NewAmountOutlierRule(p OutlierParams) AmountOutlierRule {
	return AmountOutlierRule{p: p}
}

// Name implements Rule.
func (AmountOutlierRule) Name() string { return "amount_outlier" }

// Evaluate implements Rule.
func (r AmountOutlierRule) Evaluate(tx Transaction, f Features) RuleResult {
	if len(f.AccountAmounts) < r.p.MinSamples {
		return RuleResult{Rule: r.Name()}
	}
	med := median(f.AccountAmounts)
	if med <= 0 {
		return RuleResult{Rule: r.Name()}
	}
	ratio := float64(tx.Amount) / med
	if ratio < r.p.Ratio {
		return RuleResult{Rule: r.Name()}
	}
	return RuleResult{
		Rule:   r.Name(),
		Score:  r.p.Score,
		Reason: fmt.Sprintf("amount %d is %.1fx the account median %.0f (limit %.1fx)", tx.Amount, ratio, med, r.p.Ratio),
	}
}
