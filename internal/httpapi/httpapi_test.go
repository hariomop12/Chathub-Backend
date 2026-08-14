package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrMapsCodeAndRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set(RequestIDHeader, "req-123")

	WriteErr(rec, http.StatusBadRequest, "", "bad request")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error.Code != "INVALID_REQUEST" {
		t.Errorf("expected code INVALID_REQUEST, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "bad request" {
		t.Errorf("expected message 'bad request', got %q", resp.Error.Message)
	}
	if resp.Error.RequestID != "req-123" {
		t.Errorf("expected request_id req-123, got %q", resp.Error.RequestID)
	}
}

func TestWriteErrExplicitCodeWins(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteErr(rec, http.StatusForbidden, "FORBIDDEN", "no")

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Error.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN, got %q", resp.Error.Code)
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := WithRequestID(t.Context(), "abc")
	if got := RequestID(ctx); got != "abc" {
		t.Errorf("expected abc, got %q", got)
	}
}
