package analysis

import (
	"context"
	"errors"
	"testing"
)

func newTestService(t *testing.T, store FeatureStore, pub AnalyzedPublisher, rules ...Rule) *Service {
	t.Helper()
	svc, err := NewService(ServiceDeps{
		Store: store, Publisher: pub, Rules: rules,
		Thresholds: Thresholds{Review: 50, Block: 80},
		Clock:      fixedClock{now: testNow},
		NewID:      func() string { return "event-1" },
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestServiceAnalyzePublishesDecision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		rules        []Rule
		wantDecision Decision
		wantScore    int
	}{
		{name: "nothing fired", rules: []Rule{staticRule{name: "quiet"}}, wantDecision: DecisionApprove, wantScore: 0},
		{name: "review", rules: []Rule{staticRule{name: "a", score: 30}, staticRule{name: "b", score: 25}}, wantDecision: DecisionReview, wantScore: 55},
		{name: "block", rules: []Rule{staticRule{name: "a", score: 80}}, wantDecision: DecisionBlock, wantScore: 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, pub := &fakeFeatureStore{}, &fakeAnalyzedPublisher{}
			got, err := newTestService(t, store, pub, tt.rules...).Analyze(context.Background(), newTestReceived())
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if got.Decision != tt.wantDecision || got.Score != tt.wantScore {
				t.Errorf("Analyze = %s/%d, want %s/%d", got.Decision, got.Score, tt.wantDecision, tt.wantScore)
			}
			assertPublished(t, pub, tt.wantDecision, tt.wantScore)
			if len(store.recorded) != 1 {
				t.Errorf("recorded %d transactions, want 1", len(store.recorded))
			}
		})
	}
}

func assertPublished(t *testing.T, pub *fakeAnalyzedPublisher, decision Decision, score int) {
	t.Helper()
	if len(pub.published) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.published))
	}
	ev, in := pub.published[0], newTestReceived()
	if ev.TransactionID != in.Transaction.TransactionID || ev.RequestID != in.RequestID || ev.EventID != "event-1" {
		t.Errorf("event ids = tx %s req %s event %s, want %s/%s/event-1", ev.TransactionID, ev.RequestID, ev.EventID, in.Transaction.TransactionID, in.RequestID)
	}
	if ev.Decision != string(decision) || ev.Score != score || !ev.AnalyzedAt.Equal(testNow) || ev.RuleResults == nil {
		t.Errorf("event = %+v, want decision %s score %d analyzed_at %s and non-nil rule_results", ev, decision, score, testNow)
	}
}

func TestServiceAnalyzeStopsOnStoreErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("redis down")
	tests := []struct {
		name  string
		store *fakeFeatureStore
	}{
		{name: "load fails", store: &fakeFeatureStore{loadErr: boom}},
		{name: "record fails", store: &fakeFeatureStore{recordErr: boom}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pub := &fakeAnalyzedPublisher{}
			_, err := newTestService(t, tt.store, pub, staticRule{name: "a", score: 10}).Analyze(context.Background(), newTestReceived())
			if !errors.Is(err, boom) {
				t.Fatalf("Analyze error = %v, want wrapping %v", err, boom)
			}
			if len(pub.published) != 0 {
				t.Errorf("published %d events after store failure, want 0", len(pub.published))
			}
		})
	}
}

func TestServiceAnalyzeReturnsPublishError(t *testing.T) {
	t.Parallel()
	boom := errors.New("kafka down")
	store, pub := &fakeFeatureStore{}, &fakeAnalyzedPublisher{errs: []error{boom}}
	_, err := newTestService(t, store, pub, staticRule{name: "a"}).Analyze(context.Background(), newTestReceived())
	if !errors.Is(err, boom) {
		t.Fatalf("Analyze error = %v, want wrapping %v", err, boom)
	}
}

// TestServiceReplayGivesSameDecision relies on the FeatureStore contract: the
// store returns the same features on redelivery, so the decision repeats.
func TestServiceReplayGivesSameDecision(t *testing.T) {
	t.Parallel()
	store, pub := &fakeFeatureStore{features: Features{AccountAmounts: []int64{100, 100, 100, 100, 100}}}, &fakeAnalyzedPublisher{}
	svc := newTestService(t, store, pub, NewAmountOutlierRule(OutlierParams{Ratio: 10, MinSamples: 5, Score: 60}))
	first, err1 := svc.Analyze(context.Background(), newTestReceived())
	second, err2 := svc.Analyze(context.Background(), newTestReceived())
	if err1 != nil || err2 != nil {
		t.Fatalf("Analyze errors = %v, %v", err1, err2)
	}
	if first.Decision != second.Decision || first.Score != second.Score || first.Decision != DecisionReview {
		t.Fatalf("replay = %s/%d then %s/%d, want review twice", first.Decision, first.Score, second.Decision, second.Score)
	}
}

func TestNewServiceValidates(t *testing.T) {
	t.Parallel()
	if _, err := NewService(ServiceDeps{Rules: []Rule{staticRule{name: "a"}}, Thresholds: Thresholds{Review: 90, Block: 10}}); err == nil {
		t.Error("NewService(review > block) = nil error, want error")
	}
	if _, err := NewService(ServiceDeps{Thresholds: Thresholds{Review: 50, Block: 80}}); err == nil {
		t.Error("NewService(no rules) = nil error, want error")
	}
}
