package evaluation

import (
	"context"
	"sync/atomic"
	"time"
)

// minQuietTick keeps the watcher from spinning when settle is tiny (tests).
const minQuietTick = time.Millisecond

// quietWatch ends a read once every expected decision arrived and the topic
// then stayed quiet for a settle period. Stopping at the first moment the set
// is complete would miss redeliveries published right after, which is exactly
// what the idempotency check needs to see. The handler writes, the watcher
// goroutine reads, hence the atomics.
type quietWatch struct {
	complete   atomic.Bool
	lastRecord atomic.Int64
}

// recordSeen is called by the handler for every record, decodable or not.
func (w *quietWatch) recordSeen(complete bool) {
	w.lastRecord.Store(time.Now().UnixNano())
	if complete {
		w.complete.Store(true)
	}
}

func (w *quietWatch) quiet(settle time.Duration) bool {
	return w.complete.Load() && time.Since(time.Unix(0, w.lastRecord.Load())) >= settle
}

// run calls cancel when quiet; it returns when ctx is done.
func (w *quietWatch) run(ctx context.Context, settle time.Duration, cancel context.CancelFunc) {
	ticker := time.NewTicker(max(settle/4, minQuietTick))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.quiet(settle) {
				cancel()
				return
			}
		}
	}
}
