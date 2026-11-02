// Package ingestion implements POST /transactions: it validates incoming
// transactions against the HTTP contract and publishes them to Kafka.
package ingestion

import "time"

// Channel is how the card was used.
type Channel string

const (
	ChannelCardPresent Channel = "card_present"
	ChannelEcommerce   Channel = "ecommerce"
	ChannelContactless Channel = "contactless"
)

var channels = []Channel{ChannelCardPresent, ChannelEcommerce, ChannelContactless}

// Merchant is the merchant block of a validated transaction.
type Merchant struct {
	ID      string
	Name    string
	MCC     string
	Country string
}

// Location is where the purchase happened: the terminal for in-person
// channels, the device for ecommerce. Coordinates are WGS 84 decimal degrees.
type Location struct {
	Lat float64
	Lon float64
}

// Transaction is a payload that passed every contract rule. Amount is in
// minor units of Currency; OccurredAt is UTC; Location is nil when not sent.
type Transaction struct {
	TransactionID string
	AccountID     string
	CardID        string
	Amount        int64
	Currency      string
	Merchant      Merchant
	Channel       Channel
	DeviceID      string
	IP            string
	OccurredAt    time.Time
	Location      *Location
}

// transactionRequest is the raw JSON body. Pointer fields tell a missing (or
// null) field apart from a zero value, so "amount": 0 and no amount get
// different errors. Amount is int64 so floats, strings and values beyond
// int64 fail at decode time instead of being rounded.
type transactionRequest struct {
	TransactionID *string          `json:"transaction_id"`
	AccountID     *string          `json:"account_id"`
	CardID        *string          `json:"card_id"`
	Amount        *int64           `json:"amount"`
	Currency      *string          `json:"currency"`
	Merchant      *merchantRequest `json:"merchant"`
	Channel       *string          `json:"channel"`
	DeviceID      *string          `json:"device_id"`
	IP            *string          `json:"ip"`
	OccurredAt    *string          `json:"occurred_at"`
	Location      *locationRequest `json:"location"`
}

type merchantRequest struct {
	ID      *string `json:"id"`
	Name    *string `json:"name"`
	MCC     *string `json:"mcc"`
	Country *string `json:"country"`
}

type locationRequest struct {
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
}

// Clock returns the current time; tests inject a fixed one.
type Clock interface {
	Now() time.Time
}

// SystemClock reads the wall clock in UTC.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
