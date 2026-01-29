package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

type AuthHandlers struct {
	service             *auth.Service
	cfg                 *config.AuthConfig
	bruteForceProtector BruteForceProtector
}

type BruteForceProtector interface {
	IsBlocked(key string) bool
	RecordFailedAttempt(key string)
	ClearAttempts(key string)
}

func NewAuthHandlers(db *database.DB, cfg *config.AuthConfig, bfp BruteForceProtector) *AuthHandlers {
	return &AuthHandlers{
		service:             auth.NewService(db, cfg),
		cfg:                 cfg,
		bruteForceProtector: bfp,
	}
}

func (h *AuthHandlers) Service() *auth.Service {
	return h.service
}

func (h *AuthHandlers) Status(w http.ResponseWriter, r *http.Request) {
	hasUsers, err := h.service.HasUsers(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to check for users")
		InternalError(w, "Failed to check auth status")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"needs_setup":        !hasUsers,
		"allow_registration": h.cfg.AllowRegistration,
	})
}

func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var input auth.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if input.Email == "" {
		Error(w, http.StatusBadRequest, "EMAIL_REQUIRED", "Email is required")
		return
	}

	if input.Password == "" {
		Error(w, http.StatusBadRequest, "PASSWORD_REQUIRED", "Password is required")
		return
	}

	user, tokens, err := h.service.Register(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserAlreadyExists):
			Error(w, http.StatusConflict, "USER_EXISTS", "User with this email already exists")
		case errors.Is(err, auth.ErrRegistrationClosed):
			Error(w, http.StatusForbidden, "REGISTRATION_CLOSED", "Registration is disabled")
		case errors.Is(err, auth.ErrPasswordTooShort):
			Error(w, http.StatusBadRequest, "PASSWORD_TOO_SHORT", "Password is too short")
		case errors.Is(err, auth.ErrPasswordNoUppercase):
			Error(w, http.StatusBadRequest, "PASSWORD_NO_UPPERCASE", "Password must contain an uppercase letter")
		case errors.Is(err, auth.ErrPasswordNoLowercase):
			Error(w, http.StatusBadRequest, "PASSWORD_NO_LOWERCASE", "Password must contain a lowercase letter")
		case errors.Is(err, auth.ErrPasswordNoNumber):
			Error(w, http.StatusBadRequest, "PASSWORD_NO_NUMBER", "Password must contain a number")
		case errors.Is(err, auth.ErrPasswordNoSpecial):
			Error(w, http.StatusBadRequest, "PASSWORD_NO_SPECIAL", "Password must contain a special character")
		default:
			log.Error().Err(err).Msg("Failed to register user")
			InternalError(w, "Failed to register user")
		}
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var input auth.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if input.Email == "" {
		Error(w, http.StatusBadRequest, "EMAIL_REQUIRED", "Email is required")
		return
	}

	if input.Password == "" {
		Error(w, http.StatusBadRequest, "PASSWORD_REQUIRED", "Password is required")
		return
	}

	if h.bruteForceProtector != nil && h.bruteForceProtector.IsBlocked(input.Email) {
		Error(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "Too many failed login attempts. Please try again later.")
		return
	}

	userAgent := r.Header.Get("User-Agent")
	ipAddress := getClientIP(r)

	user, tokens, err := h.service.Login(r.Context(), input, userAgent, ipAddress)
	if err != nil {
		if h.bruteForceProtector != nil && errors.Is(err, auth.ErrInvalidCredentials) {
			h.bruteForceProtector.RecordFailedAttempt(input.Email)
		}

		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		case errors.Is(err, auth.ErrEmailNotVerified):
			Error(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Email not verified")
		default:
			log.Error().Err(err).Msg("Failed to login user")
			InternalError(w, "Failed to login")
		}
		return
	}

	if h.bruteForceProtector != nil {
		h.bruteForceProtector.ClearAttempts(input.Email)
	}

	JSON(w, http.StatusOK, map[string]any{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var input auth.RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if input.RefreshToken == "" {
		Error(w, http.StatusBadRequest, "REFRESH_TOKEN_REQUIRED", "Refresh token is required")
		return
	}

	user, tokens, err := h.service.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidToken):
			Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid refresh token")
		case errors.Is(err, auth.ErrExpiredToken):
			Error(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Refresh token has expired")
		case errors.Is(err, auth.ErrSessionNotFound):
			Error(w, http.StatusUnauthorized, "SESSION_NOT_FOUND", "Session not found")
		case errors.Is(err, auth.ErrSessionExpired):
			Error(w, http.StatusUnauthorized, "SESSION_EXPIRED", "Session has expired")
		default:
			log.Error().Err(err).Msg("Failed to refresh token")
			InternalError(w, "Failed to refresh token")
		}
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	var input auth.RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if input.RefreshToken == "" {
		Error(w, http.StatusBadRequest, "REFRESH_TOKEN_REQUIRED", "Refresh token is required")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != "" && token != authHeader {
			claims, err := h.service.ValidateToken(token)
			if err == nil {
				h.service.RevokeToken(token, claims.ExpiresAt)
			}
		}
	}

	if err := h.service.Logout(r.Context(), input.RefreshToken); err != nil {
		log.Error().Err(err).Msg("Failed to logout")
		InternalError(w, "Failed to logout")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		Unauthorized(w, "Not authenticated")
		return
	}

	JSON(w, http.StatusOK, user)
}

