package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// NDJSONSender is the dry-run target: it writes each emission as one JSON line
// instead of sending it, so traffic can be inspected or replayed later.
type NDJSONSender struct {
	mu  sync.Mutex
	enc *json.Encoder
}

type dryRunRecord struct {
	Mode           Mode            `json:"mode"`
	Scenario       string          `json:"scenario"`
	TransactionID  string          `json:"transaction_id,omitempty"`
	Label          Label           `json:"label"`
	ExpectedStatus int             `json:"expected_status"`
	ContentType    string          `json:"content_type"`
	Body           json.RawMessage `json:"body,omitempty"`
	RawBody        string          `json:"raw_body,omitempty"`
}

// NewNDJSONSender writes to w.
//
//	sender := NewNDJSONSender(os.Stdout)
func NewNDJSONSender(w io.Writer) *NDJSONSender {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &NDJSONSender{enc: enc}
}

// Send writes e; valid JSON bodies are embedded as objects, others as a string.
func (s *NDJSONSender) Send(_ context.Context, e Emission) (SendResult, error) {
	rec := dryRunRecord{
		Mode:           e.Mode,
		Scenario:       e.Scenario,
		TransactionID:  e.TransactionID,
		Label:          e.Label,
		ExpectedStatus: e.ExpectedStatus,
		ContentType:    e.ContentType,
	}
	if json.Valid(e.Body) {
		rec.Body = e.Body
	} else {
		rec.RawBody = string(e.Body)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.enc.Encode(rec); err != nil {
		return SendResult{}, fmt.Errorf("write dry-run record for %s: %w", e.Scenario, err)
	}
	return SendResult{}, nil
}
