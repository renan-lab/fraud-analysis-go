package redis

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestNewClientRejectsBlankAddr(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"", "  "} {
		if _, err := NewClient(addr); err == nil {
			t.Errorf("NewClient(%q) = nil error, want error", addr)
		}
	}
}

func TestFormatScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   float64
		want string
	}{
		{in: math.Inf(-1), want: "-inf"},
		{in: math.Inf(1), want: "+inf"},
		{in: 1762110900000, want: "1762110900000"},
		{in: 1.5, want: "1.5"},
	}
	for _, tt := range tests {
		if got := formatScore(tt.in); got != tt.want {
			t.Errorf("formatScore(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBatchQueuesEveryCommand(t *testing.T) {
	t.Parallel()
	b := NewBatch()
	b.ZAdd("k", 1, "m")
	b.ZRemRangeByScore("k", math.Inf(-1), 0)
	b.ZKeepHighest("k", 10)
	b.HSetNX("h", "f", "v")
	b.Expire("k", time.Hour)
	if got := b.Len(); got != 5 {
		t.Fatalf("Batch.Len() = %d after 5 commands, want 5", got)
	}
}

// TestExecEmptyBatchSkipsServer uses an unreachable address: an empty batch
// must return before dialing.
func TestExecEmptyBatchSkipsServer(t *testing.T) {
	t.Parallel()
	c, err := NewClient("127.0.0.1:1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Exec(context.Background(), NewBatch()); err != nil {
		t.Fatalf("Exec(empty batch) = %v, want nil", err)
	}
}
