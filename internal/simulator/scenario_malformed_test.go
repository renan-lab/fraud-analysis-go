package simulator

import (
	"bytes"
	"testing"
)

func TestMalformedTruncatedBodyIsInvalidJSON(t *testing.T) {
	t.Parallel()
	body := []byte(`{"amount":100,"currency":"BRL"}`)
	got := truncateBody(body)
	if !bytes.HasPrefix(body, got) || len(got) != len(body)/2 {
		t.Errorf("truncateBody(%q) = %q, want first half", body, got)
	}
}
