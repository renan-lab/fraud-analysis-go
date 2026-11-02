package kafka

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestNewTopicReaderValidatesConfig(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name    string
		brokers []string
		topic   string
		wantErr bool
	}{
		{name: "valid", brokers: []string{"127.0.0.1:1"}, topic: "t"},
		{name: "empty topic", brokers: []string{"127.0.0.1:1"}, wantErr: true},
		{name: "no brokers", topic: "t", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, err := NewTopicReader(tt.brokers, tt.topic, logger)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewTopicReader(%v, %q) error = %v, wantErr %v", tt.brokers, tt.topic, err, tt.wantErr)
			}
			if r != nil {
				r.Close()
			}
		})
	}
}

func TestTopicReaderReadReturnsNilWhenContextDone(t *testing.T) {
	t.Parallel()
	r, err := NewTopicReader([]string{"127.0.0.1:1"}, "t", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewTopicReader: %v", err)
	}
	t.Cleanup(r.Close)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Read(ctx, func(context.Context, Record) error { return nil }); err != nil {
		t.Fatalf("Read(cancelled ctx) = %v, want nil", err)
	}
}
