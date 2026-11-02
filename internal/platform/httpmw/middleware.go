// Package httpmw holds net/http middlewares shared by HTTP binaries
// (request ID, access log, panic recovery, metrics, rate limit) and the
// RFC 9457 problem+json error writer.
package httpmw

import "net/http"

// Middleware wraps a handler with cross-cutting behavior.
type Middleware func(http.Handler) http.Handler

// Chain applies mws so the first one is the outermost.
//
//	h := httpmw.Chain(mux, httpmw.RequestID, httpmw.AccessLog(logger))
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder captures the status code and body size written by the next handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	if rec, ok := w.(*statusRecorder); ok {
		return rec
	}
	return &statusRecorder{ResponseWriter: w}
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Status returns the written status, or 200 if the handler wrote nothing.
func (r *statusRecorder) Status() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}
