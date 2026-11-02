package ingestion

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/httpmw"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

var testNow = time.Date(2026, 11, 2, 19, 15, 0, 0, time.UTC)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakePublisher records published events and fails with err when set.
type fakePublisher struct {
	mu     sync.Mutex
	err    error
	events []events.TransactionReceived
}

func (p *fakePublisher) PublishTransactionReceived(_ context.Context, ev events.TransactionReceived) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, ev)
	return nil
}

func (p *fakePublisher) published() []events.TransactionReceived {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]events.TransactionReceived(nil), p.events...)
}

// blockingPublisher waits for ctx to end, like a broker that never acknowledges.
type blockingPublisher struct{}

func (blockingPublisher) PublishTransactionReceived(ctx context.Context, _ events.TransactionReceived) error {
	<-ctx.Done()
	return ctx.Err()
}

// fakeProducer records Kafka messages and fails with err when set.
type fakeProducer struct {
	err      error
	messages []kafka.Message
}

func (p *fakeProducer) Publish(_ context.Context, msg kafka.Message) error {
	p.messages = append(p.messages, msg)
	return p.err
}

func sequentialIDs() func() string {
	n := 0
	return func() string {
		n++
		return "event-" + string(rune('0'+n))
	}
}

// validPayload returns the contract example as a mutable JSON object.
func validPayload() map[string]any {
	return map[string]any{
		"transaction_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"account_id":     "acc_3f9a1c2b7d4e",
		"card_id":        "card_9b2c4d6e8f10",
		"amount":         12990,
		"currency":       "BRL",
		"merchant":       map[string]any{"id": "mer_51a2b3c4d5e6", "name": "Grocery 5e6f", "mcc": "5411", "country": "BR"},
		"channel":        "card_present",
		"device_id":      "dev_0a1b2c3d4e5f",
		"ip":             "177.34.10.200",
		"occurred_at":    "2026-11-02T19:10:00Z",
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %v: %v", v, err)
	}
	return string(b)
}

func newTestHandler(pub Publisher) *Handler {
	svc := NewService(pub, fixedClock{testNow}, sequentialIDs(), time.Second)
	return NewHandler(svc, fixedClock{testNow}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func postTransaction(t *testing.T, h http.Handler, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/transactions", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	httpmw.RequestID(h).ServeHTTP(rec, req)
	return rec
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) httpmw.Problem {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != httpmw.ContentTypeProblem {
		t.Fatalf("Content-Type = %q, want %q (body %s)", ct, httpmw.ContentTypeProblem, rec.Body)
	}
	var p httpmw.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem %q: %v", rec.Body.String(), err)
	}
	return p
}
