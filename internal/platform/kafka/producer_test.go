package kafka

import (
	"context"
	"slices"
	"testing"
)

func TestSeedBrokers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      []string
		want    []string
		wantErr bool
	}{
		{in: []string{"localhost:29092"}, want: []string{"localhost:29092"}},
		{in: []string{" a:9092 ", "", "b:9092"}, want: []string{"a:9092", "b:9092"}},
		{in: []string{""}, wantErr: true},
		{in: nil, wantErr: true},
	}
	for _, tt := range tests {
		got, err := seedBrokers(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("seedBrokers(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("seedBrokers(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToRecordCopiesHeaders(t *testing.T) {
	t.Parallel()
	rec := toRecord(Message{
		Topic:   "t",
		Key:     []byte("k"),
		Value:   []byte("v"),
		Headers: []Header{{Key: "event_type", Value: "transaction.received"}},
	})
	if rec.Topic != "t" || string(rec.Key) != "k" || string(rec.Value) != "v" {
		t.Fatalf("toRecord = topic %q key %q value %q, want t/k/v", rec.Topic, rec.Key, rec.Value)
	}
	if len(rec.Headers) != 1 || rec.Headers[0].Key != "event_type" || string(rec.Headers[0].Value) != "transaction.received" {
		t.Fatalf("toRecord headers = %+v, want event_type=transaction.received", rec.Headers)
	}
}

func TestEnsureTopicRejectsOutOfRangeSettings(t *testing.T) {
	t.Parallel()
	p, err := NewProducer([]string{"127.0.0.1:1"})
	if err != nil {
		t.Fatalf("NewProducer: %v", err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	tests := []struct{ partitions, rf int }{{0, 1}, {1, 0}, {1, 1 << 20}}
	for _, tt := range tests {
		if err := p.EnsureTopic(context.Background(), "t", tt.partitions, tt.rf); err == nil {
			t.Errorf("EnsureTopic(partitions=%d, rf=%d) = nil, want range error", tt.partitions, tt.rf)
		}
	}
}
