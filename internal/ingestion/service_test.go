package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func validTransaction() Transaction {
	return Transaction{
		TransactionID: "7c9e6679-7425-40de-944b-e07fc1f90ae7", AccountID: "acc_1", CardID: "card_1", Amount: 100, Currency: "BRL",
		Merchant: Merchant{ID: "mer_1", Name: "Shop", MCC: "5411", Country: "BR"}, Channel: ChannelEcommerce,
		DeviceID: "dev_1", IP: "10.0.0.1", OccurredAt: testNow.Add(-time.Minute),
	}
}

func TestServiceIngestBuildsEnvelope(t *testing.T) {
	t.Parallel()
	pub := &fakePublisher{}
	svc := NewService(pub, fixedClock{testNow}, sequentialIDs(), time.Second)
	if err := svc.Ingest(context.Background(), validTransaction(), "req-1"); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	evs := pub.published()
	if len(evs) != 1 {
		t.Fatalf("published %d events, want 1", len(evs))
	}
	ev := evs[0]
	if ev.EventID != "event-1" || ev.EventType != events.EventTypeTransactionReceived || ev.SchemaVersion != 1 ||
		!ev.ReceivedAt.Equal(testNow) || ev.RequestID != "req-1" {
		t.Fatalf("envelope = %+v, want event-1, transaction.received v1, received_at %v, req-1", ev, testNow)
	}
	if ev.Transaction.Channel != "ecommerce" || ev.Transaction.Merchant.MCC != "5411" || ev.Transaction.Amount != 100 {
		t.Fatalf("event transaction = %+v, want fields copied from input", ev.Transaction)
	}
}

func TestServiceIngestWrapsPublishError(t *testing.T) {
	t.Parallel()
	brokerErr := errors.New("not enough replicas")
	svc := NewService(&fakePublisher{err: brokerErr}, fixedClock{testNow}, sequentialIDs(), time.Second)
	err := svc.Ingest(context.Background(), validTransaction(), "req-1")
	if !errors.Is(err, ErrPublishFailed) || !errors.Is(err, brokerErr) {
		t.Fatalf("Ingest error = %v, want ErrPublishFailed wrapping %v", err, brokerErr)
	}
}

func TestServiceIngestBoundsPublishByTimeout(t *testing.T) {
	t.Parallel()
	svc := NewService(blockingPublisher{}, fixedClock{testNow}, sequentialIDs(), 20*time.Millisecond)
	start := time.Now()
	err := svc.Ingest(context.Background(), validTransaction(), "req-1")
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrPublishFailed) {
		t.Fatalf("Ingest error = %v, want ErrPublishFailed wrapping DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Ingest took %v, want about the 20ms publish timeout", elapsed)
	}
}
