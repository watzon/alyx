package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
)

func TestNewOAuthManager(t *testing.T) {
	tests := []struct {
		name          string
		cfg           map[string]config.OAuthProviderConfig
		wantProviders []string
		wantMissing   []string
	}{
		{
			name: "github provider configured",
			cfg: map[string]config.OAuthProviderConfig{
				"github": {
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
			},
			wantProviders: []string{"github"},
		},
		{
			name: "google provider configured",
			cfg: map[string]config.OAuthProviderConfig{
				"google": {
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
			},
			wantProviders: []string{"google"},
		},
		{
			name: "multiple providers configured",
			cfg: map[string]config.OAuthProviderConfig{
				"github": {
					ClientID:     "github-client-id",
					ClientSecret: "github-client-secret",
				},
				"google": {
					ClientID:     "google-client-id",
					ClientSecret: "google-client-secret",
				},
			},
			wantProviders: []string{"github", "google"},
		},
		{
			name: "skips providers without credentials",
			cfg: map[string]config.OAuthProviderConfig{
				"github": {
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
				"google": {
					ClientID: "only-client-id",
				},
			},
			wantProviders: []string{"github"},
			wantMissing:   []string{"google"},
		},
		{
			name: "custom OIDC provider",
			cfg: map[string]config.OAuthProviderConfig{
				"custom": {
					ClientID:     "custom-client-id",
					ClientSecret: "custom-client-secret",
					AuthURL:      "https://custom.example.com/auth",
					TokenURL:     "https://custom.example.com/token",
					UserInfoURL:  "https://custom.example.com/userinfo",
				},
			},
			wantProviders: []string{"custom"},
		},
		{
			name: "skips custom provider without auth/token URLs",
			cfg: map[string]config.OAuthProviderConfig{
				"unknown": {
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
			},
			wantMissing: []string{"unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewOAuthManager(tt.cfg)

			providers := m.ListProviders()

			for _, want := range tt.wantProviders {
				found := false
				for _, got := range providers {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected provider %s not found in list: %v", want, providers)
				}
			}

			for _, notWant := range tt.wantMissing {
				for _, got := range providers {
					if got == notWant {
						t.Errorf("provider %s should not be in list: %v", notWant, providers)
					}
				}
			}
		})
	}
}

func TestOAuthManager_GetProvider(t *testing.T) {
	cfg := map[string]config.OAuthProviderConfig{
		"github": {
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
		},
	}
	m := NewOAuthManager(cfg)

	t.Run("existing provider", func(t *testing.T) {
		provider, err := m.GetProvider("github")
		if err != nil {
			t.Fatalf("GetProvider failed: %v", err)
		}
		if provider == nil {
			t.Fatal("GetProvider returned nil")
		}
		if provider.Name() != "github" {
			t.Errorf("expected provider name 'github', got '%s'", provider.Name())
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		provider, err := m.GetProvider("GitHub")
		if err != nil {
			t.Fatalf("GetProvider failed: %v", err)
		}
		if provider == nil {
			t.Fatal("GetProvider returned nil")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := m.GetProvider("nonexistent")
		if !errors.Is(err, ErrProviderNotFound) {
			t.Errorf("expected ErrProviderNotFound, got %v", err)
		}
	})
}

func TestOAuthManager_StateValidation(t *testing.T) {
	m := NewOAuthManager(nil)

	t.Run("valid state", func(t *testing.T) {
		state, err := m.GenerateState()
		if err != nil {
			t.Fatalf("GenerateState failed: %v", err)
		}

		if state == "" {
			t.Fatal("GenerateState returned empty state")
		}

		err = m.ValidateState(state)
		if err != nil {
			t.Errorf("ValidateState failed for valid state: %v", err)
		}
	})

	t.Run("state can only be used once", func(t *testing.T) {
		state, _ := m.GenerateState()

		err := m.ValidateState(state)
		if err != nil {
			t.Fatalf("First validation failed: %v", err)
		}

		err = m.ValidateState(state)
		if !errors.Is(err, ErrInvalidState) {
			t.Errorf("expected ErrInvalidState on second use, got %v", err)
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		err := m.ValidateState("invalid-state-token")
		if !errors.Is(err, ErrInvalidState) {
			t.Errorf("expected ErrInvalidState, got %v", err)
		}
	})

	t.Run("unique states", func(t *testing.T) {
		state1, _ := m.GenerateState()
		state2, _ := m.GenerateState()

		if state1 == state2 {
			t.Error("GenerateState should produce unique states")
		}
	})
}

func TestBaseProvider_AuthURL(t *testing.T) {
	p := &baseProvider{
		name:     "test",
		clientID: "my-client-id",
		authURL:  "https://auth.example.com/authorize",
		scopes:   []string{"email", "profile"},
	}

	state := "test-state-token"
	redirectURI := "https://myapp.com/callback"

	authURL := p.AuthURL(state, redirectURI)

	if !strings.HasPrefix(authURL, "https://auth.example.com/authorize?") {
		t.Errorf("AuthURL should start with auth URL, got: %s", authURL)
	}

	if !strings.Contains(authURL, "client_id=my-client-id") {
		t.Error("AuthURL should contain client_id")
	}

	if !strings.Contains(authURL, "state=test-state-token") {
		t.Error("AuthURL should contain state")
	}

	if !strings.Contains(authURL, "response_type=code") {
		t.Error("AuthURL should contain response_type=code")
	}

	if !strings.Contains(authURL, "scope=email+profile") && !strings.Contains(authURL, "scope=email%20profile") {
		t.Error("AuthURL should contain scopes")
	}
}

func TestBaseProvider_ExchangeCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("expected form content type, got %s", r.Header.Get("Content-Type"))
		}

		err := r.ParseForm()
		if err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}

		if r.Form.Get("code") != "test-auth-code" {
			t.Errorf("expected code 'test-auth-code', got '%s'", r.Form.Get("code"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "test-access-token",
			"token_type":    "Bearer",
			"refresh_token": "test-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	p := &baseProvider{
		name:         "test",
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
		tokenURL:     server.URL,
	}

	token, err := p.ExchangeCode(context.Background(), "test-auth-code", "https://myapp.com/callback")
	if err != nil {
		t.Fatalf("ExchangeCode failed: %v", err)
	}

	if token.AccessToken != "test-access-token" {
		t.Errorf("expected access_token 'test-access-token', got '%s'", token.AccessToken)
	}

	if token.TokenType != "Bearer" {
		t.Errorf("expected token_type 'Bearer', got '%s'", token.TokenType)
	}

	if token.RefreshToken != "test-refresh-token" {
		t.Errorf("expected refresh_token 'test-refresh-token', got '%s'", token.RefreshToken)
	}

	if token.ExpiresAt.Before(time.Now()) {
		t.Error("token should expire in the future")
	}
}

func TestBaseProvider_ExchangeCode_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid_grant"}`))
	}))
	defer server.Close()

	p := &baseProvider{
		name:         "test",
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
		tokenURL:     server.URL,
	}

	_, err := p.ExchangeCode(context.Background(), "invalid-code", "https://myapp.com/callback")
	if err == nil {
		t.Error("ExchangeCode should fail for invalid code")
	}

	if !strings.Contains(err.Error(), "failed to exchange token") {
		t.Errorf("error should mention token exchange failure: %v", err)
	}
}

func TestFetchUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-access-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "12345",
			"email": "user@example.com",
			"name":  "Test User",
		})
	}))
	defer server.Close()

	data, err := fetchUserInfo(context.Background(), server.URL, "test-access-token")
	if err != nil {
		t.Fatalf("fetchUserInfo failed: %v", err)
	}

	if data["id"] != "12345" {
		t.Errorf("expected id '12345', got '%v'", data["id"])
	}

	if data["email"] != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got '%v'", data["email"])
	}
}

