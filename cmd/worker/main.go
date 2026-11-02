// Command worker consumes transactions.received, scores each transaction with
// the analysis rules and publishes transactions.analyzed.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/analysis"
	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/ids"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/redis"
)

// startupTimeout bounds topic creation and the Redis ping, so missing infra fails fast.
const startupTimeout = 15 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	producer, err := kafka.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		return err
	}
	defer closeProducer(producer, cfg.ShutdownTimeout)
	if err := ensureTopics(ctx, producer, cfg); err != nil {
		return err
	}
	redisClient, err := connectRedis(ctx, cfg.RedisAddr)
	if err != nil {
		return err
	}
	defer closeRedis(redisClient)
	handler, err := newRecordHandler(cfg, producer, redisClient, logger)
	if err != nil {
		return err
	}
	return consume(ctx, cfg, handler, logger)
}

// ensureTopics creates both topics, so the worker can start before cmd/api.
func ensureTopics(ctx context.Context, producer *kafka.Producer, cfg config.Config) error {
	ctx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	for _, topic := range []string{events.TopicTransactionsReceived, events.TopicTransactionsAnalyzed} {
		if err := producer.EnsureTopic(ctx, topic, cfg.KafkaTopicPartitions, cfg.KafkaTopicReplicationFactor); err != nil {
			return fmt.Errorf("ensure topic on brokers %v: %w", cfg.KafkaBrokers, err)
		}
		slog.Info("kafka topic ready", "topic", topic, "partitions", cfg.KafkaTopicPartitions)
	}
	return nil
}

func connectRedis(ctx context.Context, addr string) (*redis.Client, error) {
	client, err := redis.NewClient(addr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis at %s: %w", addr, err)
	}
	return client, nil
}

func newRecordHandler(cfg config.Config, producer *kafka.Producer, rdb *redis.Client, logger *slog.Logger) (kafka.RecordHandler, error) {
	store, err := analysis.NewRedisFeatureStore(rdb, newStoreParams(cfg.Analysis))
	if err != nil {
		return nil, err
	}
	svc, err := analysis.NewService(analysis.ServiceDeps{
		Store:      store,
		Publisher:  analysis.NewKafkaPublisher(producer, events.TopicTransactionsAnalyzed),
		Rules:      newRules(cfg.Analysis),
		Thresholds: newThresholds(cfg.Analysis),
		Clock:      analysis.SystemClock{},
		NewID:      ids.NewV4,
	})
	if err != nil {
		return nil, err
	}
	consumer := analysis.NewConsumer(svc, logger, cfg.Analysis.ProcessTimeout, analysis.DefaultRetryPolicy)
	return consumer.HandleRecord, nil
}

// consume blocks until SIGINT/SIGTERM; the record in flight finishes and is
// committed before the consumer leaves the group.
func consume(ctx context.Context, cfg config.Config, handle kafka.RecordHandler, logger *slog.Logger) error {
	consumer, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers, Group: cfg.WorkerConsumerGroup, Topics: []string{events.TopicTransactionsReceived},
	}, logger)
	if err != nil {
		return err
	}
	defer consumer.Close()
	slog.Info("worker consuming", "topic", events.TopicTransactionsReceived, "group", cfg.WorkerConsumerGroup)
	if err := consumer.Run(ctx, handle); err != nil {
		return err
	}
	slog.Info("worker stopped")
	return nil
}

func closeProducer(producer *kafka.Producer, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := producer.Close(ctx); err != nil {
		slog.Error("close kafka producer", "error", err)
	}
}

func closeRedis(client *redis.Client) {
	if err := client.Close(); err != nil {
		slog.Error("close redis", "error", err)
	}
}
