package auth

import (
	"errors"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"github.com/watzon/alyx/internal/config"
)

const (
	bcryptCost = 12
)

var (
	ErrPasswordTooShort     = errors.New("password is too short")
	ErrPasswordNoUppercase  = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLowercase  = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoNumber     = errors.New("password must contain at least one number")
	ErrPasswordNoSpecial    = errors.New("password must contain at least one special character")
	ErrInvalidPassword      = errors.New("invalid password")
	ErrPasswordHashMismatch = errors.New("password does not match")
)

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks if a password matches a hash.
func VerifyPassword(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrPasswordHashMismatch
	}
	return err
}

// ValidatePassword checks if a password meets the configured requirements.
func ValidatePassword(password string, cfg config.PasswordConfig) error {
	if len(password) < cfg.MinLength {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if cfg.RequireUppercase && !hasUpper {
		return ErrPasswordNoUppercase
	}
	if cfg.RequireLowercase && !hasLower {
		return ErrPasswordNoLowercase
	}
	if cfg.RequireNumber && !hasNumber {
		return ErrPasswordNoNumber
	}
	if cfg.RequireSpecial && !hasSpecial {
		return ErrPasswordNoSpecial
	}

	return nil
}
