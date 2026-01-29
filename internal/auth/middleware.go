package auth

import (
	"net/http"
	"strings"
)

type MiddlewareConfig struct {
	Service        *Service
	RequireAuth    bool
	AllowAnonymous bool
}

func Middleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)

			if token == "" {
				if cfg.RequireAuth && !cfg.AllowAnonymous {
					http.Error(w, `{"error":"Authentication required","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if cfg.Service.IsTokenRevoked(token) {
				if cfg.RequireAuth {
					http.Error(w, `{"error":"Token has been revoked","code":"TOKEN_REVOKED"}`, http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			claims, err := cfg.Service.ValidateToken(token)
			if err != nil {
				if cfg.RequireAuth {
					http.Error(w, `{"error":"Invalid or expired token","code":"INVALID_TOKEN"}`, http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx := ContextWithClaims(r.Context(), claims)

			if claims.UserID != "" {
				user, err := cfg.Service.GetUserByID(r.Context(), claims.UserID)
				if err == nil {
					ctx = ContextWithUser(ctx, user)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAuth(service *Service) func(http.Handler) http.Handler {
	return Middleware(MiddlewareConfig{
		Service:     service,
		RequireAuth: true,
	})
}

func OptionalAuth(service *Service) func(http.Handler) http.Handler {
	return Middleware(MiddlewareConfig{
		Service:        service,
		RequireAuth:    false,
		AllowAnonymous: true,
	})
}

func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}
