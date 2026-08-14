package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
)

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ctxKey string

const requestIDKey ctxKey = "request-id"

const RequestIDHeader = "X-Request-ID"

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteErr(w http.ResponseWriter, status int, code, message string) {
	if code == "" {
		code = StatusToCode(status)
	}
	WriteJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: RequestIDFromWriter(w),
		},
	})
}

func RequestIDFromWriter(w http.ResponseWriter) string {
	if id := w.Header().Get(RequestIDHeader); id != "" {
		return id
	}
	return ""
}
