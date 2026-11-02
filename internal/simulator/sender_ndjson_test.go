package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNDJSONSenderEmbedsValidJSONAndQuotesInvalid(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	sender := NewNDJSONSender(&buf)
	for _, body := range []string{`{"amount":1}`, `{"amount":`} {
		res, err := sender.Send(context.Background(), Emission{Scenario: "s", Body: []byte(body)})
		if err != nil || res.Status != 0 {
			t.Fatalf("Send(%q) = %+v, %v; want status 0, nil", body, res, err)
		}
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var valid, invalid dryRunRecord
	if len(lines) != 2 || json.Unmarshal([]byte(lines[0]), &valid) != nil || json.Unmarshal([]byte(lines[1]), &invalid) != nil {
		t.Fatalf("output %q, want 2 JSON lines", buf.String())
	}
	if string(valid.Body) != `{"amount":1}` || invalid.RawBody != `{"amount":` || invalid.Body != nil {
		t.Errorf("records = %+v / %+v, want body embedded then raw_body string", valid, invalid)
	}
}
