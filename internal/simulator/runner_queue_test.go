package simulator

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

// timelineSender sleeps delays[e.Scenario] per send and records when each
// emission started and finished.
type timelineSender struct {
	delays map[string]time.Duration
	mu     sync.Mutex
	starts map[string]time.Time
	ends   map[string]time.Time
}

func newTimelineSender(delays map[string]time.Duration) *timelineSender {
	return &timelineSender{delays: delays, starts: map[string]time.Time{}, ends: map[string]time.Time{}}
}

func (s *timelineSender) Send(_ context.Context, e Emission) (SendResult, error) {
	s.mark(s.starts, e.Scenario)
	time.Sleep(s.delays[e.Scenario])
	s.mark(s.ends, e.Scenario)
	return SendResult{Status: http.StatusAccepted}, nil
}

func (s *timelineSender) mark(m map[string]time.Time, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m[name] = time.Now()
}

// startedAfterEnd reports whether next started only after prev finished.
func (s *timelineSender) startedAfterEnd(next, prev string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.starts[next].Before(s.ends[prev])
}

// runBatchOnce sends one pass of batch with the given worker count and fails
// the test if the run does not finish within 2 s (a deadlock).
func runBatchOnce(t *testing.T, batch []Emission, concurrency int, sender Sender) {
	t.Helper()
	runner, err := NewRunner(RunConfig{Rate: 5000, Count: len(batch), Concurrency: concurrency}, staticSource{batch: batch}, sender)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- runner.Run(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run(%d legs, concurrency %d) still running after 2s, want no deadlock", len(batch), concurrency)
	}
}

func TestRunnerSendsSequenceInCausalOrder(t *testing.T) {
	t.Parallel()
	for _, concurrency := range []int{8, 1} {
		sender := newTimelineSender(map[string]time.Duration{"a": 30 * time.Millisecond, "b": 10 * time.Millisecond})
		runBatchOnce(t, []Emission{{Scenario: "a"}, {Scenario: "b"}, {Scenario: "c"}}, concurrency, sender)
		if !sender.startedAfterEnd("b", "a") || !sender.startedAfterEnd("c", "b") {
			t.Errorf("concurrency %d: starts %v ends %v, want a, b, c each after the previous finished", concurrency, sender.starts, sender.ends)
		}
	}
}

func TestRunnerSendsParallelLegsConcurrently(t *testing.T) {
	t.Parallel()
	sender := newTimelineSender(map[string]time.Duration{"a": 50 * time.Millisecond, "b": 50 * time.Millisecond})
	runBatchOnce(t, []Emission{{Scenario: "a", Parallel: true}, {Scenario: "b", Parallel: true}}, 8, sender)
	if sender.startedAfterEnd("b", "a") {
		t.Errorf("starts %v ends %v, want parallel legs a and b in flight together", sender.starts, sender.ends)
	}
}

func TestRunnerWaitsForParallelLegsBeforeSequentialLeg(t *testing.T) {
	t.Parallel()
	delays := map[string]time.Duration{"p1": 20 * time.Millisecond, "p2": 40 * time.Millisecond, "p3": 10 * time.Millisecond}
	sender := newTimelineSender(delays)
	batch := []Emission{{Scenario: "p1", Parallel: true}, {Scenario: "p2", Parallel: true}, {Scenario: "p3", Parallel: true}, {Scenario: "cashout"}}
	runBatchOnce(t, batch, 8, sender)
	for _, p := range []string{"p1", "p2", "p3"} {
		if !sender.startedAfterEnd("cashout", p) {
			t.Errorf("cashout started %v before %s finished %v, want it after every parallel leg", sender.starts["cashout"], p, sender.ends[p])
		}
	}
}

func TestEmissionQueuePopTracksDependencies(t *testing.T) {
	t.Parallel()
	batch := []Emission{{Scenario: "s1"}, {Scenario: "p1", Parallel: true}, {Scenario: "p2", Parallel: true}, {Scenario: "s2"}}
	q := &emissionQueue{source: staticSource{batch: batch}}
	jobs := popN(t, q, len(batch)+1)
	tests := []struct {
		name string
		got  []*legSignal
		want []*legSignal
	}{
		{name: "s1 waits for nothing", got: jobs[0].waitFor, want: nil},
		{name: "p1 waits for s1", got: jobs[1].waitFor, want: []*legSignal{jobs[0].signal}},
		{name: "p2 waits for s1 only", got: jobs[2].waitFor, want: []*legSignal{jobs[0].signal}},
		{name: "s2 waits for every earlier leg", got: jobs[3].waitFor, want: []*legSignal{jobs[0].signal, jobs[1].signal, jobs[2].signal}},
		{name: "next batch starts fresh", got: jobs[4].waitFor, want: nil},
	}
	for _, tt := range tests {
		if !sameSignals(tt.got, tt.want) {
			t.Errorf("%s: waitFor = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestLegSignalClosesAfterLastCopy(t *testing.T) {
	t.Parallel()
	s := newLegSignal(2)
	s.finish()
	select {
	case <-s.done:
		t.Fatal("signal closed after 1 of 2 copies, want open")
	default:
	}
	s.finish()
	select {
	case <-s.done:
	default:
		t.Fatal("signal open after 2 of 2 copies, want closed")
	}
}

func popN(t *testing.T, q *emissionQueue, n int) []job {
	t.Helper()
	jobs := make([]job, n)
	for i := range jobs {
		j, err := q.pop()
		if err != nil {
			t.Fatalf("pop() #%d error = %v", i, err)
		}
		jobs[i] = j
	}
	return jobs
}

func sameSignals(got, want []*legSignal) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
