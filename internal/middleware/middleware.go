package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/httpapi"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(httpapi.RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(httpapi.RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(httpapi.WithRequestID(r.Context(), id)))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack lets WebSocket upgrades pass through the logging middleware.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// Flush keeps streaming responses working through the logging middleware.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			logger.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote", r.RemoteAddr,
				"request_id", httpapi.RequestID(r.Context()),
			)
		})
	}
}

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic_recovered",
						"error", rec,
						"path", r.URL.Path,
						"request_id", httpapi.RequestID(r.Context()),
					)
					httpapi.WriteErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
