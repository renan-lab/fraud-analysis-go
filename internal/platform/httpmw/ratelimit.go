package httpmw

import (
	"math"
	"net/http"
	"strconv"

	"golang.org/x/time/rate"
)

// RateLimit rejects requests above rps (with burst) with 429 and Retry-After.
// The limit is global to the process, not per client. rps <= 0 disables it.
//
//	limited := httpmw.RateLimit(500, 100)(handler)
func RateLimit(rps float64, burst int) Middleware {
	if rps <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	limiter := rate.NewLimiter(rate.Limit(rps), max(burst, 1))
	retryAfter := strconv.Itoa(int(math.Ceil(max(1/rps, 1))))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter.Allow() {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Retry-After", retryAfter)
			WriteProblem(w, r, Problem{
				Type:   "/problems/rate-limited",
				Title:  "Too many requests",
				Status: http.StatusTooManyRequests,
				Detail: "request rate above " + strconv.FormatFloat(rps, 'f', -1, 64) + " per second",
			})
		})
	}
}
