package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// topicSetupTimeout bounds topic creation at startup, so a missing broker fails fast.
const topicSetupTimeout = 15 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("application stopped with error", "error", err)
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
	if err := ensureTopic(ctx, producer, cfg); err != nil {
		return err
	}
	return serve(ctx, newServer(cfg, newRouter(cfg, producer, logger)), cfg)
}

func ensureTopic(ctx context.Context, producer *kafka.Producer, cfg config.Config) error {
	ctx, cancel := context.WithTimeout(ctx, topicSetupTimeout)
	defer cancel()
	err := producer.EnsureTopic(ctx, events.TopicTransactionsReceived, cfg.KafkaTopicPartitions, cfg.KafkaTopicReplicationFactor)
	if err != nil {
		return fmt.Errorf("ensure topic on brokers %v: %w", cfg.KafkaBrokers, err)
	}
	slog.Info("kafka topic ready", "topic", events.TopicTransactionsReceived, "partitions", cfg.KafkaTopicPartitions)
	return nil
}

func newServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              net.JoinHostPort("", cfg.HTTPPort),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func serve(ctx context.Context, srv *http.Server, cfg config.Config) error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server starting", "addr", srv.Addr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	slog.Info("http server stopped")
	return nil
}

// closeProducer runs after the HTTP server drained, so no request is still publishing.
func closeProducer(producer *kafka.Producer, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := producer.Close(ctx); err != nil {
		slog.Error("close kafka producer", "error", err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
