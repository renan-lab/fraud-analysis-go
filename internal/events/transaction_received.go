// Package events holds the contracts modules exchange over Kafka: topic
// names, header keys and payload types. It has no logic and imports no module.
package events

import "time"

// TopicTransactionsReceived carries TransactionReceived, keyed by account_id
// so one worker analyzes an account at a time (ADR 0012).
const TopicTransactionsReceived = "transactions.received"

const (
	EventTypeTransactionReceived = "transaction.received"
	// TransactionReceivedSchemaVersion changes only on breaking payload changes;
	// adding optional fields keeps the version.
	TransactionReceivedSchemaVersion = 1
)

// Kafka header keys set on every event, so consumers can route or drop a
// record without decoding its value.
const (
	HeaderEventType     = "event_type"
	HeaderSchemaVersion = "schema_version"
	HeaderRequestID     = "request_id"
)

// Merchant mirrors the merchant block of the ingestion HTTP contract.
type Merchant struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	MCC     string `json:"mcc"`
	Country string `json:"country"`
}

// Location is where the purchase happened (terminal for in-person channels,
// device for ecommerce), in WGS 84 decimal degrees.
type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Transaction is a validated transaction as accepted by ingestion.
// Amount is in minor units of Currency; OccurredAt is UTC. Location is
// optional: nil when the client did not send it, omitted from the JSON.
type Transaction struct {
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	CardID        string    `json:"card_id"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	Merchant      Merchant  `json:"merchant"`
	Channel       string    `json:"channel"`
	DeviceID      string    `json:"device_id"`
	IP            string    `json:"ip"`
	OccurredAt    time.Time `json:"occurred_at"`
	Location      *Location `json:"location,omitempty"`
}

// TransactionReceived is the JSON value published to TopicTransactionsReceived.
// EventID is unique per publish: an HTTP duplicate of the same transaction
// gets a new EventID, so consumers dedupe by Transaction.TransactionID.
type TransactionReceived struct {
	EventID       string      `json:"event_id"`
	EventType     string      `json:"event_type"`
	SchemaVersion int         `json:"schema_version"`
	ReceivedAt    time.Time   `json:"received_at"`
	RequestID     string      `json:"request_id"`
	Transaction   Transaction `json:"transaction"`
}
