package ingestion

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeTransactionStreamingBodyLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{"oversized value", `{"transaction_id":"` + strings.Repeat("a", maxBodyBytes) + `"}`},
		{"oversized trailing data", `{"transaction_id":"x"}` + strings.Repeat(" ", maxBodyBytes)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// io.MultiReader hides the length, so ContentLength is -1 and only MaxBytesReader can catch it.
			req := httptest.NewRequest(http.MethodPost, "/transactions", io.MultiReader(strings.NewReader(tt.body)))
			req.Header.Set("Content-Type", contentTypeJSON)
			req.ContentLength = -1
			_, err := decodeTransaction(httptest.NewRecorder(), req)
			if !errors.Is(err, errBodyTooLarge) {
				t.Fatalf("decodeTransaction(%d bytes, unknown length) error = %v, want errBodyTooLarge", len(tt.body), err)
			}
		})
	}
}

func TestIsJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"Application/JSON", true},
		{"text/plain", false},
		{"application/jsonx", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isJSON(tt.in); got != tt.want {
			t.Errorf("isJSON(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
