package simulator

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

// recordingSender is a fake Sender that remembers what it was asked to send.
type recordingSender struct {
	mu   sync.Mutex
	sent []Emission
}

func (s *recordingSender) Send(_ context.Context, e Emission) (SendResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, e)
	return SendResult{Status: http.StatusAccepted}, nil
}

func (s *recordingSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

// staticSource is a fake EmissionSource that always returns the same batch.
type staticSource struct {
	batch []Emission
	err   error
}

func (s staticSource) Next() ([]Emission, error) { return s.batch, s.err }

var oneEmission = staticSource{batch: []Emission{{Scenario: "x", ExpectedStatus: http.StatusAccepted}}}

func runWith(t *testing.T, ctx context.Context, cfg RunConfig, src EmissionSource) (*recordingSender, error) {
	t.Helper()
	sender := &recordingSender{}
	runner, err := NewRunner(cfg, src, sender)
	if err != nil {
		t.Fatalf("NewRunner(%+v) error = %v", cfg, err)
	}
	return sender, runner.Run(ctx)
}

func TestRunnerStopsAtCount(t *testing.T) {
	t.Parallel()
	src := staticSource{batch: []Emission{{Scenario: "a"}, {Scenario: "b"}, {Scenario: "c"}}}
	sender, err := runWith(t, context.Background(), RunConfig{Rate: 5000, Count: 25, Concurrency: 4}, src)
	if err != nil || sender.count() != 25 {
		t.Errorf("Run(count=25) sent %d, err %v; want 25, nil", sender.count(), err)
	}
}

func TestRunnerSendsEveryCopy(t *testing.T) {
	t.Parallel()
	src := staticSource{batch: []Emission{{Scenario: "retry", Copies: 3}}}
	sender, err := runWith(t, context.Background(), RunConfig{Rate: 5000, Count: 2, Concurrency: 3}, src)
	if err != nil || sender.count() != 6 {
		t.Errorf("Run(count=2, copies=3) sent %d, err %v; want 6, nil", sender.count(), err)
	}
}

func TestRunnerStopsAtDuration(t *testing.T) {
	t.Parallel()
	start := time.Now()
	sender, err := runWith(t, context.Background(), RunConfig{Rate: 100, Duration: 100 * time.Millisecond, Concurrency: 2}, oneEmission)
	if elapsed := time.Since(start); err != nil || elapsed > time.Second || sender.count() == 0 || sender.count() > 20 {
		t.Errorf("Run(duration=100ms, rate=100) sent %d in %v, err %v; want 1-20 within 1s", sender.count(), elapsed, err)
	}
}

func TestRunnerStopsOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := runWith(t, ctx, RunConfig{Rate: 100, Duration: time.Hour, Concurrency: 2}, oneEmission)
	if elapsed := time.Since(start); err != nil || elapsed > time.Second {
		t.Errorf("Run() after cancel returned %v in %v, want nil quickly", err, elapsed)
	}
}

func TestRunnerFiresBurst(t *testing.T) {
	t.Parallel()
	cfg := RunConfig{Rate: 1, Duration: 200 * time.Millisecond, Concurrency: 4, Burst: Burst{Size: 20}}
	sender, err := runWith(t, context.Background(), cfg, oneEmission)
	if err != nil || sender.count() != 21 {
		t.Errorf("Run(rate=1, burst=20@0) sent %d, err %v; want 21 (first tick + burst)", sender.count(), err)
	}
}

func TestRunnerReturnsSourceError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := runWith(t, context.Background(), RunConfig{Rate: 100, Count: 5, Concurrency: 1}, staticSource{err: boom})
	if !errors.Is(err, boom) {
		t.Errorf("Run() error = %v, want wrapping %v", err, boom)
	}
}

func TestRunnerNotifiesObservers(t *testing.T) {
	t.Parallel()
	report := NewReport(time.Now())
	runner, err := NewRunner(RunConfig{Rate: 5000, Count: 10, Concurrency: 2}, oneEmission, &recordingSender{}, report)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := report.Summary(time.Now()).ByStatus[http.StatusAccepted]; got != 10 {
		t.Errorf("report 202 count = %d, want 10", got)
	}
}

func TestRunConfigValidate(t *testing.T) {
	t.Parallel()
	valid := RunConfig{Rate: 1, Count: 1, Concurrency: 1}
	tests := []struct {
		name    string
		mutate  func(*RunConfig)
		wantErr bool
	}{
		{name: "valid", mutate: func(*RunConfig) {}},
		{name: "duration only", mutate: func(c *RunConfig) { c.Count, c.Duration = 0, time.Second }},
		{name: "zero rate", mutate: func(c *RunConfig) { c.Rate = 0 }, wantErr: true},
		{name: "zero concurrency", mutate: func(c *RunConfig) { c.Concurrency = 0 }, wantErr: true},
		{name: "negative count", mutate: func(c *RunConfig) { c.Count = -1 }, wantErr: true},
		{name: "negative ramp-up", mutate: func(c *RunConfig) { c.RampUp = -time.Second }, wantErr: true},
		{name: "no stop condition", mutate: func(c *RunConfig) { c.Count = 0 }, wantErr: true},
	}
	for _, tt := range tests {
		cfg := valid
		tt.mutate(&cfg)
		if err := cfg.validate(); (err != nil) != tt.wantErr {
			t.Errorf("%s: validate(%+v) error = %v, wantErr %v", tt.name, cfg, err, tt.wantErr)
		}
	}
}

func TestRunnerIntervalRampsUp(t *testing.T) {
	t.Parallel()
	r := &Runner{cfg: RunConfig{Rate: 10, RampUp: 10 * time.Second}}
	tests := []struct {
		elapsed time.Duration
		want    time.Duration
	}{
		{elapsed: 0, want: 2 * time.Second}, // floor at 5% of the rate
		{elapsed: 5 * time.Second, want: 200 * time.Millisecond},
		{elapsed: 20 * time.Second, want: 100 * time.Millisecond},
	}
	for _, tt := range tests {
		if got := r.interval(tt.elapsed); got != tt.want {
			t.Errorf("interval(%v) = %v, want %v", tt.elapsed, got, tt.want)
		}
	}
}

func TestParseBurst(t *testing.T) {
	t.Parallel()
	tests := []struct {
		spec    string
		want    Burst
		wantErr bool
	}{
		{spec: "", want: Burst{}},
		{spec: "500@30s", want: Burst{Size: 500, At: 30 * time.Second}},
		{spec: "1@0s", want: Burst{Size: 1}},
		{spec: "500", wantErr: true},
		{spec: "0@1s", wantErr: true},
		{spec: "x@1s", wantErr: true},
		{spec: "5@soon", wantErr: true},
		{spec: "5@-1s", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseBurst(tt.spec)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseBurst(%q) = %+v, %v; want %+v, wantErr %v", tt.spec, got, err, tt.want, tt.wantErr)
		}
	}
}
