package groundtruth

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestRecordWireFormat locks the NDJSON shape shared by simulator and
// evaluation. A diff here breaks truth files written by older runs.
func TestRecordWireFormat(t *testing.T) {
	t.Parallel()
	records := []Record{
		{TransactionID: "7c9e6679-7425-40de-944b-e07fc1f90ae7", Mode: "fraud", Scenario: "fraud/velocity", Label: LabelFraud, ExpectedStatus: 202, Status: 202},
		{Mode: "malformed", Scenario: "malformed/empty_body", Label: LabelReject, ExpectedStatus: 400, Error: "timeout"},
	}
	var got bytes.Buffer
	enc := json.NewEncoder(&got)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("encode %+v: %v", r, err)
		}
	}
	want, err := os.ReadFile(filepath.Join("testdata", "records.golden.ndjson"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(got.Bytes()), bytes.TrimSpace(want)) {
		t.Fatalf("wire format changed:\ngot:\n%s\nwant:\n%s", got.Bytes(), want)
	}
}
