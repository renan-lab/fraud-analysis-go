package simulator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// maxResponseDrain bounds how much of a response body is read to reuse the connection.
const maxResponseDrain = 1 << 20

// HTTPSender posts emissions to the ingestion API.
type HTTPSender struct {
	client   *http.Client
	endpoint string
}

// NewHTTPSender targets baseURL + "/transactions", keeping up to maxConns idle connections.
//
//	sender, err := NewHTTPSender("http://localhost:8080", 5*time.Second, 8)
func NewHTTPSender(baseURL string, timeout time.Duration, maxConns int) (*HTTPSender, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("target URL %q: want http(s)://host[:port]", baseURL)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = maxConns
	return &HTTPSender{
		client:   &http.Client{Timeout: timeout, Transport: transport},
		endpoint: u.JoinPath("transactions").String(),
	}, nil
}

// Send posts e.Body with e.ContentType and measures the round trip.
func (s *HTTPSender) Send(ctx context.Context, e Emission) (SendResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(e.Body))
	if err != nil {
		return SendResult{}, fmt.Errorf("build request to %s: %w", s.endpoint, err)
	}
	req.Header.Set("Content-Type", e.ContentType)
	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return SendResult{Latency: time.Since(start)}, fmt.Errorf("post %s: %w", s.endpoint, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseDrain))
	return SendResult{Status: resp.StatusCode, Latency: time.Since(start)}, nil
}
