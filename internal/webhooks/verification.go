package webhooks

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 - SHA1 required for compatibility with some webhook providers
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
)

// VerificationResult contains the result of webhook signature verification.
type VerificationResult struct {
	Valid  bool   // Whether signature is valid
	Error  string // Error message if verification failed
	Method string // Verification method used
}

// VerifySignature verifies a webhook signature using the configured verification method.
func VerifySignature(verification *WebhookVerification, body []byte, signature string) *VerificationResult {
	if verification == nil {
		return &VerificationResult{
			Valid:  true,
			Method: "none",
		}
	}

	var h hash.Hash
	var method string

	switch verification.Type {
	case "hmac-sha256":
		h = hmac.New(sha256.New, []byte(verification.Secret))
		method = "hmac-sha256"
	case "hmac-sha1":
		h = hmac.New(sha1.New, []byte(verification.Secret))
		method = "hmac-sha1"
	default:
		return &VerificationResult{
			Valid:  false,
			Error:  fmt.Sprintf("unsupported verification type: %s", verification.Type),
			Method: verification.Type,
		}
	}

	// Compute HMAC
	h.Write(body)
	expectedMAC := h.Sum(nil)

	// Parse signature from header
	// Support formats:
	// - "sha256=<hex>" (GitHub style)
	// - "<hex>" (raw hex)
	actualHex := signature
	if strings.Contains(signature, "=") {
		parts := strings.SplitN(signature, "=", 2)
		if len(parts) == 2 {
			actualHex = parts[1]
		}
	}

	// Decode actual signature
	actualMAC, err := hex.DecodeString(actualHex)
	if err != nil {
		return &VerificationResult{
			Valid:  false,
			Error:  fmt.Sprintf("invalid signature format: %v", err),
			Method: method,
		}
	}

	// Constant-time comparison to prevent timing attacks
	valid := hmac.Equal(expectedMAC, actualMAC)

	result := &VerificationResult{
		Valid:  valid,
		Method: method,
	}

	if !valid {
		result.Error = "signature mismatch"
	}

	return result
}

// ExtractSignature extracts the signature from request headers.
func ExtractSignature(headers map[string]string, headerName string) string {
	// Try exact match first
	if sig := headers[headerName]; sig != "" {
		return sig
	}

	// Try case-insensitive match
	lowerHeaderName := strings.ToLower(headerName)
	for k, v := range headers {
		if strings.ToLower(k) == lowerHeaderName {
			return v
		}
	}

	return ""
}
