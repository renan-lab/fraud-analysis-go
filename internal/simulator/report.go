package simulator

import (
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxViolationSamples = 5

// Report aggregates outcomes from concurrent workers.
type Report struct {
	mu            sync.Mutex
	start         time.Time
	total         int
	networkErrors int
	byMode        map[Mode]int
	byStatus      map[int]int
	latencies     []time.Duration
	violations    map[string]int
	samples       []string
}

// NewReport starts measuring throughput at start.
//
//	report := NewReport(time.Now())
func NewReport(start time.Time) *Report {
	return &Report{start: start, byMode: map[Mode]int{}, byStatus: map[int]int{}, violations: map[string]int{}}
}

// Observe counts o. Status 0 means the sender did not use HTTP (dry run), so
// latency and expectations are not checked.
func (r *Report) Observe(o Outcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.total++
	r.byMode[o.Emission.Mode]++
	if o.Err != nil {
		r.networkErrors++
		return
	}
	if o.Result.Status == 0 {
		return
	}
	r.byStatus[o.Result.Status]++
	r.latencies = append(r.latencies, o.Result.Latency)
	if o.Result.Status != o.Emission.ExpectedStatus {
		r.recordViolation(o)
	}
}

func (r *Report) recordViolation(o Outcome) {
	r.violations[o.Emission.Scenario]++
	if len(r.samples) < maxViolationSamples {
		r.samples = append(r.samples, fmt.Sprintf("%s transaction_id=%q got %d, want %d",
			o.Emission.Scenario, o.Emission.TransactionID, o.Result.Status, o.Emission.ExpectedStatus))
	}
}

// Summary is an immutable snapshot of a Report.
type Summary struct {
	Total         int
	NetworkErrors int
	Elapsed       time.Duration
	TPS           float64
	ByMode        map[Mode]int
	ByStatus      map[int]int
	P50, P95, P99 time.Duration
	Violations    map[string]int
	Samples       []string
}

// Summary snapshots the report as of end.
func (r *Report) Summary(end time.Time) Summary {
	r.mu.Lock()
	defer r.mu.Unlock()
	sorted := slices.Clone(r.latencies)
	slices.Sort(sorted)
	elapsed := end.Sub(r.start)
	s := Summary{
		Total: r.total, NetworkErrors: r.networkErrors, Elapsed: elapsed,
		ByMode: maps.Clone(r.byMode), ByStatus: maps.Clone(r.byStatus),
		P50: percentile(sorted, 50), P95: percentile(sorted, 95), P99: percentile(sorted, 99),
		Violations: maps.Clone(r.violations), Samples: slices.Clone(r.samples),
	}
	if elapsed > 0 {
		s.TPS = float64(r.total) / elapsed.Seconds()
	}
	return s
}

// percentile uses the nearest-rank method on an ascending slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(float64(len(sorted))*p/100)) - 1
	return sorted[min(max(rank, 0), len(sorted)-1)]
}

// WriteText prints the summary for humans.
func (s Summary) WriteText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "sent: %d in %s (%.1f/s), network errors: %d\n", s.Total, s.Elapsed.Round(time.Millisecond), s.TPS, s.NetworkErrors)
	b.WriteString("by mode:")
	for _, m := range slices.Sorted(maps.Keys(s.ByMode)) {
		fmt.Fprintf(&b, " %s=%d", m, s.ByMode[m])
	}
	b.WriteString("\nby status:")
	if len(s.ByStatus) == 0 {
		b.WriteString(" none (dry run or all requests failed)")
	}
	for _, code := range slices.Sorted(maps.Keys(s.ByStatus)) {
		fmt.Fprintf(&b, " %d=%d", code, s.ByStatus[code])
	}
	fmt.Fprintf(&b, "\nlatency: p50=%s p95=%s p99=%s\n", s.P50, s.P95, s.P99)
	writeViolations(&b, s)
	_, err := io.WriteString(w, b.String())
	return err
}

func writeViolations(b *strings.Builder, s Summary) {
	if len(s.Violations) == 0 {
		b.WriteString("expectation violations: none\n")
		return
	}
	b.WriteString("expectation violations:\n")
	for _, name := range slices.Sorted(maps.Keys(s.Violations)) {
		fmt.Fprintf(b, "  %s: %d\n", name, s.Violations[name])
	}
	for _, sample := range s.Samples {
		fmt.Fprintf(b, "  e.g. %s\n", sample)
	}
}
