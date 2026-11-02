// Package simulator generates realistic and adversarial card transactions to
// exercise the ingestion API through its public HTTP contract only.
package simulator

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

// Mode groups scenarios by the kind of pressure they put on the service.
type Mode string

const (
	ModeLegit     Mode = "legit"
	ModeFraud     Mode = "fraud"
	ModeMalformed Mode = "malformed"
	ModeAttack    Mode = "attack"
	ModeDuplicate Mode = "duplicate"
	ModeTiming    Mode = "timing"
)

// Modes lists every mode in a stable order, used for parsing and reporting.
var Modes = []Mode{ModeLegit, ModeFraud, ModeMalformed, ModeAttack, ModeDuplicate, ModeTiming}

// Label is the ground truth the analysis pipeline is expected to reach.
type Label string

const (
	LabelLegit  Label = "legit"
	LabelFraud  Label = "fraud"
	LabelReject Label = "reject"
)

// Channel is how the card was used.
type Channel string

const (
	ChannelCardPresent Channel = "card_present"
	ChannelEcommerce   Channel = "ecommerce"
	ChannelContactless Channel = "contactless"
)

const contentTypeJSON = "application/json"

// Merchant is the merchant block of the ingestion contract.
type Merchant struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	MCC     string `json:"mcc"`
	Country string `json:"country"`
}

// Transaction is the POST /transactions payload documented in ARCHITECTURE.md.
type Transaction struct {
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	CardID        string    `json:"card_id"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	Merchant      Merchant  `json:"merchant"`
	Channel       Channel   `json:"channel"`
	DeviceID      string    `json:"device_id"`
	IP            string    `json:"ip"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// Emission is one HTTP request the simulator will send, plus what it expects back.
// Body is raw bytes so malformed and hostile payloads can be represented.
type Emission struct {
	Mode           Mode
	Scenario       string
	TransactionID  string
	Label          Label
	ExpectedStatus int
	ContentType    string
	Body           []byte
	// Copies > 1 sends the same request that many times concurrently.
	Copies int
}

func (e Emission) copies() int {
	return max(e.Copies, 1)
}

// acceptedEmission encodes a valid transaction that must be answered with 202.
func acceptedEmission(mode Mode, scenario string, label Label, tx Transaction) (Emission, error) {
	body, err := json.Marshal(tx)
	if err != nil {
		return Emission{}, fmt.Errorf("encode transaction %s for %s: %w", tx.TransactionID, scenario, err)
	}
	return Emission{
		Mode:           mode,
		Scenario:       scenario,
		TransactionID:  tx.TransactionID,
		Label:          label,
		ExpectedStatus: http.StatusAccepted,
		ContentType:    contentTypeJSON,
		Body:           body,
	}, nil
}

// rejectedEmission wraps a payload the ingestion API must refuse with status.
func rejectedEmission(mode Mode, scenario, txID string, status int, body []byte) Emission {
	return Emission{
		Mode:           mode,
		Scenario:       scenario,
		TransactionID:  txID,
		Label:          LabelReject,
		ExpectedStatus: status,
		ContentType:    contentTypeJSON,
		Body:           body,
	}
}

// newUUID returns a random UUID v4 drawn from r, so runs are reproducible by seed.
func newUUID(r *rand.Rand) string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], r.Uint64())
	binary.BigEndian.PutUint64(b[8:], r.Uint64())
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// newID returns an identifier such as "acc_3f9a1c2b7d4e".
func newID(r *rand.Rand, prefix string) string {
	return fmt.Sprintf("%s_%012x", prefix, r.Uint64()&0xffffffffffff)
}
