// Package auth provides authentication and authorization for Alyx.
package auth

import (
	"context"
	"time"
)

// User represents an authenticated user.
type User struct {
	ID        string         `json:"id"`
	Email     string         `json:"email"`
	Verified  bool           `json:"verified"`
	Role      string         `json:"role"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// UserRole constants for the built-in role system.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// IsAdmin returns true if the user has admin role.
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// ListUsersOptions contains options for listing users.
type ListUsersOptions struct {
	Limit   int
	Offset  int
	SortBy  string
	SortDir string // "asc" or "desc"
	Search  string // Search in email
	Role    string // Filter by role
}

// ListUsersResult contains the result of listing users.
type ListUsersResult struct {
	Users []*User `json:"users"`
	Total int     `json:"total"`
}

// UpdateUserInput contains the data for updating a user.
type UpdateUserInput struct {
	Email    *string         `json:"email,omitempty"`
	Verified *bool           `json:"verified,omitempty"`
	Role     *string         `json:"role,omitempty"`
	Metadata *map[string]any `json:"metadata,omitempty"`
}

// CreateUserInput contains the data for creating a user via admin API.
type CreateUserInput struct {
	Email    string         `json:"email"`
	Password string         `json:"password"`
	Verified bool           `json:"verified"`
	Role     string         `json:"role"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Session represents an active user session.
type Session struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	RefreshTokenHash string    `json:"-"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
	UserAgent        string    `json:"user_agent,omitempty"`
	IPAddress        string    `json:"ip_address,omitempty"`
}

// OAuthAccount represents a linked OAuth provider account.
type OAuthAccount struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// TokenPair contains both access and refresh tokens.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// Claims represents the JWT claims for access tokens.
type Claims struct {
	UserID   string `json:"sub"`
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
	Role     string `json:"role,omitempty"`
}

// RegisterInput contains the data needed to register a new user.
type RegisterInput struct {
	Email    string         `json:"email"`
	Password string         `json:"password"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// LoginInput contains the data needed to log in.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshInput contains the refresh token for token refresh.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

// contextKey is used for context values.
type contextKey string

const (
	// userContextKey is the context key for the authenticated user.
	userContextKey contextKey = "auth_user"
	// claimsContextKey is the context key for the JWT claims.
	claimsContextKey contextKey = "auth_claims"
)

// UserFromContext retrieves the authenticated user from the context.
func UserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(userContextKey).(*User); ok {
		return user
	}
	return nil
}

// ClaimsFromContext retrieves the JWT claims from the context.
func ClaimsFromContext(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(claimsContextKey).(*Claims); ok {
		return claims
	}
	return nil
}

// ContextWithUser returns a new context with the user attached.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// ContextWithClaims returns a new context with the claims attached.
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

// IsAuthenticated returns true if the context has an authenticated user.
func IsAuthenticated(ctx context.Context) bool {
	return UserFromContext(ctx) != nil || ClaimsFromContext(ctx) != nil
}
