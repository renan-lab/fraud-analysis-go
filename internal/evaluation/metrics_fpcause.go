package evaluation

import (
	"sort"
	"strings"

	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

// Causes of a flagged legit transaction.
const (
	// CauseFraudBeforeOnCard: the card had a fraud transaction earlier in the
	// run. The simulator labels the next purchase legit, but a real issuer
	// would treat the card as compromised, so these are label semantics more
	// than engine errors.
	CauseFraudBeforeOnCard = "fraud-before-on-card"
	// CauseCleanCard: no earlier fraud on the card; the rules misfired.
	CauseCleanCard = "clean-card"
)

// noRules names the rule set of a flagged transaction where no rule fired.
const noRules = "none"

// FalsePositiveCause counts flagged legit transactions by cause and by the
// rules that fired (joined with "+").
type FalsePositiveCause struct {
	Cause string `json:"cause"`
	Rules string `json:"rules"`
	Count int    `json:"count"`
}

type causeKey struct {
	cause string
	rules string
}

func falsePositiveCauses(txs []analyzedTransaction) []FalsePositiveCause {
	counts := map[causeKey]int{}
	for _, card := range byCardChronological(txs) {
		countCardFalsePositives(card, counts)
	}
	return sortCauses(counts)
}

// countCardFalsePositives walks one card in time order, remembering whether
// a fraud transaction already happened.
func countCardFalsePositives(card []analyzedTransaction, counts map[causeKey]int) {
	sawFraud := false
	for _, tx := range card {
		if tx.Label == groundtruth.LabelLegit && flagged(tx.Decision) {
			counts[causeKey{cause: causeOf(sawFraud), rules: ruleSet(tx.Rules)}]++
		}
		sawFraud = sawFraud || tx.Label == groundtruth.LabelFraud
	}
}

func causeOf(sawFraud bool) string {
	if sawFraud {
		return CauseFraudBeforeOnCard
	}
	return CauseCleanCard
}

func ruleSet(rules []string) string {
	if len(rules) == 0 {
		return noRules
	}
	return strings.Join(rules, "+")
}

// byCardChronological groups by card and orders by occurred_at; ties keep
// truth file order, which is send order.
func byCardChronological(txs []analyzedTransaction) map[string][]analyzedTransaction {
	cards := map[string][]analyzedTransaction{}
	for _, tx := range txs {
		cards[tx.CardID] = append(cards[tx.CardID], tx)
	}
	for _, card := range cards {
		sort.SliceStable(card, func(i, j int) bool { return card[i].OccurredAt.Before(card[j].OccurredAt) })
	}
	return cards
}

func sortCauses(counts map[causeKey]int) []FalsePositiveCause {
	out := make([]FalsePositiveCause, 0, len(counts))
	for key, n := range counts {
		out = append(out, FalsePositiveCause{Cause: key.cause, Rules: key.rules, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Cause+out[i].Rules < out[j].Cause+out[j].Rules
	})
	return out
}
