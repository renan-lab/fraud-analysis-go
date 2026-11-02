package analysis

import (
	"context"
	"fmt"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

// FeatureStore loads the state rules need and records analyzed transactions.
// Load must exclude tx itself and Record must be idempotent, so a redelivered
// transaction gets the same decision (ADR 0009).
type FeatureStore interface {
	Load(ctx context.Context, tx Transaction) (Features, error)
	Record(ctx context.Context, tx Transaction) error
}

// AnalyzedPublisher delivers TransactionAnalyzed events; it must return only
// after the broker acknowledged the event.
type AnalyzedPublisher interface {
	PublishTransactionAnalyzed(ctx context.Context, ev events.TransactionAnalyzed) error
}

// Service scores received transactions and publishes the decision.
type Service struct {
	store      FeatureStore
	publisher  AnalyzedPublisher
	rules      []Rule
	thresholds Thresholds
	clock      Clock
	newID      func() string
}

// ServiceDeps groups the collaborators of Service.
type ServiceDeps struct {
	Store      FeatureStore
	Publisher  AnalyzedPublisher
	Rules      []Rule
	Thresholds Thresholds
	Clock      Clock
	// NewID generates event IDs (ids.NewV4 in production).
	NewID func() string
}

// NewService validates the thresholds and requires at least one rule.
//
//	svc, err := analysis.NewService(analysis.ServiceDeps{Store: store, Publisher: pub, Rules: rules, Thresholds: th, Clock: analysis.SystemClock{}, NewID: ids.NewV4})
func NewService(d ServiceDeps) (*Service, error) {
	if err := d.Thresholds.Validate(); err != nil {
		return nil, err
	}
	if len(d.Rules) == 0 {
		return nil, fmt.Errorf("rules %v: want at least one rule", d.Rules)
	}
	return &Service{store: d.Store, publisher: d.Publisher, rules: d.Rules, thresholds: d.Thresholds, clock: d.Clock, newID: d.NewID}, nil
}

// Analyze loads features, evaluates the rules, records the transaction and
// publishes the decision, in that order. Any error is transient (Redis or
// Kafka): retrying the same event is safe and yields the same decision.
func (s *Service) Analyze(ctx context.Context, ev events.TransactionReceived) (Analysis, error) {
	tx := fromEventTransaction(ev.Transaction)
	features, err := s.store.Load(ctx, tx)
	if err != nil {
		return Analysis{}, fmt.Errorf("analyze %s: %w", tx.ID, err)
	}
	result := s.evaluate(tx, features)
	if err := s.store.Record(ctx, tx); err != nil {
		return Analysis{}, fmt.Errorf("analyze %s: %w", tx.ID, err)
	}
	if err := s.publisher.PublishTransactionAnalyzed(ctx, s.newEvent(ev.RequestID, result)); err != nil {
		return Analysis{}, fmt.Errorf("analyze %s: %w", tx.ID, err)
	}
	return result, nil
}

func (s *Service) evaluate(tx Transaction, f Features) Analysis {
	score, fired := evaluateRules(s.rules, tx, f)
	return Analysis{Transaction: tx, Decision: s.thresholds.Decide(score), Score: score, Results: fired}
}

func (s *Service) newEvent(requestID string, a Analysis) events.TransactionAnalyzed {
	tx := a.Transaction
	return events.TransactionAnalyzed{
		EventID:       s.newID(),
		EventType:     events.EventTypeTransactionAnalyzed,
		SchemaVersion: events.TransactionAnalyzedSchemaVersion,
		AnalyzedAt:    s.clock.Now().UTC(),
		RequestID:     requestID,
		TransactionID: tx.ID,
		AccountID:     tx.AccountID,
		CardID:        tx.CardID,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		OccurredAt:    tx.OccurredAt,
		Decision:      string(a.Decision),
		Score:         a.Score,
		RuleResults:   toEventResults(a.Results),
	}
}

func toEventResults(results []RuleResult) []events.RuleResult {
	out := make([]events.RuleResult, 0, len(results))
	for _, r := range results {
		out = append(out, events.RuleResult{Rule: r.Rule, Score: r.Score, Reason: r.Reason})
	}
	return out
}
