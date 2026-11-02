package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/ingestion"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/httpmw"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/ids"
)

// readyTimeout bounds the broker ping behind GET /ready.
const readyTimeout = 2 * time.Second

// broker is what the API needs from Kafka: publish and a health ping.
type broker interface {
	ingestion.MessageProducer
	Ping(ctx context.Context) error
}

// newRouter wires every route. Middleware order: RequestID first so the
// access log and problem bodies carry the ID; AccessLog outside Recover so
// panics are logged with their 500.
func newRouter(cfg config.Config, b broker, logger *slog.Logger) http.Handler {
	reg := newRegistry()
	metrics := httpmw.NewHTTPMetrics(reg)

	mux := http.NewServeMux()
	mux.Handle("POST /transactions", metrics.Instrument("/transactions", newIngestHandler(cfg, b, reg, logger)))
	mux.Handle("GET /health", metrics.Instrument("/health", http.HandlerFunc(healthHandler)))
	mux.Handle("GET /ready", metrics.Instrument("/ready", readyHandler(b)))
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return httpmw.Chain(mux, httpmw.RequestID, httpmw.AccessLog(logger), httpmw.Recover(logger))
}

func newRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return reg
}

func newIngestHandler(cfg config.Config, b broker, reg prometheus.Registerer, logger *slog.Logger) http.Handler {
	publisher := ingestion.NewKafkaPublisher(b, events.TopicTransactionsReceived, reg)
	service := ingestion.NewService(publisher, ingestion.SystemClock{}, ids.NewV4, cfg.IngestPublishTimeout)
	handler := ingestion.NewHandler(service, ingestion.SystemClock{}, logger)
	return httpmw.RateLimit(cfg.IngestRateLimitRPS, cfg.IngestRateLimitBurst)(handler)
}

// readyHandler answers 200 only while Kafka is reachable; POST /transactions
// cannot succeed without it, so load balancers should stop routing on 503.
func readyHandler(b broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
		defer cancel()
		if err := b.Ping(ctx); err != nil {
			slog.WarnContext(r.Context(), "not ready", "error", err, "request_id", httpmw.RequestIDFrom(r.Context()))
			httpmw.WriteProblem(w, r, httpmw.Problem{Type: "/problems/not-ready", Title: "Kafka unreachable", Status: http.StatusServiceUnavailable})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}
