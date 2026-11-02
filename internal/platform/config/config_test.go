package config

import (
	"testing"
	"time"
)

func TestLoadSimulatorTargetURL(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{env: "", want: "http://localhost:8080"},
		{env: "http://app:8080", want: "http://app:8080"},
	}
	for _, tt := range tests {
		t.Setenv("SIMULATOR_TARGET_URL", tt.env)
		if got := Load().SimulatorTargetURL; got != tt.want {
			t.Errorf("Load() with SIMULATOR_TARGET_URL=%q: SimulatorTargetURL = %q, want %q", tt.env, got, tt.want)
		}
	}
}

func TestLoadIngestSettings(t *testing.T) {
	t.Setenv("KAFKA_TOPIC_PARTITIONS", "3")
	t.Setenv("KAFKA_TOPIC_REPLICATION_FACTOR", "")
	t.Setenv("INGEST_PUBLISH_TIMEOUT", "2s")
	t.Setenv("INGEST_RATE_LIMIT_RPS", "12.5")
	t.Setenv("INGEST_RATE_LIMIT_BURST", "not-a-number")
	cfg := Load()
	if cfg.KafkaTopicPartitions != 3 {
		t.Errorf("KafkaTopicPartitions = %d, want 3", cfg.KafkaTopicPartitions)
	}
	if cfg.KafkaTopicReplicationFactor != 1 {
		t.Errorf("KafkaTopicReplicationFactor = %d, want default 1", cfg.KafkaTopicReplicationFactor)
	}
	if cfg.IngestPublishTimeout != 2*time.Second {
		t.Errorf("IngestPublishTimeout = %v, want 2s", cfg.IngestPublishTimeout)
	}
	if cfg.IngestRateLimitRPS != 12.5 {
		t.Errorf("IngestRateLimitRPS = %v, want 12.5", cfg.IngestRateLimitRPS)
	}
	if cfg.IngestRateLimitBurst != 100 {
		t.Errorf("IngestRateLimitBurst = %d for invalid value, want default 100", cfg.IngestRateLimitBurst)
	}
}

func TestGetIntFallsBack(t *testing.T) {
	tests := []struct {
		env  string
		want int
	}{
		{env: "", want: 7},
		{env: "abc", want: 7},
		{env: "42", want: 42},
	}
	for _, tt := range tests {
		t.Setenv("CONFIG_TEST_INT", tt.env)
		if got := getInt("CONFIG_TEST_INT", 7); got != tt.want {
			t.Errorf("getInt with %q = %d, want %d", tt.env, got, tt.want)
		}
	}
}
