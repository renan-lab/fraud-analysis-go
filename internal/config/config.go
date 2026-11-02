// Package config loads application settings from environment variables.
package config

import (
	"os"
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
}

func Load() Config {
	return Config{
		AppEnv:          getEnv("APP_ENV", "development"),
		HTTPPort:        getEnv("HTTP_PORT", "8080"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://fraud:fraud@localhost:5432/fraud?sslmode=disable"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:    strings.Split(getEnv("KAFKA_BROKERS", "localhost:29092"), ","),
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
