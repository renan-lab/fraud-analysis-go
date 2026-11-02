package evaluation

// Report is the full evaluation of one run.
type Report struct {
	Pipeline            PipelineStats        `json:"pipeline"`
	Transactions        TransactionStats     `json:"transactions"`
	Scenarios           []ScenarioDecisions  `json:"scenarios"`
	Sequences           []SequenceStats      `json:"sequences"`
	SequenceRecall      float64              `json:"sequence_recall"`
	Rules               []RuleFirings        `json:"rules"`
	FalsePositiveCauses []FalsePositiveCause `json:"false_positive_causes"`
}

// BuildReport computes every section from r; it does no I/O.
//
//	report := evaluation.BuildReport(collector.Results())
func BuildReport(r Results) Report {
	txs := r.analyzed()
	sequences := sequenceStats(txs)
	return Report{
		Pipeline:            pipelineStats(r),
		Transactions:        transactionStats(txs),
		Scenarios:           scenarioDecisions(txs),
		Sequences:           sequences,
		SequenceRecall:      sequenceRecall(sequences),
		Rules:               ruleFirings(txs),
		FalsePositiveCauses: falsePositiveCauses(txs),
	}
}
