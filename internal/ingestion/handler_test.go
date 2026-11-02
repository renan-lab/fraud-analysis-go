package ingestion

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func TestHandlerAcceptsValidTransaction(t *testing.T) {
	t.Parallel()
	pub := &fakePublisher{}
	rec := postTransaction(t, newTestHandler(pub), "application/json; charset=utf-8", mustJSON(t, validPayload()))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d body %s, want 202", rec.Code, rec.Body)
	}
	var body acceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.TransactionID != "7c9e6679-7425-40de-944b-e07fc1f90ae7" {
		t.Fatalf("body = %s (err %v), want transaction_id echoed", rec.Body, err)
	}
	evs := pub.published()
	if len(evs) != 1 || evs[0].RequestID == "" || evs[0].RequestID != rec.Header().Get("X-Request-ID") {
		t.Fatalf("published %+v, want 1 event carrying request id %q", evs, rec.Header().Get("X-Request-ID"))
	}
}

func TestHandlerPublishesLocation(t *testing.T) {
	t.Parallel()
	withoutLocation := validPayload()
	delete(withoutLocation, "location")
	tests := []struct {
		name    string
		payload map[string]any
		want    *events.Location
	}{
		{"sent", validPayload(), &events.Location{Lat: -23.5505, Lon: -46.6333}},
		{"absent", withoutLocation, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pub := &fakePublisher{}
			if rec := postTransaction(t, newTestHandler(pub), "application/json", mustJSON(t, tt.payload)); rec.Code != http.StatusAccepted {
				t.Fatalf("status = %d body %s, want 202", rec.Code, rec.Body)
			}
			evs := pub.published()
			if len(evs) != 1 || !reflect.DeepEqual(evs[0].Transaction.Location, tt.want) {
				t.Fatalf("published %+v, want 1 event with location %+v", evs, tt.want)
			}
		})
	}
}

func TestHandlerAcceptsDuplicates(t *testing.T) {
	t.Parallel()
	pub := &fakePublisher{}
	h := newTestHandler(pub)
	body := mustJSON(t, validPayload())
	for i := range 2 {
		if rec := postTransaction(t, h, "application/json", body); rec.Code != http.StatusAccepted {
			t.Fatalf("send %d: status = %d, want 202 (ADR 0002)", i, rec.Code)
		}
	}
	if n := len(pub.published()); n != 2 {
		t.Fatalf("published %d events, want 2: dedupe belongs to consumers", n)
	}
}

func TestHandlerRejects(t *testing.T) {
	t.Parallel()
	withField := func(key string, raw string) string {
		p := validPayload()
		delete(p, key)
		s := mustJSON(t, p)
		return s[:len(s)-1] + `,"` + key + `":` + raw + `}`
	}
	valid := mustJSON(t, validPayload())
	tests := []struct {
		name        string
		contentType string
		body        string
		status      int
		problemType string
	}{
		{"wrong content type", "text/plain", valid, 415, "/problems/unsupported-media-type"},
		{"missing content type", "", valid, 415, "/problems/unsupported-media-type"},
		{"oversized body", "application/json", withField("merchant", `{"id":"m","name":"`+strings.Repeat("A", 128<<10)+`","mcc":"5411","country":"BR"}`), 413, "/problems/body-too-large"},
		{"empty body", "application/json", "", 400, "/problems/malformed-body"},
		{"truncated json", "application/json", valid[:len(valid)/2], 400, "/problems/malformed-body"},
		{"trailing data", "application/json", valid + `{"x":1}`, 400, "/problems/malformed-body"},
		{"amount as string", "application/json", withField("amount", `"129.90"`), 400, "/problems/malformed-body"},
		{"amount as float", "application/json", withField("amount", `129.9`), 400, "/problems/malformed-body"},
		{"int overflow", "application/json", withField("amount", `99999999999999999999999`), 400, "/problems/malformed-body"},
		{"extra fields", "application/json", withField("is_admin", `true`), 400, "/problems/malformed-body"},
		{"deep nesting", "application/json", withField("merchant", strings.Repeat(`{"x":`, 5000)+"1"+strings.Repeat("}", 5000)), 400, "/problems/malformed-body"},
		{"null merchant", "application/json", withField("merchant", `null`), 400, "/problems/validation"},
		{"invalid currency", "application/json", withField("currency", `"REAL"`), 400, "/problems/validation"},
		{"location out of range", "application/json", withField("location", `{"lat":91,"lon":0}`), 400, "/problems/validation"},
		{"location as string", "application/json", withField("location", `"-23.55,-46.63"`), 400, "/problems/malformed-body"},
		{"location extra field", "application/json", withField("location", `{"lat":0,"lon":0,"alt":10}`), 400, "/problems/malformed-body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pub := &fakePublisher{}
			rec := postTransaction(t, newTestHandler(pub), tt.contentType, tt.body)
			if rec.Code != tt.status {
				t.Fatalf("status = %d body %.300s, want %d", rec.Code, rec.Body, tt.status)
			}
			if p := decodeProblem(t, rec); p.Type != tt.problemType || p.Status != tt.status || p.RequestID == "" {
				t.Fatalf("problem = %+v, want type %s status %d with request_id", p, tt.problemType, tt.status)
			}
			if n := len(pub.published()); n != 0 {
				t.Fatalf("published %d events for rejected request, want 0", n)
			}
		})
	}
}

func TestHandlerPointsTypeErrorsAtField(t *testing.T) {
	t.Parallel()
	payload := validPayload()
	payload["amount"] = 129.9
	p := decodeProblem(t, postTransaction(t, newTestHandler(&fakePublisher{}), "application/json", mustJSON(t, payload)))
	if len(p.Errors) != 1 || p.Errors[0].Field != "amount" {
		t.Fatalf("problem errors = %+v, want one error on amount", p.Errors)
	}
}

func TestHandlerListsAllValidationErrors(t *testing.T) {
	t.Parallel()
	p := validPayload()
	p["currency"] = "brl"
	p["channel"] = "telepathy"
	prob := decodeProblem(t, postTransaction(t, newTestHandler(&fakePublisher{}), "application/json", mustJSON(t, p)))
	if len(prob.Errors) != 2 || prob.Errors[0].Field != "currency" || prob.Errors[1].Field != "channel" {
		t.Fatalf("problem errors = %+v, want currency and channel", prob.Errors)
	}
}

func TestHandlerPublishFailureIs503(t *testing.T) {
	t.Parallel()
	rec := postTransaction(t, newTestHandler(&fakePublisher{err: errors.New("broker down")}), "application/json", mustJSON(t, validPayload()))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("status = %d Retry-After %q, want 503 and 1", rec.Code, rec.Header().Get("Retry-After"))
	}
	if p := decodeProblem(t, rec); p.Type != "/problems/publish-unavailable" {
		t.Fatalf("problem type = %q, want /problems/publish-unavailable", p.Type)
	}
}
