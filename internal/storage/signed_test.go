package storage

import (
	"testing"
	"time"
)

func TestSignedURLService_GenerateSignedURL(t *testing.T) {
	secret := []byte("test-secret-key-for-signing")
	svc := NewSignedURLService(secret)

	fileID := "file-123"
	bucket := "uploads"
	operation := "download"
	expiry := 15 * time.Minute
	userID := "user-456"

	token, expiresAt, err := svc.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	if expiresAt.IsZero() {
		t.Error("Expected non-zero expiry time")
	}

	expectedExpiry := time.Now().Add(expiry)
	if expiresAt.Before(expectedExpiry.Add(-1*time.Second)) || expiresAt.After(expectedExpiry.Add(1*time.Second)) {
		t.Errorf("Expected expiry around %v, got %v", expectedExpiry, expiresAt)
	}
}

func TestSignedURLService_ValidateSignedURL(t *testing.T) {
	secret := []byte("test-secret-key-for-signing")
	svc := NewSignedURLService(secret)

	fileID := "file-123"
	bucket := "uploads"
	operation := "download"
	expiry := 15 * time.Minute
	userID := "user-456"

	token, _, err := svc.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	claims, err := svc.ValidateSignedURL(token, fileID, bucket)
	if err != nil {
		t.Fatalf("ValidateSignedURL failed: %v", err)
	}

	if claims.FileID != fileID {
		t.Errorf("Expected file ID %s, got %s", fileID, claims.FileID)
	}

	if claims.Bucket != bucket {
		t.Errorf("Expected bucket %s, got %s", bucket, claims.Bucket)
	}

	if claims.Operation != operation {
		t.Errorf("Expected operation %s, got %s", operation, claims.Operation)
	}

	if claims.UserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, claims.UserID)
	}
}

func TestSignedURLService_ValidateExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key-for-signing")
	svc := NewSignedURLService(secret)

	fileID := "file-123"
	bucket := "uploads"
	operation := "download"
	expiry := -1 * time.Second // Already expired
	userID := "user-456"

	token, _, err := svc.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	_, err = svc.ValidateSignedURL(token, fileID, bucket)
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got %v", err)
	}
}

func TestSignedURLService_ValidateTamperedToken(t *testing.T) {
	secret := []byte("test-secret-key-for-signing")
	svc := NewSignedURLService(secret)

	fileID := "file-123"
	bucket := "uploads"
	operation := "download"
	expiry := 15 * time.Minute
	userID := "user-456"

	token, _, err := svc.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	// Tamper with token by changing a character
	tamperedToken := token[:len(token)-5] + "XXXXX"

	_, err = svc.ValidateSignedURL(tamperedToken, fileID, bucket)
	if err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature, got %v", err)
	}
}

func TestSignedURLService_ValidateWrongFileID(t *testing.T) {
	secret := []byte("test-secret-key-for-signing")
	svc := NewSignedURLService(secret)

	fileID := "file-123"
	bucket := "uploads"
	operation := "download"
	expiry := 15 * time.Minute
	userID := "user-456"

	token, _, err := svc.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	// Try to validate with different file ID
	_, err = svc.ValidateSignedURL(token, "different-file", bucket)
	if err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature, got %v", err)
	}
}

func TestSignedURLService_ValidateWrongBucket(t *testing.T) {
	secret := []byte("test-secret-key-for-signing")
	svc := NewSignedURLService(secret)

	fileID := "file-123"
	bucket := "uploads"
	operation := "download"
	expiry := 15 * time.Minute
	userID := "user-456"

	token, _, err := svc.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	// Try to validate with different bucket
	_, err = svc.ValidateSignedURL(token, fileID, "different-bucket")
	if err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature, got %v", err)
	}
}

func TestSignedURLService_DifferentSecrets(t *testing.T) {
	secret1 := []byte("secret-1")
	secret2 := []byte("secret-2")

	svc1 := NewSignedURLService(secret1)
	svc2 := NewSignedURLService(secret2)

	fileID := "file-123"
	bucket := "uploads"
	operation := "download"
	expiry := 15 * time.Minute
	userID := "user-456"

	token, _, err := svc1.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	// Try to validate with different secret
	_, err = svc2.ValidateSignedURL(token, fileID, bucket)
	if err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature, got %v", err)
	}
}

func TestSignedURLService_ViewOperation(t *testing.T) {
	secret := []byte("test-secret-key-for-signing")
	svc := NewSignedURLService(secret)

	fileID := "file-123"
	bucket := "uploads"
	operation := "view"
	expiry := 15 * time.Minute
	userID := "user-456"

	token, _, err := svc.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	claims, err := svc.ValidateSignedURL(token, fileID, bucket)
	if err != nil {
		t.Fatalf("ValidateSignedURL failed: %v", err)
	}

	if claims.Operation != "view" {
		t.Errorf("Expected operation 'view', got %s", claims.Operation)
	}
}

func TestSignedURLService_EmptyUserID(t *testing.T) {
	secret := []byte("test-secret-key-for-signing")
	svc := NewSignedURLService(secret)

	fileID := "file-123"
	bucket := "uploads"
	operation := "download"
	expiry := 15 * time.Minute
	userID := "" // Empty user ID (unauthenticated access)

	token, _, err := svc.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	claims, err := svc.ValidateSignedURL(token, fileID, bucket)
	if err != nil {
		t.Fatalf("ValidateSignedURL failed: %v", err)
	}

	if claims.UserID != "" {
		t.Errorf("Expected empty user ID, got %s", claims.UserID)
	}
}
