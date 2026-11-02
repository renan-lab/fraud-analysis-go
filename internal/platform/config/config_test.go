package config

import (
	"strings"
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

func TestLoadWorkerAndAnalysisSettings(t *testing.T) {
	t.Setenv("WORKER_CONSUMER_GROUP", "")
	t.Setenv("ANALYSIS_VELOCITY_WINDOW", "90s")
	t.Setenv("ANALYSIS_CARD_TESTING_SMALL_AMOUNT", "500")
	t.Setenv("ANALYSIS_OUTLIER_RATIO", "7.5")
	t.Setenv("ANALYSIS_RISKY_MCCS", " 7995 ,,6051,4829 ")
	cfg := Load()
	if cfg.WorkerConsumerGroup != "analysis" {
		t.Errorf("WorkerConsumerGroup = %q, want default analysis", cfg.WorkerConsumerGroup)
	}
	a := cfg.Analysis
	if a.VelocityWindow != 90*time.Second || a.CardTestingSmallAmount != 500 || a.OutlierRatio != 7.5 {
		t.Errorf("Analysis = window %v small %d ratio %v, want 90s/500/7.5", a.VelocityWindow, a.CardTestingSmallAmount, a.OutlierRatio)
	}
	if a.ReviewThreshold != 50 || a.BlockThreshold != 80 {
		t.Errorf("thresholds = %d/%d, want defaults 50/80", a.ReviewThreshold, a.BlockThreshold)
	}
	if got := strings.Join(a.RiskyMCCs, ","); got != "7995,6051,4829" {
		t.Errorf("RiskyMCCs = %q, want 7995,6051,4829", got)
	}
}

func TestGetList(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{env: "", want: "a,b"},
		{env: "x", want: "x"},
		{env: " x , ,y ", want: "x,y"},
	}
	for _, tt := range tests {
		t.Setenv("CONFIG_TEST_LIST", tt.env)
		if got := strings.Join(getList("CONFIG_TEST_LIST", "a,b"), ","); got != tt.want {
			t.Errorf("getList with %q = %q, want %q", tt.env, got, tt.want)
		}
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
