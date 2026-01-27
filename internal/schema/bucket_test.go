package schema

import (
	"strings"
	"testing"
)

func TestParseBucket_Valid(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

buckets:
  avatars:
    backend: local
    max_file_size: 5242880
    max_total_size: 104857600
    allowed_types:
      - image/jpeg
      - image/png
      - image/webp
    compression: true
    rules:
      create: "auth.id != null"
      read: "true"
      update: "auth.id == file.user_id"
      delete: "auth.id == file.user_id"
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	bucket, ok := schema.Buckets["avatars"]
	if !ok {
		t.Fatal("avatars bucket not found")
	}

	if bucket.Name != "avatars" {
		t.Errorf("expected name 'avatars', got %q", bucket.Name)
	}

	if bucket.Backend != "local" {
		t.Errorf("expected backend 'local', got %q", bucket.Backend)
	}

	if bucket.MaxFileSize != 5242880 {
		t.Errorf("expected max_file_size 5242880, got %d", bucket.MaxFileSize)
	}

	if bucket.MaxTotalSize != 104857600 {
		t.Errorf("expected max_total_size 104857600, got %d", bucket.MaxTotalSize)
	}

	expectedTypes := []string{"image/jpeg", "image/png", "image/webp"}
	if len(bucket.AllowedTypes) != len(expectedTypes) {
		t.Errorf("expected %d allowed types, got %d", len(expectedTypes), len(bucket.AllowedTypes))
	}
	for i, expected := range expectedTypes {
		if bucket.AllowedTypes[i] != expected {
			t.Errorf("expected allowed_types[%d] = %q, got %q", i, expected, bucket.AllowedTypes[i])
		}
	}

	if !bucket.Compression {
		t.Error("expected compression to be true")
	}

	if bucket.Rules == nil {
		t.Fatal("expected rules to be set")
	}

	if bucket.Rules.Create != "auth.id != null" {
		t.Errorf("expected create rule 'auth.id != null', got %q", bucket.Rules.Create)
	}

	if bucket.Rules.Read != "true" {
		t.Errorf("expected read rule 'true', got %q", bucket.Rules.Read)
	}

	if bucket.Rules.Update != "auth.id == file.user_id" {
		t.Errorf("expected update rule 'auth.id == file.user_id', got %q", bucket.Rules.Update)
	}

	if bucket.Rules.Delete != "auth.id == file.user_id" {
		t.Errorf("expected delete rule 'auth.id == file.user_id', got %q", bucket.Rules.Delete)
	}
}

func TestParseBucket_MinimalConfig(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

buckets:
  files:
    backend: local
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	bucket, ok := schema.Buckets["files"]
	if !ok {
		t.Fatal("files bucket not found")
	}

	if bucket.Backend != "local" {
		t.Errorf("expected backend 'local', got %q", bucket.Backend)
	}

	// Defaults should be zero values
	if bucket.MaxFileSize != 0 {
		t.Errorf("expected max_file_size 0 (unlimited), got %d", bucket.MaxFileSize)
	}

	if bucket.MaxTotalSize != 0 {
		t.Errorf("expected max_total_size 0 (unlimited), got %d", bucket.MaxTotalSize)
	}

	if len(bucket.AllowedTypes) != 0 {
		t.Errorf("expected no allowed_types restrictions, got %d", len(bucket.AllowedTypes))
	}

	if bucket.Compression {
		t.Error("expected compression to be false by default")
	}
}

func TestValidation_InvalidBucketName(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
		wantError  bool
	}{
		{"uppercase", "MyBucket", true},
		{"starts with number", "1bucket", true},
		{"special chars", "my-bucket", true},
		{"spaces", "my bucket", true},
		{"valid lowercase", "my_bucket", false},
		{"valid with numbers", "bucket_123", false},
		{"reserved prefix", "_alyx_bucket", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

buckets:
  ` + tt.bucketName + `:
    backend: local
`
			_, err := Parse([]byte(yaml))
			if tt.wantError && err == nil {
				t.Errorf("expected validation error for bucket name %q", tt.bucketName)
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error for valid bucket name %q: %v", tt.bucketName, err)
			}
		})
	}
}

func TestValidation_AllowedTypesFormat(t *testing.T) {
	tests := []struct {
		name      string
		mimeType  string
		wantError bool
	}{
		{"valid image/jpeg", "image/jpeg", false},
		{"valid application/pdf", "application/pdf", false},
		{"valid wildcard image/*", "image/*", false},
		{"invalid no slash", "jpeg", true},
		{"invalid multiple slashes", "image/jpeg/extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

buckets:
  files:
    backend: local
    allowed_types:
      - ` + tt.mimeType + `
`
			_, err := Parse([]byte(yaml))
			if tt.wantError && err == nil {
				t.Errorf("expected validation error for mime type %q", tt.mimeType)
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error for valid mime type %q: %v", tt.mimeType, err)
			}
		})
	}
}

func TestValidation_MissingBackend(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

buckets:
  files:
    max_file_size: 1048576
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for missing backend")
	}
	if err != nil && !strings.Contains(err.Error(), "backend") {
		t.Errorf("expected error about missing backend, got: %v", err)
	}
}

func TestValidation_NegativeFileSizes(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value int
	}{
		{"negative max_file_size", "max_file_size", -1},
		{"negative max_total_size", "max_total_size", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

buckets:
  files:
    backend: local
    ` + tt.field + `: ` + string(rune(tt.value)) + `
`
			_, err := Parse([]byte(yaml))
			if err == nil {
				t.Errorf("expected validation error for negative %s", tt.field)
			}
		})
	}
}

func TestParseBucket_MultipleBuckets(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

buckets:
  avatars:
    backend: local
    allowed_types:
      - image/*
  documents:
    backend: s3
    allowed_types:
      - application/pdf
      - application/msword
  videos:
    backend: local
    max_file_size: 104857600
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	if len(schema.Buckets) != 3 {
		t.Errorf("expected 3 buckets, got %d", len(schema.Buckets))
	}

	if _, ok := schema.Buckets["avatars"]; !ok {
		t.Error("avatars bucket not found")
	}

	if _, ok := schema.Buckets["documents"]; !ok {
		t.Error("documents bucket not found")
	}

	if _, ok := schema.Buckets["videos"]; !ok {
		t.Error("videos bucket not found")
	}

	// Verify backend names are stored correctly
	if schema.Buckets["avatars"].Backend != "local" {
		t.Errorf("expected avatars backend 'local', got %q", schema.Buckets["avatars"].Backend)
	}

	if schema.Buckets["documents"].Backend != "s3" {
		t.Errorf("expected documents backend 's3', got %q", schema.Buckets["documents"].Backend)
	}
}
