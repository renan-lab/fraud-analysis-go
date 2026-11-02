package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// fakeBroker records published messages; pingErr makes /ready fail.
type fakeBroker struct {
	mu       sync.Mutex
	pingErr  error
	messages []kafka.Message
}

func (b *fakeBroker) Publish(_ context.Context, msg kafka.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, msg)
	return nil
}

func (b *fakeBroker) Ping(context.Context) error { return b.pingErr }

func testConfig() config.Config {
	return config.Config{IngestPublishTimeout: time.Second}
}

func newTestRouter(b broker) http.Handler {
	return newRouter(testConfig(), b, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func doRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func validBody() string {
	occurred := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	return `{"transaction_id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","account_id":"acc_1","card_id":"card_1",` +
		`"amount":12990,"currency":"BRL","merchant":{"id":"mer_1","name":"Shop","mcc":"5411","country":"BR"},` +
		`"channel":"ecommerce","device_id":"dev_1","ip":"10.0.0.1","occurred_at":"` + occurred + `"}`
}

func TestRouterPostTransactionPublishes(t *testing.T) {
	t.Parallel()
	b := &fakeBroker{}
	rec := doRequest(t, newTestRouter(b), http.MethodPost, "/transactions", validBody())
	if rec.Code != http.StatusAccepted || rec.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status = %d request id %q body %s, want 202 with X-Request-ID", rec.Code, rec.Header().Get("X-Request-ID"), rec.Body)
	}
	if len(b.messages) != 1 || b.messages[0].Topic != "transactions.received" {
		t.Fatalf("messages = %+v, want one on transactions.received", b.messages)
	}
}

func TestRouterReady(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pingErr error
		want    int
	}{
		{nil, http.StatusOK},
		{errors.New("no brokers"), http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		rec := doRequest(t, newTestRouter(&fakeBroker{pingErr: tt.pingErr}), http.MethodGet, "/ready", "")
		if rec.Code != tt.want {
			t.Errorf("GET /ready with ping error %v: status %d, want %d", tt.pingErr, rec.Code, tt.want)
		}
	}
}

func TestRouterMetricsExposesRequestCounts(t *testing.T) {
	t.Parallel()
	h := newTestRouter(&fakeBroker{})
	doRequest(t, h, http.MethodGet, "/health", "")
	rec := doRequest(t, h, http.MethodGet, "/metrics", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `http_requests_total{method="GET",route="/health",status="200"} 1`) {
		t.Fatalf("GET /metrics status %d, want 200 with the /health counter; body:\n%.2000s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatalf("GET /metrics missing the go runtime collector")
	}
}

func TestRouterRejectsWrongMethod(t *testing.T) {
	t.Parallel()
	rec := doRequest(t, newTestRouter(&fakeBroker{}), http.MethodGet, "/transactions", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /transactions status %d, want 405", rec.Code)
	}
}
