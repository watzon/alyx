package webhooks

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 - SHA1 required for testing webhook compatibility
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

const testSecret = "my-secret-key"

func TestVerifySignature_HMACSHA256(t *testing.T) {
	secret := testSecret
	body := []byte(`{"event":"test","data":"hello"}`)

	// Compute expected signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedMAC := h.Sum(nil)
	expectedHex := hex.EncodeToString(expectedMAC)

	tests := []struct {
		name       string
		signature  string
		wantValid  bool
		wantError  string
		wantMethod string
	}{
		{
			name:       "valid signature with sha256= prefix",
			signature:  "sha256=" + expectedHex,
			wantValid:  true,
			wantMethod: "hmac-sha256",
		},
		{
			name:       "valid signature without prefix",
			signature:  expectedHex,
			wantValid:  true,
			wantMethod: "hmac-sha256",
		},
		{
			name:       "invalid signature",
			signature:  "sha256=invalid",
			wantValid:  false,
			wantError:  "invalid signature format",
			wantMethod: "hmac-sha256",
		},
		{
			name:       "wrong signature",
			signature:  "sha256=" + hex.EncodeToString([]byte("wrong")),
			wantValid:  false,
			wantError:  "signature mismatch",
			wantMethod: "hmac-sha256",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verification := &WebhookVerification{
				Type:   "hmac-sha256",
				Secret: secret,
			}

			result := VerifySignature(verification, body, tt.signature)

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.Method != tt.wantMethod {
				t.Errorf("Method = %v, want %v", result.Method, tt.wantMethod)
			}

			if tt.wantError != "" {
				if result.Error == "" {
					t.Errorf("Expected error containing %q, got none", tt.wantError)
				}
			} else if result.Error != "" {
				t.Errorf("Unexpected error: %v", result.Error)
			}
		})
	}
}

func TestVerifySignature_HMACSHA1(t *testing.T) {
	secret := testSecret
	body := []byte(`{"event":"test","data":"hello"}`)

	// Compute expected signature
	h := hmac.New(sha1.New, []byte(secret))
	h.Write(body)
	expectedMAC := h.Sum(nil)
	expectedHex := hex.EncodeToString(expectedMAC)

	tests := []struct {
		name       string
		signature  string
		wantValid  bool
		wantMethod string
	}{
		{
			name:       "valid signature with sha1= prefix",
			signature:  "sha1=" + expectedHex,
			wantValid:  true,
			wantMethod: "hmac-sha1",
		},
		{
			name:       "valid signature without prefix",
			signature:  expectedHex,
			wantValid:  true,
			wantMethod: "hmac-sha1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verification := &WebhookVerification{
				Type:   "hmac-sha1",
				Secret: secret,
			}

			result := VerifySignature(verification, body, tt.signature)

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.Method != tt.wantMethod {
				t.Errorf("Method = %v, want %v", result.Method, tt.wantMethod)
			}
		})
	}
}

func TestVerifySignature_NoVerification(t *testing.T) {
	body := []byte(`{"event":"test"}`)

	result := VerifySignature(nil, body, "")

	if !result.Valid {
		t.Errorf("Expected valid result when verification is nil")
	}

	if result.Method != "none" {
		t.Errorf("Method = %v, want 'none'", result.Method)
	}
}

func TestVerifySignature_UnsupportedType(t *testing.T) {
	verification := &WebhookVerification{
		Type:   "unsupported",
		Secret: "secret",
	}

	body := []byte(`{"event":"test"}`)
	result := VerifySignature(verification, body, "signature")

	if result.Valid {
		t.Errorf("Expected invalid result for unsupported type")
	}

	if result.Error == "" {
		t.Errorf("Expected error for unsupported type")
	}

	if result.Method != "unsupported" {
		t.Errorf("Method = %v, want 'unsupported'", result.Method)
	}
}

func TestExtractSignature(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		headerName string
		want       string
	}{
		{
			name: "exact match",
			headers: map[string]string{
				"X-Hub-Signature": "sha256=abc123",
			},
			headerName: "X-Hub-Signature",
			want:       "sha256=abc123",
		},
		{
			name: "case insensitive match",
			headers: map[string]string{
				"x-hub-signature": "sha256=abc123",
			},
			headerName: "X-Hub-Signature",
			want:       "sha256=abc123",
		},
		{
			name: "not found",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			headerName: "X-Hub-Signature",
			want:       "",
		},
		{
			name:       "empty headers",
			headers:    map[string]string{},
			headerName: "X-Hub-Signature",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSignature(tt.headers, tt.headerName)
			if got != tt.want {
				t.Errorf("ExtractSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestVerifySignature_ConstantTime ensures we use constant-time comparison.
// This is a basic test - timing attacks would require more sophisticated testing.
func TestVerifySignature_ConstantTime(t *testing.T) {
	secret := "my-secret-key"
	body := []byte(`{"event":"test"}`)

	// Compute correct signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	correctMAC := h.Sum(nil)
	correctHex := hex.EncodeToString(correctMAC)

	verification := &WebhookVerification{
		Type:   "hmac-sha256",
		Secret: secret,
	}

	// Test with correct signature
	result := VerifySignature(verification, body, correctHex)
	if !result.Valid {
		t.Errorf("Expected valid signature")
	}

	// Test with signature that differs only in last byte
	wrongHex := correctHex[:len(correctHex)-2] + "ff"
	result = VerifySignature(verification, body, wrongHex)
	if result.Valid {
		t.Errorf("Expected invalid signature")
	}

	// Test with signature that differs in first byte
	wrongHex = "ff" + correctHex[2:]
	result = VerifySignature(verification, body, wrongHex)
	if result.Valid {
		t.Errorf("Expected invalid signature")
	}
}
