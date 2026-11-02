package simulator

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

// TruthWriter records the expected label of every sent emission as NDJSON, to
// compare later with the decisions the analysis pipeline produced. The line
// format is groundtruth.Record.
type TruthWriter struct {
	mu  sync.Mutex
	enc *json.Encoder
	err error
}

// NewTruthWriter writes to w.
//
//	truth := NewTruthWriter(file)
func NewTruthWriter(w io.Writer) *TruthWriter {
	return &TruthWriter{enc: json.NewEncoder(w)}
}

// Observe writes one line per outcome; the first write error is kept for Err.
func (t *TruthWriter) Observe(o Outcome) {
	rec := groundtruth.Record{
		TransactionID:  o.Emission.TransactionID,
		Mode:           string(o.Emission.Mode),
		Scenario:       o.Emission.Scenario,
		Label:          o.Emission.Label,
		ExpectedStatus: o.Emission.ExpectedStatus,
		Status:         o.Result.Status,
	}
	if o.Err != nil {
		rec.Error = o.Err.Error()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.enc.Encode(rec); err != nil && t.err == nil {
		t.err = fmt.Errorf("write truth record for %s: %w", o.Emission.Scenario, err)
	}
}

// Err returns the first write error, if any.
func (t *TruthWriter) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}
