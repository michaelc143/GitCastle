package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type contextKey int

const requestIDKey contextKey = 1

// RequestIDFrom returns the request ID stored by the RequestID middleware.
func RequestIDFrom(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// RequestID attaches a unique ID to every request (honoring an incoming
// X-Request-ID), exposes it in the response, and into the context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if len(id) < 8 || len(id) > 128 {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func newRequestID() string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(raw)
}

// statusRecorder captures the response status for access logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(code)
}

// AccessLog emits one structured line per request after completion.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			started := time.Now()
			next.ServeHTTP(recorder, r)
			if logger != nil {
				logger.Info("access",
					"method", r.Method,
					"path", r.URL.Path,
					"status", recorder.status,
					"duration_ms", time.Since(started).Milliseconds(),
					"request_id", RequestIDFrom(r.Context()),
				)
			}
		})
	}
}

// Recovery converts panics into JSON 500 responses with a correlation ID.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil && recovered != http.ErrAbortHandler {
					requestID := RequestIDFrom(r.Context())
					if logger != nil {
						logger.Error("panic recovered",
							"panic", recovered,
							"request_id", requestID,
							"method", r.Method,
							"path", r.URL.Path,
						)
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"internal server error","request_id":"` + requestID + `"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