func TestFetchUserInfo_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	_, err := fetchUserInfo(context.Background(), server.URL, "invalid-token")
	if err == nil {
		t.Error("fetchUserInfo should fail for unauthorized request")
	}
}

func TestGitHubProvider(t *testing.T) {
	cfg := config.OAuthProviderConfig{
		ClientID:     "github-client-id",
		ClientSecret: "github-client-secret",
		Scopes:       []string{"user:email"},
	}

	p := newGitHubProvider(cfg)

	if p.Name() != "github" {
		t.Errorf("expected name 'github', got '%s'", p.Name())
	}

	authURL := p.AuthURL("test-state", "https://app.com/callback")
	if !strings.Contains(authURL, "github.com") {
		t.Errorf("GitHub AuthURL should contain github.com: %s", authURL)
	}
}

func TestGoogleProvider(t *testing.T) {
	cfg := config.OAuthProviderConfig{
		ClientID:     "google-client-id",
		ClientSecret: "google-client-secret",
		Scopes:       []string{"email", "profile"},
	}

	p := newGoogleProvider(cfg)

	if p.Name() != "google" {
		t.Errorf("expected name 'google', got '%s'", p.Name())
	}

	authURL := p.AuthURL("test-state", "https://app.com/callback")
	if !strings.Contains(authURL, "google.com") {
		t.Errorf("Google AuthURL should contain google.com: %s", authURL)
	}
}

func TestGenericOIDCProvider(t *testing.T) {
	cfg := config.OAuthProviderConfig{
		ClientID:     "custom-client-id",
		ClientSecret: "custom-client-secret",
		AuthURL:      "https://auth.custom.com/authorize",
		TokenURL:     "https://auth.custom.com/token",
		UserInfoURL:  "https://auth.custom.com/userinfo",
		Scopes:       []string{"openid", "email"},
	}

	p := newGenericOIDCProvider("custom", cfg)

	if p.Name() != "custom" {
		t.Errorf("expected name 'custom', got '%s'", p.Name())
	}

	authURL := p.AuthURL("test-state", "https://app.com/callback")
	if !strings.HasPrefix(authURL, "https://auth.custom.com/authorize") {
		t.Errorf("Custom provider AuthURL should use configured auth URL: %s", authURL)
	}
}
