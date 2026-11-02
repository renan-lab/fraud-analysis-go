// Package groundtruth is the contract of the ground-truth NDJSON file: the
// simulator writes one Record per send (simulator run --truth) and the
// evaluation module reads it back. It holds types only, no logic.
package groundtruth

// Label is the outcome the pipeline is expected to reach for a transaction.
type Label string

const (
	// LabelLegit is a genuine purchase the analysis should approve.
	LabelLegit Label = "legit"
	// LabelFraud is a fraudulent purchase the analysis should flag.
	LabelFraud Label = "fraud"
	// LabelReject is a payload the ingestion API must refuse (4xx).
	LabelReject Label = "reject"
)

// StatusAccepted is the HTTP status of a send that reached Kafka; only those
// records can have an analysis decision.
const StatusAccepted = 202

// Record is one line of the ground-truth file. TransactionID is empty when
// the payload had none (malformed bodies); Status is 0 and Error is set when
// the request failed before a response arrived. A duplicate scenario writes
// one Record per send, so a transaction_id may appear more than once.
type Record struct {
	TransactionID  string `json:"transaction_id,omitempty"`
	Mode           string `json:"mode"`
	Scenario       string `json:"scenario"`
	Label          Label  `json:"label"`
	ExpectedStatus int    `json:"expected_status"`
	Status         int    `json:"status,omitempty"`
	Error          string `json:"error,omitempty"`
}
