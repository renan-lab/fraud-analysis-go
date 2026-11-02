package simulator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPSenderPostsBodyAndContentType(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath, gotType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotType, gotBody = r.Method, r.URL.Path, r.Header.Get("Content-Type"), string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)
	sender, err := NewHTTPSender(srv.URL, time.Second, 1)
	if err != nil {
		t.Fatalf("NewHTTPSender(%q) error = %v", srv.URL, err)
	}
	res, err := sender.Send(context.Background(), Emission{Body: []byte(`{"a":1}`), ContentType: "text/plain"})
	if err != nil || res.Status != http.StatusAccepted || res.Latency <= 0 {
		t.Fatalf("Send() = %+v, %v; want 202 with latency", res, err)
	}
	if gotMethod != http.MethodPost || gotPath != "/transactions" || gotType != "text/plain" || gotBody != `{"a":1}` {
		t.Errorf("server got %s %s type=%q body=%q, want POST /transactions text/plain {\"a\":1}", gotMethod, gotPath, gotType, gotBody)
	}
}

func TestHTTPSenderReportsNetworkError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	sender, err := NewHTTPSender(srv.URL, time.Second, 1)
	if err != nil {
		t.Fatalf("NewHTTPSender() error = %v", err)
	}
	if _, err := sender.Send(context.Background(), Emission{ContentType: contentTypeJSON}); err == nil {
		t.Error("Send() to closed server error = nil, want error")
	}
}

func TestNewHTTPSenderRejectsInvalidURL(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "localhost:8080", "ftp://host", "http://", "://bad"} {
		if _, err := NewHTTPSender(raw, time.Second, 1); err == nil {
			t.Errorf("NewHTTPSender(%q) error = nil, want error", raw)
		}
	}
}