func (h *AuthHandlers) Providers(w http.ResponseWriter, r *http.Request) {
	providers := make([]string, 0)
	for name, cfg := range h.cfg.OAuth {
		if cfg.ClientID != "" && cfg.ClientSecret != "" {
			providers = append(providers, name)
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"providers": providers,
	})
}

// OAuthRedirect initiates the OAuth flow by redirecting to the provider's auth URL.
func (h *AuthHandlers) OAuthRedirect(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	if providerName == "" {
		Error(w, http.StatusBadRequest, "PROVIDER_REQUIRED", "Provider name is required")
		return
	}

	provider, err := h.service.OAuth().GetProvider(providerName)
	if err != nil {
		if errors.Is(err, auth.ErrProviderNotFound) {
			Error(w, http.StatusNotFound, "PROVIDER_NOT_FOUND", "OAuth provider not found")
			return
		}
		log.Error().Err(err).Str("provider", providerName).Msg("Failed to get OAuth provider")
		InternalError(w, "Failed to get OAuth provider")
		return
	}

	state, err := h.service.OAuth().GenerateState()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate OAuth state")
		InternalError(w, "Failed to generate OAuth state")
		return
	}

	redirectURI := buildRedirectURI(r, providerName)

	authURL := provider.AuthURL(state, redirectURI)

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// OAuthCallback handles the OAuth callback from the provider.
func (h *AuthHandlers) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	if providerName == "" {
		Error(w, http.StatusBadRequest, "PROVIDER_REQUIRED", "Provider name is required")
		return
	}

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		log.Warn().Str("provider", providerName).Str("error", errParam).Str("description", errDesc).Msg("OAuth provider returned error")
		Error(w, http.StatusBadRequest, "OAUTH_ERROR", errDesc)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		Error(w, http.StatusBadRequest, "CODE_REQUIRED", "Authorization code is required")
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		Error(w, http.StatusBadRequest, "STATE_REQUIRED", "State parameter is required")
		return
	}

	if err := h.service.OAuth().ValidateState(state); err != nil {
		if errors.Is(err, auth.ErrInvalidState) {
			Error(w, http.StatusBadRequest, "INVALID_STATE", "Invalid state parameter")
			return
		}
		if errors.Is(err, auth.ErrStateExpired) {
			Error(w, http.StatusBadRequest, "STATE_EXPIRED", "State parameter has expired")
			return
		}
		log.Error().Err(err).Msg("Failed to validate OAuth state")
		InternalError(w, "Failed to validate OAuth state")
		return
	}

	provider, err := h.service.OAuth().GetProvider(providerName)
	if err != nil {
		if errors.Is(err, auth.ErrProviderNotFound) {
			Error(w, http.StatusNotFound, "PROVIDER_NOT_FOUND", "OAuth provider not found")
			return
		}
		log.Error().Err(err).Str("provider", providerName).Msg("Failed to get OAuth provider")
		InternalError(w, "Failed to get OAuth provider")
		return
	}

	redirectURI := buildRedirectURI(r, providerName)

	token, err := provider.ExchangeCode(r.Context(), code, redirectURI)
	if err != nil {
		log.Error().Err(err).Str("provider", providerName).Msg("Failed to exchange OAuth code")
		Error(w, http.StatusBadRequest, "TOKEN_EXCHANGE_FAILED", "Failed to exchange authorization code")
		return
	}

	userInfo, err := provider.GetUserInfo(r.Context(), token)
	if err != nil {
		log.Error().Err(err).Str("provider", providerName).Msg("Failed to get user info from OAuth provider")
		Error(w, http.StatusBadRequest, "USER_INFO_FAILED", "Failed to get user information from provider")
		return
	}

	if userInfo.Email == "" {
		Error(w, http.StatusBadRequest, "EMAIL_REQUIRED", "Email is required from OAuth provider")
		return
	}

	userAgent := r.Header.Get("User-Agent")
	ipAddress := getClientIP(r)

	user, tokens, err := h.service.OAuthLogin(r.Context(), userInfo, userAgent, ipAddress)
	if err != nil {
		if errors.Is(err, auth.ErrAccountAlreadyLinked) {
			Error(w, http.StatusConflict, "ACCOUNT_ALREADY_LINKED", "This OAuth account is already linked to another user")
			return
		}
		log.Error().Err(err).Str("provider", providerName).Msg("Failed to complete OAuth login")
		InternalError(w, "Failed to complete OAuth login")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"user":   user,
		"tokens": tokens,
	})
}

// buildRedirectURI constructs the OAuth callback URI from the request.
func buildRedirectURI(r *http.Request, provider string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	return scheme + "://" + host + "/api/auth/oauth/" + provider + "/callback"
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip := r.RemoteAddr
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		ip = ip[:colonIdx]
	}
	return ip
}
