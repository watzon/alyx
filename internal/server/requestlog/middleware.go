package requestlog

import (
	"net/http"
	"strings"
	"time"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/requestctx"
)

// Middleware creates an HTTP middleware that logs requests to the store.
func Middleware(store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkip(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			requestID := requestctx.RequestID(r.Context())

			wrapped := &responseCapture{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			entry := Entry{
				ID:         requestID,
				Timestamp:  start,
				Method:     r.Method,
				Path:       r.URL.Path,
				Query:      r.URL.RawQuery,
				Status:     wrapped.status,
				Duration:   duration,
				DurationMS: float64(duration.Microseconds()) / 1000.0,
				BytesIn:    r.ContentLength,
				BytesOut:   int64(wrapped.bytes),
				ClientIP:   extractClientIP(r),
				UserAgent:  r.UserAgent(),
			}

			if user := auth.UserFromContext(r.Context()); user != nil {
				entry.UserID = user.ID
			} else if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
				entry.UserID = claims.UserID
			}

			store.Add(entry)
		})
	}
}

func shouldSkip(path string) bool {
	skipPaths := []string{
		"/health",
		"/metrics",
		"/_admin",
	}
	for _, skip := range skipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

type responseCapture struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseCapture) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseCapture) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}
