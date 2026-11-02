// Package analysis consumes transactions.received, scores each transaction
// with pure rules over features loaded from Redis, and publishes
// transactions.analyzed. See README.md for the flow and invariants.
package analysis

import (
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

// Channel values mirror the ingestion contract.
const (
	ChannelCardPresent = "card_present"
	ChannelEcommerce   = "ecommerce"
	ChannelContactless = "contactless"
)

// Decision is the outcome of scoring a transaction.
type Decision string

const (
	DecisionApprove Decision = events.DecisionApprove
	DecisionReview  Decision = events.DecisionReview
	DecisionBlock   Decision = events.DecisionBlock
)

// Location is a WGS 84 point in decimal degrees.
type Location struct {
	Lat float64
	Lon float64
}

// Transaction is the slice of a received transaction the rules look at.
// Amount is in minor units; OccurredAt is UTC; Location may be nil.
type Transaction struct {
	ID              string
	AccountID       string
	CardID          string
	Amount          int64
	Currency        string
	MerchantMCC     string
	MerchantCountry string
	Channel         string
	DeviceID        string
	OccurredAt      time.Time
	Location        *Location
}

// CardEvent is a past transaction of the same card, as stored in Redis.
type CardEvent struct {
	TransactionID string
	At            time.Time
	Amount        int64
	Channel       string
	Location      *Location
}

// Features is the state rules evaluate against. It never contains the
// transaction being analyzed, so a redelivery sees the same features.
type Features struct {
	// CardHistory holds card events with At in [tx.OccurredAt - retention, tx.OccurredAt], oldest first.
	CardHistory []CardEvent
	// AccountAmounts holds the most recent amounts of the account up to tx.OccurredAt.
	AccountAmounts []int64
	DeviceKnown    bool
	CountryKnown   bool
}

// RuleResult is the verdict of one rule; Score 0 means the rule did not fire.
type RuleResult struct {
	Rule   string
	Score  int
	Reason string
}

// Fired reports whether the rule added to the score.
func (r RuleResult) Fired() bool {
	return r.Score > 0
}

// Analysis is the scored outcome of one transaction.
type Analysis struct {
	Transaction Transaction
	Decision    Decision
	Score       int
	// Results lists only the rules that fired, in rule order.
	Results []RuleResult
}

// Clock returns the current time; injected so tests control analyzed_at.
type Clock interface {
	Now() time.Time
}

// SystemClock is the wall clock.
type SystemClock struct{}

// Now returns time.Now().
func (SystemClock) Now() time.Time { return time.Now() }

func fromEventTransaction(tx events.Transaction) Transaction {
	return Transaction{
		ID:              tx.TransactionID,
		AccountID:       tx.AccountID,
		CardID:          tx.CardID,
		Amount:          tx.Amount,
		Currency:        tx.Currency,
		MerchantMCC:     tx.Merchant.MCC,
		MerchantCountry: tx.Merchant.Country,
		Channel:         tx.Channel,
		DeviceID:        tx.DeviceID,
		OccurredAt:      tx.OccurredAt.UTC(),
		Location:        fromEventLocation(tx.Location),
	}
}

func fromEventLocation(l *events.Location) *Location {
	if l == nil {
		return nil
	}
	return &Location{Lat: l.Lat, Lon: l.Lon}
}
