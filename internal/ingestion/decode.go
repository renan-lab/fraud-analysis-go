package ingestion

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// maxBodyBytes is the contract's body limit; larger bodies get 413.
const maxBodyBytes = 64 << 10

const contentTypeJSON = "application/json"

var (
	errUnsupportedMediaType = errors.New("unsupported media type")
	errBodyTooLarge         = errors.New("body too large")
)

// malformedBodyError means the body is not one JSON object of the expected
// shape. Field is set when the failure points at a known field.
type malformedBodyError struct {
	Field  string
	Detail string
}

func (e *malformedBodyError) Error() string {
	return fmt.Sprintf("malformed body: %s %s", e.Field, e.Detail)
}

// decodeTransaction checks headers and size, then strictly decodes the body:
// unknown fields, trailing data and type mismatches are rejected.
func decodeTransaction(w http.ResponseWriter, r *http.Request) (transactionRequest, error) {
	if !isJSON(r.Header.Get("Content-Type")) {
		return transactionRequest{}, fmt.Errorf("content type %q, want %s: %w", r.Header.Get("Content-Type"), contentTypeJSON, errUnsupportedMediaType)
	}
	if r.ContentLength > maxBodyBytes {
		return transactionRequest{}, fmt.Errorf("content length %d > %d: %w", r.ContentLength, maxBodyBytes, errBodyTooLarge)
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	var req transactionRequest
	if err := dec.Decode(&req); err != nil {
		return transactionRequest{}, classifyDecodeError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return transactionRequest{}, classifyTrailingData(err)
	}
	return req, nil
}

func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == contentTypeJSON
}

func classifyDecodeError(err error) error {
	var maxBytes *http.MaxBytesError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &maxBytes):
		return fmt.Errorf("body above %d bytes: %w", maxBytes.Limit, errBodyTooLarge)
	case errors.Is(err, io.EOF):
		return &malformedBodyError{Detail: "request body is empty"}
	case errors.As(err, &typeErr) && typeErr.Field != "":
		return &malformedBodyError{Field: typeErr.Field, Detail: fmt.Sprintf("must be %s, got JSON %s", jsonTypeName(typeErr), typeErr.Value)}
	default:
		return &malformedBodyError{Detail: err.Error()}
	}
}

func classifyTrailingData(err error) error {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return fmt.Errorf("body above %d bytes: %w", maxBytes.Limit, errBodyTooLarge)
	}
	return &malformedBodyError{Detail: "unexpected data after the JSON object"}
}

// jsonTypeName describes the Go target type in contract terms.
func jsonTypeName(e *json.UnmarshalTypeError) string {
	switch e.Type.String() {
	case "int64":
		return "an integer within int64 (minor units)"
	case "string":
		return "a string"
	case "float64":
		return "a number in decimal degrees"
	default:
		return "a JSON object"
	}
}
