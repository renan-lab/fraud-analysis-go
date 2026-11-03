package kafka

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestNewConsumerValidatesConfig(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name    string
		cfg     ConsumerConfig
		wantErr bool
	}{
		{name: "valid", cfg: ConsumerConfig{Brokers: []string{"127.0.0.1:1"}, Group: "g", Topics: []string{"t"}}},
		{name: "blank group", cfg: ConsumerConfig{Brokers: []string{"127.0.0.1:1"}, Group: " ", Topics: []string{"t"}}, wantErr: true},
		{name: "no topics", cfg: ConsumerConfig{Brokers: []string{"127.0.0.1:1"}, Group: "g"}, wantErr: true},
		{name: "no brokers", cfg: ConsumerConfig{Group: "g", Topics: []string{"t"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := NewConsumer(tt.cfg, logger)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewConsumer(%+v) error = %v, wantErr %v", tt.cfg, err, tt.wantErr)
			}
			if c != nil {
				c.Close()
			}
		})
	}
}

func TestRunReturnsNilWhenContextIsCancelled(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := ConsumerConfig{Brokers: []string{"127.0.0.1:1"}, Group: "g", Topics: []string{"t"}}
	c, err := NewConsumer(cfg, logger)
	if err != nil {
		t.Fatalf("NewConsumer(%+v): %v", cfg, err)
	}
	t.Cleanup(c.Close)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	handle := func(context.Context, Record) error {
		t.Error("handler called with a cancelled context, want no records")
		return nil
	}
	if err := c.Run(ctx, handle); err != nil {
		t.Fatalf("Run(cancelled ctx) = %v, want nil", err)
	}
}

func TestToConsumedCopiesFieldsAndHeaders(t *testing.T) {
	t.Parallel()
	rec := toConsumed(&kgo.Record{
		Topic: "t", Partition: 2, Offset: 7, Key: []byte("k"), Value: []byte("v"),
		Headers: []kgo.RecordHeader{{Key: "event_type", Value: []byte("transaction.received")}},
	})
	if rec.Topic != "t" || rec.Partition != 2 || rec.Offset != 7 || string(rec.Key) != "k" || string(rec.Value) != "v" {
		t.Fatalf("toConsumed = %+v, want t/2@7 key k value v", rec)
	}
	if got := rec.HeaderValue("event_type"); got != "transaction.received" {
		t.Fatalf("HeaderValue(event_type) = %q, want transaction.received", got)
	}
	if got := rec.HeaderValue("missing"); got != "" {
		t.Fatalf("HeaderValue(missing) = %q, want empty", got)
	}
}
