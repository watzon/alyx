package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const signedURLPartCount = 6

var (
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrInvalidToken     = errors.New("invalid token format")
)

type SignedURLClaims struct {
	FileID    string
	Bucket    string
	Operation string
	ExpiresAt time.Time
	UserID    string
}

type SignedURLService struct {
	secret []byte
}

func NewSignedURLService(secret []byte) *SignedURLService {
	return &SignedURLService{
		secret: secret,
	}
}

func (s *SignedURLService) GenerateSignedURL(fileID, bucket, operation string, expiry time.Duration, userID string) (string, time.Time, error) {
	expiresAt := time.Now().Add(expiry)
	expiresAtStr := expiresAt.Format(time.RFC3339)

	payload := fmt.Sprintf("%s|%s|%s|%s|%s", fileID, bucket, operation, expiresAtStr, userID)

	signature := s.sign(payload)

	token := base64.URLEncoding.EncodeToString([]byte(payload + "|" + signature))

	return token, expiresAt, nil
}

func (s *SignedURLService) ValidateSignedURL(token, fileID, bucket string) (*SignedURLClaims, error) {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != signedURLPartCount {
		return nil, ErrInvalidToken
	}

	tokenFileID := parts[0]
	tokenBucket := parts[1]
	operation := parts[2]
	expiresAtStr := parts[3]
	userID := parts[4]
	signature := parts[5]

	if tokenFileID != fileID || tokenBucket != bucket {
		return nil, ErrInvalidSignature
	}

	payload := fmt.Sprintf("%s|%s|%s|%s|%s", tokenFileID, tokenBucket, operation, expiresAtStr, userID)
	expectedSignature := s.sign(payload)

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, ErrInvalidSignature
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().After(expiresAt) {
		return nil, ErrExpiredToken
	}

	return &SignedURLClaims{
		FileID:    tokenFileID,
		Bucket:    tokenBucket,
		Operation: operation,
		ExpiresAt: expiresAt,
		UserID:    userID,
	}, nil
}

func (s *SignedURLService) sign(payload string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(payload))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}
