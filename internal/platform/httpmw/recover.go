package httpmw

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover turns a handler panic into a 500 problem response and an error log.
// http.ErrAbortHandler is re-panicked so net/http can abort the connection.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				logger.ErrorContext(r.Context(), "handler panic", "panic", v,
					"request_id", RequestIDFrom(r.Context()), "stack", string(debug.Stack()))
				WriteProblem(w, r, Problem{Type: "/problems/internal", Title: "Internal server error", Status: http.StatusInternalServerError})
			}()
			next.ServeHTTP(w, r)
		})
	}
}
