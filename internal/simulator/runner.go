package simulator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// minRampFactor keeps the first ticks of a ramp-up from waiting forever.
const minRampFactor = 0.05

// Sender delivers one emission and reports the HTTP status it got (0 when not sent over HTTP).
type Sender interface {
	Send(ctx context.Context, e Emission) (SendResult, error)
}

// EmissionSource yields the next batch of emissions; *Generator implements it.
type EmissionSource interface {
	Next() ([]Emission, error)
}

// Observer receives every send outcome. Observe is called from many workers.
type Observer interface {
	Observe(o Outcome)
}

// SendResult is what the target answered.
type SendResult struct {
	Status  int
	Latency time.Duration
}

// Outcome pairs an emission with the result of sending it.
type Outcome struct {
	Emission Emission
	Result   SendResult
	Err      error
}

// Burst fires Size extra emissions at once when the run reaches At.
type Burst struct {
	Size int
	At   time.Duration
}

// ParseBurst parses "size@offset", e.g. "500@30s". An empty spec means no burst.
//
//	burst, err := ParseBurst("500@30s")
func ParseBurst(spec string) (Burst, error) {
	if spec == "" {
		return Burst{}, nil
	}
	rawSize, rawAt, ok := strings.Cut(spec, "@")
	if !ok {
		return Burst{}, fmt.Errorf("burst %q: want size@offset, e.g. 500@30s", spec)
	}
	size, err := strconv.Atoi(rawSize)
	if err != nil || size < 1 {
		return Burst{}, fmt.Errorf("burst %q: size %q must be an integer >= 1", spec, rawSize)
	}
	at, err := time.ParseDuration(rawAt)
	if err != nil || at < 0 {
		return Burst{}, fmt.Errorf("burst %q: offset %q must be a duration >= 0, e.g. 30s", spec, rawAt)
	}
	return Burst{Size: size, At: at}, nil
}

// RunConfig controls pacing and stop conditions. The run stops at whichever of
// Count or Duration is reached first; zero disables that condition.
type RunConfig struct {
	Rate        float64
	Duration    time.Duration
	Count       int
	Concurrency int
	RampUp      time.Duration
	Burst       Burst
}

func (c RunConfig) validate() error {
	switch {
	case c.Rate <= 0:
		return fmt.Errorf("rate must be > 0 emissions/s, got %v", c.Rate)
	case c.Concurrency < 1:
		return fmt.Errorf("concurrency must be >= 1, got %d", c.Concurrency)
	case c.Count < 0 || c.Duration < 0 || c.RampUp < 0:
		return fmt.Errorf("count, duration and ramp-up must be >= 0, got %d, %v, %v", c.Count, c.Duration, c.RampUp)
	case c.Count == 0 && c.Duration == 0:
		return fmt.Errorf("set count or duration, got count=0 and duration=0")
	}
	return nil
}

// Runner paces emissions from a source into a pool of concurrent senders.
// Legs of one batch keep their causal order unless marked Parallel; different
// batches run concurrently.
type Runner struct {
	cfg       RunConfig
	source    EmissionSource
	sender    Sender
	observers []Observer
}

// NewRunner validates cfg and returns a Runner.
//
//	runner, err := NewRunner(RunConfig{Rate: 50, Count: 1000, Concurrency: 8}, gen, sender, report)
func NewRunner(cfg RunConfig, source EmissionSource, sender Sender, observers ...Observer) (*Runner, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Runner{cfg: cfg, source: source, sender: sender, observers: observers}, nil
}

// Run dispatches until a stop condition or ctx cancellation, then waits for
// in-flight sends. Sends use a context detached from ctx, so Ctrl+C stops new
// work without aborting requests already on the wire.
func (r *Runner) Run(ctx context.Context) error {
	if r.cfg.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.cfg.Duration)
		defer cancel()
	}
	jobs := make(chan job, r.cfg.Concurrency)
	var wg sync.WaitGroup
	sendCtx := context.WithoutCancel(ctx)
	for range r.cfg.Concurrency {
		wg.Go(func() { r.work(sendCtx, jobs) })
	}
	err := r.dispatch(ctx, jobs)
	close(jobs)
	wg.Wait()
	return err
}

func (r *Runner) work(ctx context.Context, jobs <-chan job) {
	for j := range jobs {
		r.send(ctx, j)
	}
}

// send waits for the earlier legs j depends on, then sends it. The signal is
// finished even when the send fails: a later purchase still happens in the
// real world after an earlier one was refused.
func (r *Runner) send(ctx context.Context, j job) {
	defer j.signal.finish()
	j.awaitDependencies()
	res, err := r.sender.Send(ctx, j.emission)
	for _, o := range r.observers {
		o.Observe(Outcome{Emission: j.emission, Result: res, Err: err})
	}
}

func (r *Runner) dispatch(ctx context.Context, jobs chan<- job) error {
	queue := &emissionQueue{source: r.source}
	start := time.Now()
	burstPending := r.cfg.Burst.Size > 0
	timer := time.NewTimer(0)
	defer timer.Stop()
	for dispatched := 0; r.cfg.Count == 0 || dispatched < r.cfg.Count; {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		elapsed := time.Since(start)
		batch := 1
		if burstPending && elapsed >= r.cfg.Burst.At {
			batch += r.cfg.Burst.Size
			burstPending = false
		}
		n, err := r.dispatchBatch(ctx, queue, jobs, batch, dispatched)
		if err != nil {
			return err
		}
		dispatched += n
		timer.Reset(r.interval(elapsed))
	}
	return nil
}

// dispatchBatch enqueues up to size emissions without exceeding Count.
func (r *Runner) dispatchBatch(ctx context.Context, q *emissionQueue, jobs chan<- job, size, dispatched int) (int, error) {
	n := 0
	for ; n < size && (r.cfg.Count == 0 || dispatched+n < r.cfg.Count); n++ {
		j, err := q.pop()
		if err != nil {
			return n, err
		}
		if !enqueue(ctx, jobs, j) {
			return n, nil
		}
	}
	return n, nil
}

// interval is the wait before the next tick, scaled down during ramp-up.
func (r *Runner) interval(elapsed time.Duration) time.Duration {
	factor := 1.0
	if r.cfg.RampUp > 0 && elapsed < r.cfg.RampUp {
		factor = max(float64(elapsed)/float64(r.cfg.RampUp), minRampFactor)
	}
	return time.Duration(float64(time.Second) / (r.cfg.Rate * factor))
}

// enqueue sends every copy of j to the workers, giving up if ctx is done.
// After a give-up no later leg of the batch is enqueued, so nobody waits on
// the signal of a leg whose copies were only partly sent.
func enqueue(ctx context.Context, jobs chan<- job, j job) bool {
	for range j.emission.copies() {
		select {
		case jobs <- j:
		case <-ctx.Done():
			return false
		}
	}
	return true
}
