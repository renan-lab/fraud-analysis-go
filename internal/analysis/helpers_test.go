package analysis

import (
	"context"
	"sync"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

var testNow = time.Date(2026, 11, 2, 19, 15, 0, 0, time.UTC)

var (
	saoPaulo = Location{Lat: -23.5505, Lon: -46.6333}
	campinas = Location{Lat: -22.9056, Lon: -47.0608}
	lisbon   = Location{Lat: 38.7223, Lon: -9.1393}
)

// newTestTransaction is a small in-person grocery purchase in São Paulo.
func newTestTransaction() Transaction {
	return Transaction{
		ID:              "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		AccountID:       "acc_test",
		CardID:          "card_test",
		Amount:          5000,
		Currency:        "BRL",
		MerchantMCC:     "5411",
		MerchantCountry: "BR",
		Channel:         ChannelCardPresent,
		DeviceID:        "dev_test",
		OccurredAt:      testNow,
		Location:        &saoPaulo,
	}
}

func cardEvent(ago time.Duration, amount int64, channel string, loc *Location) CardEvent {
	return CardEvent{TransactionID: "past-" + ago.String(), At: testNow.Add(-ago), Amount: amount, Channel: channel, Location: loc}
}

// repeatEvents returns n events spaced by gap, the newest gap before testNow.
func repeatEvents(n int, gap time.Duration, amount int64, channel string) []CardEvent {
	out := make([]CardEvent, 0, n)
	for i := n; i >= 1; i-- {
		out = append(out, cardEvent(time.Duration(i)*gap, amount, channel, nil))
	}
	return out
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// staticRule always returns the same result; used to drive scoring.
type staticRule struct {
	name  string
	score int
}

func (r staticRule) Name() string { return r.name }

func (r staticRule) Evaluate(Transaction, Features) RuleResult {
	return RuleResult{Rule: r.name, Score: r.score, Reason: r.name + " fired"}
}

// fakeFeatureStore returns fixed features and records every write.
type fakeFeatureStore struct {
	mu        sync.Mutex
	features  Features
	loadErr   error
	recordErr error
	recorded  []Transaction
}

func (s *fakeFeatureStore) Load(context.Context, Transaction) (Features, error) {
	return s.features, s.loadErr
}

func (s *fakeFeatureStore) Record(_ context.Context, tx Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recordErr != nil {
		return s.recordErr
	}
	s.recorded = append(s.recorded, tx)
	return nil
}

// fakeAnalyzedPublisher keeps published events; errs are returned in order, then nil.
type fakeAnalyzedPublisher struct {
	mu        sync.Mutex
	published []events.TransactionAnalyzed
	errs      []error
}

func (p *fakeAnalyzedPublisher) PublishTransactionAnalyzed(_ context.Context, ev events.TransactionAnalyzed) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.errs) > 0 {
		err := p.errs[0]
		p.errs = p.errs[1:]
		return err
	}
	p.published = append(p.published, ev)
	return nil
}

// fakeAnalyzer returns errs in order, then succeeds; it counts calls.
type fakeAnalyzer struct {
	mu    sync.Mutex
	errs  []error
	calls int
}

func (a *fakeAnalyzer) Analyze(_ context.Context, ev events.TransactionReceived) (Analysis, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if len(a.errs) > 0 {
		err := a.errs[0]
		a.errs = a.errs[1:]
		return Analysis{}, err
	}
	return Analysis{Transaction: fromEventTransaction(ev.Transaction), Decision: DecisionApprove}, nil
}

func (a *fakeAnalyzer) callCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

// fakeMessageProducer records published messages.
type fakeMessageProducer struct {
	messages []kafka.Message
	err      error
}

func (p *fakeMessageProducer) Publish(_ context.Context, msg kafka.Message) error {
	if p.err != nil {
		return p.err
	}
	p.messages = append(p.messages, msg)
	return nil
}

func newTestReceived() events.TransactionReceived {
	return events.TransactionReceived{
		EventID:       "0b6f3c1e-2d4a-4f5b-8c7d-9e0f1a2b3c4d",
		EventType:     events.EventTypeTransactionReceived,
		SchemaVersion: events.TransactionReceivedSchemaVersion,
		ReceivedAt:    testNow,
		RequestID:     "5f1d2c3b-4a59-4687-9a1b-2c3d4e5f6a7b",
		Transaction: events.Transaction{
			TransactionID: "7c9e6679-7425-40de-944b-e07fc1f90ae7",
			AccountID:     "acc_test",
			CardID:        "card_test",
			Amount:        5000,
			Currency:      "BRL",
			Merchant:      events.Merchant{ID: "mer_test", Name: "Grocery", MCC: "5411", Country: "BR"},
			Channel:       ChannelCardPresent,
			DeviceID:      "dev_test",
			IP:            "177.34.10.200",
			OccurredAt:    testNow,
			Location:      &events.Location{Lat: saoPaulo.Lat, Lon: saoPaulo.Lon},
		},
	}
}
