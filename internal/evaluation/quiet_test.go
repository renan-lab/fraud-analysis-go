package evaluation

import (
	"context"
	"testing"
	"time"
)

func TestQuietWatchQuiet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		complete bool
		ago      time.Duration
		want     bool
	}{
		{name: "incomplete never quiet", complete: false, ago: time.Hour, want: false},
		{name: "complete but recent record", complete: true, ago: 0, want: false},
		{name: "complete and settled", complete: true, ago: time.Second, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &quietWatch{}
			w.complete.Store(tt.complete)
			w.lastRecord.Store(time.Now().Add(-tt.ago).UnixNano())
			if got := w.quiet(500 * time.Millisecond); got != tt.want {
				t.Errorf("quiet(complete=%v, last record %v ago) = %v, want %v", tt.complete, tt.ago, got, tt.want)
			}
		})
	}
}

func TestQuietWatchRunCancelsOnceSettled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w := &quietWatch{}
	w.recordSeen(true)
	go w.run(ctx, 10*time.Millisecond, cancel)
	<-ctx.Done()
	if err := ctx.Err(); err != context.Canceled {
		t.Errorf("ctx.Err() = %v, want context.Canceled from the watcher, not the 5s timeout", err)
	}
}
