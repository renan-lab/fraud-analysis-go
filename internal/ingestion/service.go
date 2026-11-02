package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

// ErrPublishFailed means the event was not acknowledged by the broker; the
// handler answers 503 and the client may retry with the same transaction_id.
var ErrPublishFailed = errors.New("publish transaction received event")

// Publisher delivers TransactionReceived events; it must return only after
// the broker acknowledged the event.
type Publisher interface {
	PublishTransactionReceived(ctx context.Context, ev events.TransactionReceived) error
}

// Service turns validated transactions into TransactionReceived events.
type Service struct {
	publisher      Publisher
	clock          Clock
	newID          func() string
	publishTimeout time.Duration
}

// NewService publishes through publisher, bounding each publish by publishTimeout.
// newID generates event IDs (ids.NewV4 in production).
//
//	svc := ingestion.NewService(pub, ingestion.SystemClock{}, ids.NewV4, 5*time.Second)
func NewService(publisher Publisher, clock Clock, newID func() string, publishTimeout time.Duration) *Service {
	return &Service{publisher: publisher, clock: clock, newID: newID, publishTimeout: publishTimeout}
}

// Ingest publishes tx and returns only after the broker acknowledged it.
// Duplicates are published again: consumers dedupe by transaction_id (ADR 0002).
func (s *Service) Ingest(ctx context.Context, tx Transaction, requestID string) error {
	ctx, cancel := context.WithTimeout(ctx, s.publishTimeout)
	defer cancel()
	if err := s.publisher.PublishTransactionReceived(ctx, s.newEvent(tx, requestID)); err != nil {
		return fmt.Errorf("%w: transaction %s: %w", ErrPublishFailed, tx.TransactionID, err)
	}
	return nil
}

func (s *Service) newEvent(tx Transaction, requestID string) events.TransactionReceived {
	return events.TransactionReceived{
		EventID:       s.newID(),
		EventType:     events.EventTypeTransactionReceived,
		SchemaVersion: events.TransactionReceivedSchemaVersion,
		ReceivedAt:    s.clock.Now().UTC(),
		RequestID:     requestID,
		Transaction:   toEventTransaction(tx),
	}
}

func toEventTransaction(tx Transaction) events.Transaction {
	return events.Transaction{
		TransactionID: tx.TransactionID,
		AccountID:     tx.AccountID,
		CardID:        tx.CardID,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		Merchant:      events.Merchant(tx.Merchant),
		Channel:       string(tx.Channel),
		DeviceID:      tx.DeviceID,
		IP:            tx.IP,
		OccurredAt:    tx.OccurredAt,
	}
}
