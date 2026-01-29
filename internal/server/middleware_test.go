package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/requestctx"
)

func TestRecoveryMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	wrapped := RecoveryMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "internal server error" {
		t.Errorf("expected error message 'internal server error', got %v", response["error"])
	}
}

func TestRecoveryMiddleware_NoError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	wrapped := RecoveryMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "success" {
		t.Errorf("expected body 'success', got %s", w.Body.String())
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	var capturedCtx *http.Request

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RequestIDMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	requestID := requestctx.RequestID(capturedCtx.Context())
	if requestID == "" {
		t.Error("request ID should be set in context")
	}

	headerID := w.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Error("X-Request-ID header should be set")
	}

	if requestID != headerID {
		t.Errorf("context request ID %q should match header ID %q", requestID, headerID)
	}
}

func TestRequestIDMiddleware_ExistingID(t *testing.T) {
	existingID := "existing-request-id"

	var capturedCtx *http.Request

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r
		w.WriteHeader(http.StatusOK)
	})

	wrapped := RequestIDMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	requestID := requestctx.RequestID(capturedCtx.Context())
	if requestID != existingID {
		t.Errorf("expected request ID %q, got %q", existingID, requestID)
	}

	headerID := w.Header().Get("X-Request-ID")
	if headerID != existingID {
		t.Errorf("expected header ID %q, got %q", existingID, headerID)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	wrapped := RequestIDMiddleware(LoggingMiddleware(handler))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "test response" {
		t.Errorf("expected body 'test response', got %s", w.Body.String())
	}
}

func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		corsConfig    config.CORSConfig
		origin        string
		method        string
		expectOrigin  string
		expectCreds   bool
		expectStatus  int
		expectMethods bool
		expectHeaders bool
	}{
		{
			name: "allowed origin",
			corsConfig: config.CORSConfig{
				Enabled:          true,
				AllowedOrigins:   []string{"http://localhost:3000"},
				AllowCredentials: true,
			},
			origin:       "http://localhost:3000",
			method:       http.MethodGet,
			expectOrigin: "http://localhost:3000",
			expectCreds:  true,
			expectStatus: http.StatusOK,
		},
		{
			name: "wildcard origin",
			corsConfig: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
			},
			origin:       "http://example.com",
			method:       http.MethodGet,
			expectOrigin: "http://example.com",
			expectStatus: http.StatusOK,
		},
		{
			name: "disallowed origin",
			corsConfig: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"http://localhost:3000"},
			},
			origin:       "http://evil.com",
			method:       http.MethodGet,
			expectOrigin: "",
			expectStatus: http.StatusOK,
		},
		{
			name: "preflight request",
			corsConfig: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"http://localhost:3000"},
				MaxAge:         3600 * time.Second,
			},
			origin:        "http://localhost:3000",
			method:        http.MethodOptions,
			expectOrigin:  "http://localhost:3000",
			expectStatus:  http.StatusNoContent,
			expectMethods: true,
			expectHeaders: true,
		},
		{
			name: "exposed headers",
			corsConfig: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"http://localhost:3000"},
				ExposedHeaders: []string{"X-Custom-Header", "X-Another-Header"},
			},
			origin:       "http://localhost:3000",
			method:       http.MethodGet,
			expectOrigin: "http://localhost:3000",
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := CORSMiddleware(tt.corsConfig)
			wrapped := middleware(handler)

			req := httptest.NewRequest(tt.method, "/", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			origin := w.Header().Get("Access-Control-Allow-Origin")
			if origin != tt.expectOrigin {
				t.Errorf("expected origin %q, got %q", tt.expectOrigin, origin)
			}

			if tt.expectCreds {
				creds := w.Header().Get("Access-Control-Allow-Credentials")
				if creds != "true" {
					t.Errorf("expected credentials header 'true', got %q", creds)
				}
			}

			if tt.expectMethods {
				methods := w.Header().Get("Access-Control-Allow-Methods")
				if methods == "" {
					t.Error("expected Allow-Methods header to be set")
				}
			}

			if tt.expectHeaders {
				headers := w.Header().Get("Access-Control-Allow-Headers")
				if headers == "" {
					t.Error("expected Allow-Headers header to be set")
				}
			}

			if len(tt.corsConfig.ExposedHeaders) > 0 {
				exposed := w.Header().Get("Access-Control-Expose-Headers")
				if tt.expectOrigin != "" && exposed == "" {
					t.Error("expected Expose-Headers header to be set")
				}
			}
		})
	}
}

func TestMaxBodySizeMiddleware(t *testing.T) {
	maxSize := int64(100)

	tests := []struct {
		name         string
		bodySize     int
		expectStatus int
	}{
		{
			name:         "within limit",
			bodySize:     50,
			expectStatus: http.StatusOK,
		},
		{
			name:         "at limit",
			bodySize:     100,
			expectStatus: http.StatusOK,
		},
		{
			name:         "over limit",
			bodySize:     150,
			expectStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write(body)
			})

			middleware := MaxBodySizeMiddleware(maxSize)
			wrapped := middleware(handler)

			body := bytes.Repeat([]byte("a"), tt.bodySize)
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			req.Header.Set("Content-Length", fmt.Sprintf("%d", tt.bodySize))
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}
		})
	}
}

func TestMaxBodySizeMiddleware_ContentLengthCheck(t *testing.T) {
	maxSize := int64(100)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := MaxBodySizeMiddleware(maxSize)
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	req.ContentLength = 150
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d, got %d", http.StatusRequestEntityTooLarge, w.Code)
	}
}

func TestMiddlewareChain(t *testing.T) {
	var order []string

	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	middleware3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m3-before")
			next.ServeHTTP(w, r)
			order = append(order, "m3-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware1(middleware2(middleware3(handler)))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	expected := []string{"m1-before", "m2-before", "m3-before", "handler", "m3-after", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(order))
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d]: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}

	rw.WriteHeader(http.StatusCreated)

	if rw.status != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rw.status)
	}

	if w.Code != http.StatusCreated {
		t.Errorf("expected underlying status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestResponseWriter_Write(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}

	body := []byte("test response")
	n, err := rw.Write(body)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != len(body) {
		t.Errorf("expected to write %d bytes, wrote %d", len(body), n)
	}

	if rw.bytes != len(body) {
		t.Errorf("expected bytes counter %d, got %d", len(body), rw.bytes)
	}

	if w.Body.String() != string(body) {
		t.Errorf("expected body %q, got %q", string(body), w.Body.String())
	}
}

func TestMetricsMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	wrapped := MetricsMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestMetricsMiddleware_SkipsMetricsEndpoint(t *testing.T) {
	var handlerCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := MetricsMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called for /metrics")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/api/users/123", "/api/users/:id"},
		{"/api/users/550e8400-e29b-41d4-a716-446655440000", "/api/users/:id"},
		{"/api/posts/abc123", "/api/posts/abc123"},
		{"/api/collections/users/123/documents/456", "/api/collections/users/:id/documents/:id"},
		{"/health", "/health"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := normalizePath(tt.path)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"550E8400-E29B-41D4-A716-446655440000", true},
		{"invalid-uuid", false},
		{"123", false},
		{"", false},
		{"550e8400-e29b-41d4-a716-44665544000", false},
		{"550e8400-e29b-41d4-a716-44665544000g", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isUUID(tt.input)
			if result != tt.expected {
				t.Errorf("isUUID(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"999999", true},
		{"abc", false},
		{"12a", false},
		{"", false},
		{"-123", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
