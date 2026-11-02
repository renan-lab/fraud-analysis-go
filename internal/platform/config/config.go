// Package config loads application settings from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv          string
	HTTPPort        string
	ShutdownTimeout time.Duration
	DatabaseURL     string
	RedisAddr       string
	KafkaBrokers    []string
	// KafkaTopicPartitions and KafkaTopicReplicationFactor are used when
	// cmd/api creates transactions.received at startup.
	KafkaTopicPartitions        int
	KafkaTopicReplicationFactor int
	// IngestPublishTimeout bounds the synchronous Kafka publish behind POST /transactions.
	IngestPublishTimeout time.Duration
	// IngestRateLimitRPS is the global request rate for POST /transactions; 0 disables the limit.
	IngestRateLimitRPS   float64
	IngestRateLimitBurst int
	// SimulatorTargetURL is the ingestion API base URL used by cmd/simulator.
	SimulatorTargetURL string
}

func Load() Config {
	return Config{
		AppEnv:                      getEnv("APP_ENV", "development"),
		HTTPPort:                    getEnv("HTTP_PORT", "8080"),
		ShutdownTimeout:             getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		DatabaseURL:                 getEnv("DATABASE_URL", "postgres://fraud:fraud@localhost:5432/fraud?sslmode=disable"),
		RedisAddr:                   getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:                strings.Split(getEnv("KAFKA_BROKERS", "localhost:29092"), ","),
		KafkaTopicPartitions:        getInt("KAFKA_TOPIC_PARTITIONS", 6),
		KafkaTopicReplicationFactor: getInt("KAFKA_TOPIC_REPLICATION_FACTOR", 1),
		IngestPublishTimeout:        getDuration("INGEST_PUBLISH_TIMEOUT", 5*time.Second),
		IngestRateLimitRPS:          getFloat("INGEST_RATE_LIMIT_RPS", 0),
		IngestRateLimitBurst:        getInt("INGEST_RATE_LIMIT_BURST", 100),
		SimulatorTargetURL:          getEnv("SIMULATOR_TARGET_URL", "http://localhost:8080"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(getEnv(key, ""))
	if err != nil {
		return fallback
	}
	return d
}

func getInt(key string, fallback int) int {
	n, err := strconv.Atoi(getEnv(key, ""))
	if err != nil {
		return fallback
	}
	return n
}

func getFloat(key string, fallback float64) float64 {
	f, err := strconv.ParseFloat(getEnv(key, ""), 64)
	if err != nil {
		return fallback
	}
	return f
}
