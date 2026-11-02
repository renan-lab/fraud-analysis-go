package httpmw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestChainOrderFirstIsOutermost(t *testing.T) {
	t.Parallel()
	var order []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	serve(t, Chain(statusHandler{http.StatusOK}, mark("a"), mark("b")), httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Join(order, ",") != "a,b" {
		t.Fatalf("Chain order = %v, want [a b]", order)
	}
}

func TestRequestID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		incoming string
		wantSame bool
	}{
		{name: "reuses well-formed id", incoming: "abc-123_X.y", wantSame: true},
		{name: "generates when missing", incoming: ""},
		{name: "replaces log injection", incoming: "a\nlevel=ERROR"},
		{name: "replaces oversized", incoming: strings.Repeat("a", 65)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var seen string
			h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(HeaderRequestID, tt.incoming)
			rec := serve(t, h, req)
			if got := rec.Header().Get(HeaderRequestID); got != seen {
				t.Fatalf("response header %q != context id %q", got, seen)
			}
			if tt.wantSame && seen != tt.incoming {
				t.Fatalf("request id = %q, want reused %q", seen, tt.incoming)
			}
			if !tt.wantSame && !uuidV4Pattern.MatchString(seen) {
				t.Fatalf("request id for %q = %q, want generated UUID v4", tt.incoming, seen)
			}
		})
	}
}

func TestRecoverWritesProblemAndLogs(t *testing.T) {
	t.Parallel()
	logger, buf := newBufferLogger()
	rec := serve(t, Recover(logger)(panicHandler{"boom"}), httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Status != 500 || p.Instance != "/x" {
		t.Fatalf("problem = %+v, want status 500 instance /x", p)
	}
	if !strings.Contains(buf.String(), `"panic":"boom"`) {
		t.Fatalf("log = %s, want panic value", buf.String())
	}
}

func TestRecoverRepanicsAbortHandler(t *testing.T) {
	t.Parallel()
	logger, _ := newBufferLogger()
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", v)
		}
	}()
	serve(t, Recover(logger)(panicHandler{http.ErrAbortHandler}), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestAccessLogRecordsStatus(t *testing.T) {
	t.Parallel()
	logger, buf := newBufferLogger()
	h := Chain(statusHandler{http.StatusTeapot}, RequestID, AccessLog(logger))
	serve(t, h, httptest.NewRequest(http.MethodPost, "/transactions", nil))
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("decode log %q: %v", buf.String(), err)
	}
	if line["status"] != float64(http.StatusTeapot) || line["path"] != "/transactions" || line["request_id"] == "" {
		t.Fatalf("log line = %v, want status 418, path /transactions and request_id", line)
	}
}

func TestMetricsInstrumentCountsByRouteAndStatus(t *testing.T) {
	t.Parallel()
	m := NewHTTPMetrics(prometheus.NewRegistry())
	h := m.Instrument("/transactions", statusHandler{http.StatusAccepted})
	for range 3 {
		serve(t, h, httptest.NewRequest(http.MethodPost, "/transactions?x=1", nil))
	}
	var got dto.Metric
	if err := m.requests.WithLabelValues("/transactions", "POST", "202").Write(&got); err != nil {
		t.Fatalf("read counter: %v", err)
	}
	if got := got.GetCounter().GetValue(); got != 3 {
		t.Fatalf("http_requests_total{route=/transactions,status=202} = %v, want 3", got)
	}
}

func TestRateLimit(t *testing.T) {
	t.Parallel()
	h := RateLimit(1, 2)(statusHandler{http.StatusAccepted})
	var codes []int
	for range 3 {
		codes = append(codes, serve(t, h, httptest.NewRequest(http.MethodPost, "/", nil)).Code)
	}
	if codes[0] != 202 || codes[1] != 202 || codes[2] != 429 {
		t.Fatalf("statuses = %v, want [202 202 429] with burst 2", codes)
	}
	rec := serve(t, h, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Header().Get("Retry-After") != "1" || decodeProblem(t, rec).Status != 429 {
		t.Fatalf("429 response: Retry-After %q body %s, want 1 and problem status 429", rec.Header().Get("Retry-After"), rec.Body)
	}
}

func TestRateLimitDisabled(t *testing.T) {
	t.Parallel()
	h := RateLimit(0, 1)(statusHandler{http.StatusAccepted})
	for i := range 50 {
		if code := serve(t, h, httptest.NewRequest(http.MethodPost, "/", nil)).Code; code != 202 {
			t.Fatalf("request %d with rps=0: status %d, want 202", i, code)
		}
	}
}

func TestWriteProblemIncludesRequestID(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/transactions", nil)
	req = req.WithContext(WithRequestID(req.Context(), "rid-1"))
	rec := httptest.NewRecorder()
	WriteProblem(rec, req, Problem{Type: "/problems/validation", Title: "Invalid", Status: 400,
		Errors: []FieldError{{Field: "amount", Detail: "must be > 0"}}})
	p := decodeProblem(t, rec)
	if rec.Code != 400 || p.RequestID != "rid-1" || p.Instance != "/transactions" || len(p.Errors) != 1 {
		t.Fatalf("problem = %+v (status %d), want 400 with request_id rid-1, instance and 1 field error", p, rec.Code)
	}
}
