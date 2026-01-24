package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("HashPassword returned empty hash")
	}

	if hash == password {
		t.Error("HashPassword returned unhashed password")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if err := VerifyPassword(password, hash); err != nil {
		t.Errorf("VerifyPassword failed for correct password: %v", err)
	}

	if err := VerifyPassword("wrongpassword", hash); err == nil {
		t.Error("VerifyPassword should fail for wrong password")
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		cfg      config.PasswordConfig
		wantErr  error
	}{
		{
			name:     "valid password",
			password: "testpassword123",
			cfg:      config.PasswordConfig{MinLength: 8},
			wantErr:  nil,
		},
		{
			name:     "too short",
			password: "short",
			cfg:      config.PasswordConfig{MinLength: 8},
			wantErr:  ErrPasswordTooShort,
		},
		{
			name:     "missing uppercase",
			password: "lowercase123",
			cfg:      config.PasswordConfig{MinLength: 8, RequireUppercase: true},
			wantErr:  ErrPasswordNoUppercase,
		},
		{
			name:     "has uppercase",
			password: "Lowercase123",
			cfg:      config.PasswordConfig{MinLength: 8, RequireUppercase: true},
			wantErr:  nil,
		},
		{
			name:     "missing lowercase",
			password: "UPPERCASE123",
			cfg:      config.PasswordConfig{MinLength: 8, RequireLowercase: true},
			wantErr:  ErrPasswordNoLowercase,
		},
		{
			name:     "missing number",
			password: "noNumbersHere",
			cfg:      config.PasswordConfig{MinLength: 8, RequireNumber: true},
			wantErr:  ErrPasswordNoNumber,
		},
		{
			name:     "missing special",
			password: "noSpecial123",
			cfg:      config.PasswordConfig{MinLength: 8, RequireSpecial: true},
			wantErr:  ErrPasswordNoSpecial,
		},
		{
			name:     "has special",
			password: "hasSpecial!23",
			cfg:      config.PasswordConfig{MinLength: 8, RequireSpecial: true},
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password, tt.cfg)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func testJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Secret:     "testsecret12345678901234567890123456",
		Issuer:     "test",
		Audience:   []string{"test-audience"},
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
	}
}

func TestJWTService_GenerateAccessToken(t *testing.T) {
	svc := NewJWTService(testJWTConfig())

	user := &User{
		ID:       "user123",
		Email:    "test@example.com",
		Verified: true,
	}

	token, expiresAt, err := svc.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	if token == "" {
		t.Error("GenerateAccessToken returned empty token")
	}

	if expiresAt.Before(time.Now()) {
		t.Error("Token expires in the past")
	}

	expectedExpiry := time.Now().Add(15 * time.Minute)
	if expiresAt.Sub(expectedExpiry) > time.Minute {
		t.Errorf("Token expiry time is incorrect: got %v, expected around %v", expiresAt, expectedExpiry)
	}
}

func TestJWTService_ValidateAccessToken(t *testing.T) {
	svc := NewJWTService(testJWTConfig())

	user := &User{
		ID:       "user123",
		Email:    "test@example.com",
		Verified: true,
	}

	token, _, err := svc.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("UserID mismatch: got %s, want %s", claims.UserID, user.ID)
	}

	if claims.Email != user.Email {
		t.Errorf("Email mismatch: got %s, want %s", claims.Email, user.Email)
	}

	if claims.Verified != user.Verified {
		t.Errorf("Verified mismatch: got %v, want %v", claims.Verified, user.Verified)
	}
}

func TestJWTService_InvalidToken(t *testing.T) {
	svc := NewJWTService(testJWTConfig())

	_, err := svc.ValidateAccessToken("invalid-token")
	if err == nil {
		t.Error("ValidateAccessToken should fail for invalid token")
	}
}

func TestJWTService_WrongSecret(t *testing.T) {
	svc1 := NewJWTService(testJWTConfig())

	cfg2 := testJWTConfig()
	cfg2.Secret = "differentsecret1234567890123456789"
	svc2 := NewJWTService(cfg2)

	user := &User{
		ID:       "user123",
		Email:    "test@example.com",
		Verified: true,
	}

	token, _, err := svc1.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	_, err = svc2.ValidateAccessToken(token)
	if err == nil {
		t.Error("ValidateAccessToken should fail for token signed with different secret")
	}
}

func TestJWTService_RefreshToken(t *testing.T) {
	svc := NewJWTService(testJWTConfig())

	userID := "user123"

	token, expiresAt, err := svc.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	if token == "" {
		t.Error("GenerateRefreshToken returned empty token")
	}

	expectedExpiry := time.Now().Add(7 * 24 * time.Hour)
	if expiresAt.Sub(expectedExpiry) > time.Minute {
		t.Errorf("Refresh token expiry incorrect: got %v, expected around %v", expiresAt, expectedExpiry)
	}

	validatedUserID, err := svc.ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken failed: %v", err)
	}

	if validatedUserID != userID {
		t.Errorf("UserID mismatch: got %s, want %s", validatedUserID, userID)
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	if IsAuthenticated(ctx) {
		t.Error("Empty context should not be authenticated")
	}

	user := &User{
		ID:    "user123",
		Email: "test@example.com",
	}

	ctx = ContextWithUser(ctx, user)

	if !IsAuthenticated(ctx) {
		t.Error("Context with user should be authenticated")
	}

	retrievedUser := UserFromContext(ctx)
	if retrievedUser == nil {
		t.Fatal("UserFromContext returned nil")
	}

	if retrievedUser.ID != user.ID {
		t.Errorf("User ID mismatch: got %s, want %s", retrievedUser.ID, user.ID)
	}

	claims := &Claims{
		UserID: "user456",
		Email:  "claims@example.com",
	}

	ctx2 := ContextWithClaims(context.Background(), claims)

	if !IsAuthenticated(ctx2) {
		t.Error("Context with claims should be authenticated")
	}

	retrievedClaims := ClaimsFromContext(ctx2)
	if retrievedClaims == nil {
		t.Fatal("ClaimsFromContext returned nil")
	}

	if retrievedClaims.UserID != claims.UserID {
		t.Errorf("Claims UserID mismatch: got %s, want %s", retrievedClaims.UserID, claims.UserID)
	}
}
