package webhooks

import "time"

// WebhookEndpoint represents a registered webhook endpoint.
type WebhookEndpoint struct {
	ID           string               // Unique endpoint ID
	Path         string               // Webhook path (e.g., "/webhooks/stripe")
	FunctionID   string               // Function to invoke
	Methods      []string             // Allowed HTTP methods (e.g., ["POST"])
	Verification *WebhookVerification // Optional verification config
	Enabled      bool                 // Whether endpoint is enabled
	CreatedAt    time.Time            // When endpoint was created
}

// WebhookVerification contains configuration for webhook signature verification.
type WebhookVerification struct {
	Type        string // Verification type: "hmac-sha256", "hmac-sha1"
	Header      string // Header containing signature (e.g., "X-Hub-Signature")
	Secret      string // Secret key for HMAC verification
	SkipInvalid bool   // If true, pass verification result to function; if false, reject with 401
}
