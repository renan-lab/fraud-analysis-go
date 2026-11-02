package analysis

// Rule scores one fraud signal. Evaluate must be pure and deterministic:
// no I/O, no clock, only tx and f (ADR 0010). Return a zero Score when the
// rule does not fire.
type Rule interface {
	Name() string
	Evaluate(tx Transaction, f Features) RuleResult
}
