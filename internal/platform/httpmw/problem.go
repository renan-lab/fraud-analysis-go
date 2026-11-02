package httpmw

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ContentTypeProblem is the RFC 9457 media type for error bodies.
const ContentTypeProblem = "application/problem+json"

// FieldError points at one invalid request field.
type FieldError struct {
	Field  string `json:"field"`
	Detail string `json:"detail"`
}

// Problem is an RFC 9457 problem details body. Type is a relative URI that
// identifies the problem kind; Errors and RequestID are extension members.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Instance  string       `json:"instance,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// WriteProblem writes p as application/problem+json, filling Status in the
// body from the HTTP status and RequestID/Instance from r.
//
//	httpmw.WriteProblem(w, r, httpmw.Problem{Type: "/problems/validation", Title: "Invalid transaction", Status: 400})
func WriteProblem(w http.ResponseWriter, r *http.Request, p Problem) {
	p.RequestID = RequestIDFrom(r.Context())
	if p.Instance == "" {
		p.Instance = r.URL.Path
	}
	w.Header().Set("Content-Type", ContentTypeProblem)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(p.Status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.WarnContext(r.Context(), "write problem response", "error", err, "request_id", p.RequestID)
	}
}
