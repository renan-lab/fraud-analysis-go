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
	// WorkerConsumerGroup is the Kafka consumer group of cmd/worker's analysis consumer.
	WorkerConsumerGroup string
	Analysis            AnalysisConfig
}

// AnalysisConfig holds the ANALYSIS_* settings of cmd/worker: state
// retention, rule limits and scores, and decision thresholds (ADR 0010).
type AnalysisConfig struct {
	// ProcessTimeout bounds one attempt at analyzing a message (Redis + publish).
	ProcessTimeout  time.Duration
	ReviewThreshold int
	BlockThreshold  int
	// HistoryRetention is how far back card history is kept and read.
	HistoryRetention time.Duration
	// BaselineSize is how many recent amounts per account feed the median.
	BaselineSize int
	// ProfileTTL expires known devices and countries of idle accounts.
	ProfileTTL time.Duration

	VelocityWindow time.Duration
	VelocityMax    int
	VelocityScore  int

	CardTestingWindow time.Duration
	// CardTestingSmallAmount is the largest amount, in minor units, counted as a test charge.
	CardTestingSmallAmount int64
	CardTestingMinCount    int
	CardTestingScore       int
	// CardTestingCashoutScore is added when the current charge is not small.
	CardTestingCashoutScore int

	TravelMaxKMH float64
	TravelMinKM  float64
	TravelScore  int

	OutlierRatio      float64
	OutlierMinSamples int
	OutlierScore      int

	RiskyMCCs       []string
	NewDeviceScore  int
	NewCountryScore int
	RiskyMCCScore   int
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
		WorkerConsumerGroup:         getEnv("WORKER_CONSUMER_GROUP", "analysis"),
		Analysis:                    loadAnalysis(),
	}
}

func loadAnalysis() AnalysisConfig {
	a := AnalysisConfig{
		ProcessTimeout:   getDuration("ANALYSIS_PROCESS_TIMEOUT", 10*time.Second),
		ReviewThreshold:  getInt("ANALYSIS_REVIEW_THRESHOLD", 50),
		BlockThreshold:   getInt("ANALYSIS_BLOCK_THRESHOLD", 80),
		HistoryRetention: getDuration("ANALYSIS_HISTORY_RETENTION", 24*time.Hour),
		BaselineSize:     getInt("ANALYSIS_BASELINE_SIZE", 50),
		ProfileTTL:       getDuration("ANALYSIS_PROFILE_TTL", 90*24*time.Hour),
	}
	loadVelocityAndCardTesting(&a)
	loadTravelAndOutlier(&a)
	loadDeviceAndCountry(&a)
	return a
}

func loadVelocityAndCardTesting(a *AnalysisConfig) {
	a.VelocityWindow = getDuration("ANALYSIS_VELOCITY_WINDOW", 3*time.Minute)
	a.VelocityMax = getInt("ANALYSIS_VELOCITY_MAX", 6)
	a.VelocityScore = getInt("ANALYSIS_VELOCITY_SCORE", 80)
	a.CardTestingWindow = getDuration("ANALYSIS_CARD_TESTING_WINDOW", 10*time.Minute)
	a.CardTestingSmallAmount = int64(getInt("ANALYSIS_CARD_TESTING_SMALL_AMOUNT", 1000))
	a.CardTestingMinCount = getInt("ANALYSIS_CARD_TESTING_MIN_COUNT", 4)
	a.CardTestingScore = getInt("ANALYSIS_CARD_TESTING_SCORE", 60)
	a.CardTestingCashoutScore = getInt("ANALYSIS_CARD_TESTING_CASHOUT_SCORE", 30)
}

func loadTravelAndOutlier(a *AnalysisConfig) {
	a.TravelMaxKMH = getFloat("ANALYSIS_TRAVEL_MAX_KMH", 900)
	a.TravelMinKM = getFloat("ANALYSIS_TRAVEL_MIN_KM", 500)
	a.TravelScore = getInt("ANALYSIS_TRAVEL_SCORE", 80)
	a.OutlierRatio = getFloat("ANALYSIS_OUTLIER_RATIO", 10)
	a.OutlierMinSamples = getInt("ANALYSIS_OUTLIER_MIN_SAMPLES", 5)
	a.OutlierScore = getInt("ANALYSIS_OUTLIER_SCORE", 60)
}

func loadDeviceAndCountry(a *AnalysisConfig) {
	a.RiskyMCCs = getList("ANALYSIS_RISKY_MCCS", "7995,6051")
	a.NewDeviceScore = getInt("ANALYSIS_NEW_DEVICE_SCORE", 20)
	a.NewCountryScore = getInt("ANALYSIS_NEW_COUNTRY_SCORE", 25)
	a.RiskyMCCScore = getInt("ANALYSIS_RISKY_MCC_SCORE", 25)
}

// getList splits a comma-separated value, dropping blanks around items.
func getList(key, fallback string) []string {
	var out []string
	for _, item := range strings.Split(getEnv(key, fallback), ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
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
