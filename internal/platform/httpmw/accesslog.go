package httpmw

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog writes one structured line per request after it completes.
// Place it outside Recover so panics are logged with their 500 status.
func AccessLog(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := newStatusRecorder(w)
			next.ServeHTTP(rec, r)
			logger.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.Status(),
				"bytes", rec.bytes,
				"duration_ms", float64(time.Since(start).Microseconds())/1000,
				"remote_addr", r.RemoteAddr,
				"request_id", RequestIDFrom(r.Context()),
			)
		})
	}
}
