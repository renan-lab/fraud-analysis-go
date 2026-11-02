package evaluation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

// maxTruthLine bounds one NDJSON line; records are ~200 bytes.
const maxTruthLine = 64 * 1024

// ReadTruth parses a ground-truth NDJSON file. Lines without transaction_id
// or with a status other than 202 are only counted (NotAccepted): they never
// reached Kafka, so they have no decision to evaluate.
//
//	truth, err := evaluation.ReadTruth(file)
func ReadTruth(r io.Reader) (TruthSet, error) {
	set := TruthSet{Expectations: map[string]Expectation{}}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 4096), maxTruthLine)
	for scanner.Scan() {
		set.Lines++
		rec, err := decodeTruthLine(scanner.Bytes(), set.Lines)
		if err != nil {
			return TruthSet{}, err
		}
		set.add(rec)
	}
	if err := scanner.Err(); err != nil {
		return TruthSet{}, fmt.Errorf("read truth after line %d: %w", set.Lines, err)
	}
	return set, nil
}

func decodeTruthLine(line []byte, n int) (groundtruth.Record, error) {
	var rec groundtruth.Record
	if err := json.Unmarshal(line, &rec); err != nil {
		return rec, fmt.Errorf("truth line %d %q: %w; want a groundtruth.Record JSON object", n, truncate(line), err)
	}
	return rec, nil
}

func (s *TruthSet) add(rec groundtruth.Record) {
	if rec.TransactionID == "" || rec.Status != groundtruth.StatusAccepted {
		s.NotAccepted++
		return
	}
	if _, seen := s.Expectations[rec.TransactionID]; seen {
		return
	}
	s.Expectations[rec.TransactionID] = Expectation{TransactionID: rec.TransactionID, Scenario: rec.Scenario, Label: rec.Label}
	s.Order = append(s.Order, rec.TransactionID)
}

func truncate(line []byte) string {
	const limit = 80
	if len(line) <= limit {
		return string(line)
	}
	return string(line[:limit]) + "..."
}
