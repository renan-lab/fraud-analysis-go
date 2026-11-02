package simulator

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPercentileNearestRank(t *testing.T) {
	t.Parallel()
	sorted := make([]time.Duration, 100)
	for i := range sorted {
		sorted[i] = time.Duration(i+1) * time.Millisecond
	}
	tests := []struct {
		in   []time.Duration
		p    float64
		want time.Duration
	}{
		{in: sorted, p: 50, want: 50 * time.Millisecond},
		{in: sorted, p: 95, want: 95 * time.Millisecond},
		{in: sorted, p: 99, want: 99 * time.Millisecond},
		{in: sorted[:1], p: 99, want: time.Millisecond},
		{in: nil, p: 50, want: 0},
	}
	for _, tt := range tests {
		if got := percentile(tt.in, tt.p); got != tt.want {
			t.Errorf("percentile(len=%d, %v) = %v, want %v", len(tt.in), tt.p, got, tt.want)
		}
	}
}

func TestReportCountsStatusesErrorsAndViolations(t *testing.T) {
	t.Parallel()
	start := time.Now()
	r := NewReport(start)
	ok := Emission{Mode: ModeLegit, Scenario: "legit/purchase", ExpectedStatus: http.StatusAccepted}
	attack := Emission{Mode: ModeAttack, Scenario: "attack/xss", TransactionID: "t1", ExpectedStatus: http.StatusBadRequest}
	r.Observe(Outcome{Emission: ok, Result: SendResult{Status: 202, Latency: time.Millisecond}})
	r.Observe(Outcome{Emission: attack, Result: SendResult{Status: 202, Latency: 3 * time.Millisecond}})
	r.Observe(Outcome{Emission: ok, Err: errors.New("connection refused")})
	r.Observe(Outcome{Emission: ok})

	s := r.Summary(start.Add(2 * time.Second))
	if s.Total != 4 || s.NetworkErrors != 1 || s.ByStatus[202] != 2 || s.ByMode[ModeLegit] != 3 || s.TPS != 2 {
		t.Errorf("Summary() = %+v, want total 4, 1 network error, two 202, 3 legit, 2/s", s)
	}
	if s.Violations["attack/xss"] != 1 || len(s.Samples) != 1 || s.P99 != 3*time.Millisecond {
		t.Errorf("violations %v samples %v p99 %v, want attack/xss=1, one sample, 3ms", s.Violations, s.Samples, s.P99)
	}
}

func TestSummaryWriteText(t *testing.T) {
	t.Parallel()
	s := Summary{
		Total: 2, ByMode: map[Mode]int{ModeAttack: 2}, ByStatus: map[int]int{202: 2},
		Violations: map[string]int{"attack/xss": 2}, Samples: []string{"attack/xss got 202, want 400"},
	}
	var buf bytes.Buffer
	if err := s.WriteText(&buf); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	for _, want := range []string{"sent: 2", "attack=2", "202=2", "attack/xss: 2", "e.g. attack/xss"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("WriteText() = %q, want it to contain %q", buf.String(), want)
		}
	}
}
