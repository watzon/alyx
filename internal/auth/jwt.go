package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/watzon/alyx/internal/config"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidIssuer    = errors.New("invalid token issuer")
	ErrInvalidAudience  = errors.New("invalid token audience")
	ErrMissingSubject   = errors.New("token missing subject")
	ErrInvalidSignature = errors.New("invalid token signature")
)

type jwtClaims struct {
	jwt.RegisteredClaims
	Email    string `json:"email,omitempty"`
	Verified bool   `json:"verified,omitempty"`
	Role     string `json:"role,omitempty"`
}

// JWTService handles JWT token generation and validation.
type JWTService struct {
	secret     []byte
	issuer     string
	audience   []string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewJWTService creates a new JWT service from config.
func NewJWTService(cfg config.JWTConfig) *JWTService {
	return &JWTService{
		secret:     []byte(cfg.Secret),
		issuer:     cfg.Issuer,
		audience:   cfg.Audience,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
	}
}

// GenerateAccessToken creates a new access token for the user.
func (s *JWTService) GenerateAccessToken(user *User) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.accessTTL)

	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
		Email:    user.Email,
		Verified: user.Verified,
	}

	if len(s.audience) > 0 {
		claims.Audience = s.audience
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signedToken, expiresAt, nil
}

// GenerateRefreshToken creates a new refresh token.
func (s *JWTService) GenerateRefreshToken(userID string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.refreshTTL)

	claims := jwt.RegisteredClaims{
		Issuer:    s.issuer,
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		NotBefore: jwt.NewNumericDate(now),
	}

	if len(s.audience) > 0 {
		claims.Audience = s.audience
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signedToken, expiresAt, nil
}

// ValidateAccessToken validates an access token and returns the claims.
func (s *JWTService) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSignature
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Issuer != s.issuer {
		return nil, ErrInvalidIssuer
	}

	if claims.Subject == "" {
		return nil, ErrMissingSubject
	}

	if len(s.audience) > 0 {
		valid := false
		for _, aud := range claims.Audience {
			for _, expected := range s.audience {
				if aud == expected {
					valid = true
					break
				}
			}
		}
		if !valid {
			return nil, ErrInvalidAudience
		}
	}

	return &Claims{
		UserID:   claims.Subject,
		Email:    claims.Email,
		Verified: claims.Verified,
		Role:     claims.Role,
	}, nil
}

// ValidateRefreshToken validates a refresh token and returns the user ID.
func (s *JWTService) ValidateRefreshToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSignature
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrExpiredToken
		}
		return "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", ErrInvalidToken
	}

	if claims.Issuer != s.issuer {
		return "", ErrInvalidIssuer
	}

	if claims.Subject == "" {
		return "", ErrMissingSubject
	}

	return claims.Subject, nil
}
