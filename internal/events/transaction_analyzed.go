package events

import "time"

// TopicTransactionsAnalyzed carries TransactionAnalyzed, keyed by transaction_id.
const TopicTransactionsAnalyzed = "transactions.analyzed"

const (
	EventTypeTransactionAnalyzed = "transaction.analyzed"
	// TransactionAnalyzedSchemaVersion changes only on breaking payload changes.
	TransactionAnalyzedSchemaVersion = 1
)

// Decision values of TransactionAnalyzed.Decision.
const (
	DecisionApprove = "approve"
	DecisionReview  = "review"
	DecisionBlock   = "block"
)

// RuleResult is one rule that fired: its name, the score it added and why.
type RuleResult struct {
	Rule   string `json:"rule"`
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

// TransactionAnalyzed is the JSON value published to TopicTransactionsAnalyzed.
// EventID is unique per publish: a redelivered transaction produces a new
// event with the same decision, so consumers upsert by TransactionID.
// RuleResults lists only the rules that fired and is never null.
type TransactionAnalyzed struct {
	EventID       string       `json:"event_id"`
	EventType     string       `json:"event_type"`
	SchemaVersion int          `json:"schema_version"`
	AnalyzedAt    time.Time    `json:"analyzed_at"`
	RequestID     string       `json:"request_id"`
	TransactionID string       `json:"transaction_id"`
	AccountID     string       `json:"account_id"`
	CardID        string       `json:"card_id"`
	Amount        int64        `json:"amount"`
	Currency      string       `json:"currency"`
	OccurredAt    time.Time    `json:"occurred_at"`
	Decision      string       `json:"decision"`
	Score         int          `json:"score"`
	RuleResults   []RuleResult `json:"rule_results"`
}
