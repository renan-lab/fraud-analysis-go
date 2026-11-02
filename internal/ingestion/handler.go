package ingestion

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/renan-lab/fraud-analysis-go/internal/platform/httpmw"
)

// Handler serves POST /transactions.
type Handler struct {
	service *Service
	clock   Clock
	logger  *slog.Logger
}

// NewHandler validates requests with clock (for the occurred_at skew check)
// and hands valid transactions to service.
//
//	mux.Handle("POST /transactions", ingestion.NewHandler(svc, ingestion.SystemClock{}, logger))
func NewHandler(service *Service, clock Clock, logger *slog.Logger) *Handler {
	return &Handler{service: service, clock: clock, logger: logger}
}

type acceptedResponse struct {
	TransactionID string `json:"transaction_id"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := decodeTransaction(w, r)
	if err != nil {
		writeDecodeProblem(w, r, err)
		return
	}
	tx, fieldErrs := req.toTransaction(h.clock.Now())
	if len(fieldErrs) > 0 {
		writeValidationProblem(w, r, fieldErrs)
		return
	}
	if err := h.service.Ingest(r.Context(), tx, httpmw.RequestIDFrom(r.Context())); err != nil {
		h.writeUnavailable(w, r, tx, err)
		return
	}
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(acceptedResponse{TransactionID: tx.TransactionID})
}

func (h *Handler) writeUnavailable(w http.ResponseWriter, r *http.Request, tx Transaction, err error) {
	h.logger.ErrorContext(r.Context(), "transaction not published", "error", err,
		"transaction_id", tx.TransactionID, "request_id", httpmw.RequestIDFrom(r.Context()))
	w.Header().Set("Retry-After", "1")
	httpmw.WriteProblem(w, r, httpmw.Problem{
		Type:   "/problems/publish-unavailable",
		Title:  "Transaction not accepted, retry later",
		Status: http.StatusServiceUnavailable,
		Detail: "the event broker did not acknowledge the transaction in time; retrying with the same transaction_id is safe",
	})
}

func writeDecodeProblem(w http.ResponseWriter, r *http.Request, err error) {
	var malformed *malformedBodyError
	switch {
	case errors.Is(err, errUnsupportedMediaType):
		httpmw.WriteProblem(w, r, httpmw.Problem{Type: "/problems/unsupported-media-type", Title: "Unsupported media type",
			Status: http.StatusUnsupportedMediaType, Detail: "Content-Type must be " + contentTypeJSON})
	case errors.Is(err, errBodyTooLarge):
		httpmw.WriteProblem(w, r, httpmw.Problem{Type: "/problems/body-too-large", Title: "Request body too large",
			Status: http.StatusRequestEntityTooLarge, Detail: "body must be at most 65536 bytes"})
	case errors.As(err, &malformed):
		writeMalformedProblem(w, r, malformed)
	default:
		writeMalformedProblem(w, r, &malformedBodyError{Detail: err.Error()})
	}
}

func writeMalformedProblem(w http.ResponseWriter, r *http.Request, e *malformedBodyError) {
	p := httpmw.Problem{Type: "/problems/malformed-body", Title: "Malformed request body", Status: http.StatusBadRequest, Detail: e.Detail}
	if e.Field != "" {
		p.Detail = ""
		p.Errors = []httpmw.FieldError{{Field: e.Field, Detail: e.Detail}}
	}
	httpmw.WriteProblem(w, r, p)
}

func writeValidationProblem(w http.ResponseWriter, r *http.Request, fieldErrs []FieldError) {
	errs := make([]httpmw.FieldError, len(fieldErrs))
	for i, fe := range fieldErrs {
		errs[i] = httpmw.FieldError{Field: fe.Field, Detail: fe.Detail}
	}
	httpmw.WriteProblem(w, r, httpmw.Problem{Type: "/problems/validation", Title: "Invalid transaction",
		Status: http.StatusBadRequest, Errors: errs})
}
