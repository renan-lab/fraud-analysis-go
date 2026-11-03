package simulator

import (
	"fmt"
	"slices"
	"sync/atomic"
)

// legSignal is closed once every copy of one emission got a response.
type legSignal struct {
	remaining atomic.Int32
	done      chan struct{}
}

func newLegSignal(copies int) *legSignal {
	s := &legSignal{done: make(chan struct{})}
	s.remaining.Store(int32(copies))
	return s
}

// finish marks one copy as answered; the last one releases the waiters.
func (s *legSignal) finish() {
	if s.remaining.Add(-1) == 0 {
		close(s.done)
	}
}

// job is one copy of an emission plus the earlier legs of its batch it must
// wait for. waitFor never contains a later job: jobs are queued FIFO, so every
// dependency was taken by a worker before the job that waits on it, and the
// pool cannot deadlock, even with a single worker.
type job struct {
	emission Emission
	waitFor  []*legSignal
	signal   *legSignal
}

func (j job) awaitDependencies() {
	for _, s := range j.waitFor {
		<-s.done
	}
}

// emissionQueue buffers multi-emission scenarios so they are paced one per
// tick, and tracks the order dependencies inside the current batch.
type emissionQueue struct {
	source  EmissionSource
	pending []Emission
	// issued holds the signal of every leg of the current batch popped so far;
	// barrier is the last sequential leg plus everything before it, which the
	// parallel legs that follow it wait for.
	issued  []*legSignal
	barrier []*legSignal
}

// pop returns the next job. A sequential leg waits for every earlier leg of
// its batch; a parallel leg waits only for the last sequential leg (and what
// that one waited for), so consecutive parallel legs run together.
func (q *emissionQueue) pop() (job, error) {
	if err := q.refill(); err != nil {
		return job{}, err
	}
	e := q.pending[0]
	q.pending = q.pending[1:]
	j := job{emission: e, waitFor: q.barrier, signal: newLegSignal(e.copies())}
	if !e.Parallel {
		j.waitFor = slices.Clone(q.issued)
	}
	q.issued = append(q.issued, j.signal)
	if !e.Parallel {
		q.barrier = slices.Clone(q.issued)
	}
	return j, nil
}

// refill loads the next batch once the current one is exhausted; batches
// never wait for each other.
func (q *emissionQueue) refill() error {
	for len(q.pending) == 0 {
		batch, err := q.source.Next()
		if err != nil {
			return fmt.Errorf("generate emissions: %w", err)
		}
		q.pending, q.issued, q.barrier = batch, nil, nil
	}
	return nil
}
