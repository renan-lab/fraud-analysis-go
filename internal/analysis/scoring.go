package analysis

import "fmt"

// Thresholds maps a total score to a decision: >= Block blocks, >= Review
// sends to review, anything lower approves.
type Thresholds struct {
	Review int
	Block  int
}

// Validate rejects thresholds that cannot produce all three decisions.
func (t Thresholds) Validate() error {
	if t.Review < 1 || t.Block < t.Review {
		return fmt.Errorf("thresholds review=%d block=%d: want 1 <= review <= block", t.Review, t.Block)
	}
	return nil
}

// Decide maps score to a decision.
//
//	Thresholds{Review: 50, Block: 80}.Decide(60) // DecisionReview
func (t Thresholds) Decide(score int) Decision {
	switch {
	case score >= t.Block:
		return DecisionBlock
	case score >= t.Review:
		return DecisionReview
	}
	return DecisionApprove
}

// evaluateRules runs every rule and returns the summed score and the rules that fired.
func evaluateRules(rules []Rule, tx Transaction, f Features) (int, []RuleResult) {
	total := 0
	fired := []RuleResult{}
	for _, r := range rules {
		res := r.Evaluate(tx, f)
		if !res.Fired() {
			continue
		}
		total += res.Score
		fired = append(fired, res)
	}
	return total, fired
}
