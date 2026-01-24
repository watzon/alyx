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

// passwordCharacteristics holds the character type flags for a password.
type passwordCharacteristics struct {
	hasUpper   bool
	hasLower   bool
	hasNumber  bool
	hasSpecial bool
}

// analyzePassword analyzes a password and returns its character characteristics.
func analyzePassword(password string) passwordCharacteristics {
	var chars passwordCharacteristics

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			chars.hasUpper = true
		case unicode.IsLower(r):
			chars.hasLower = true
		case unicode.IsDigit(r):
			chars.hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			chars.hasSpecial = true
		}
	}

	return chars
}

// ValidatePassword checks if a password meets the configured requirements.
func ValidatePassword(password string, cfg config.PasswordConfig) error {
	if len(password) < cfg.MinLength {
		return ErrPasswordTooShort
	}

	chars := analyzePassword(password)

	if cfg.RequireUppercase && !chars.hasUpper {
		return ErrPasswordNoUppercase
	}
	if cfg.RequireLowercase && !chars.hasLower {
		return ErrPasswordNoLowercase
	}
	if cfg.RequireNumber && !chars.hasNumber {
		return ErrPasswordNoNumber
	}
	if cfg.RequireSpecial && !chars.hasSpecial {
		return ErrPasswordNoSpecial
	}

	return nil
}
