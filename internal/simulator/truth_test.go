package simulator

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

func TestTruthWriterWritesOneRecordPerOutcome(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tw := NewTruthWriter(&buf)
	e := Emission{Mode: ModeFraud, Scenario: "fraud/velocity", TransactionID: "t1", Label: LabelFraud, ExpectedStatus: 202}
	tw.Observe(Outcome{Emission: e, Result: SendResult{Status: 202}})
	tw.Observe(Outcome{Emission: e, Err: errors.New("timeout")})
	if err := tw.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var first, second groundtruth.Record
	if len(lines) != 2 || json.Unmarshal([]byte(lines[0]), &first) != nil || json.Unmarshal([]byte(lines[1]), &second) != nil {
		t.Fatalf("output %q, want 2 JSON lines", buf.String())
	}
	if first.Label != LabelFraud || first.Status != 202 || second.Error != "timeout" {
		t.Errorf("records = %+v / %+v, want fraud label, status 202, then error timeout", first, second)
	}
}

// failingWriter is a fake io.Writer that always fails.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestTruthWriterKeepsFirstError(t *testing.T) {
	t.Parallel()
	tw := NewTruthWriter(failingWriter{})
	tw.Observe(Outcome{Emission: Emission{Scenario: "s"}})
	if err := tw.Err(); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("Err() = %v, want error containing %q", err, "disk full")
	}
}
