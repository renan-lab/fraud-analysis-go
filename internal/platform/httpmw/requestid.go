package httpmw

import (
	"context"
	"net/http"
	"regexp"

	"github.com/renan-lab/fraud-analysis-go/internal/platform/ids"
)

// HeaderRequestID carries the request ID in requests and responses.
const HeaderRequestID = "X-Request-ID"

// incomingRequestID bounds client-supplied IDs so they cannot inject log lines or huge values.
var incomingRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type requestIDKey struct{}

// RequestID reuses a well-formed X-Request-ID or generates a UUID v4, stores
// it in the request context and echoes it in the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !incomingRequestID.MatchString(id) {
			id = ids.NewV4()
		}
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}

// WithRequestID returns ctx carrying id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFrom returns the request ID stored by RequestID, or "" if absent.
//
//	slog.InfoContext(ctx, "published", "request_id", httpmw.RequestIDFrom(ctx))
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
