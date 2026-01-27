package codegen

import (
	"strings"
	"testing"

	"github.com/watzon/alyx/internal/schema"
)

func TestTypeScriptGenerator_StorageClient(t *testing.T) {
	cfg := &Config{
		ServerURL: "http://localhost:8090",
	}
	gen := NewTypeScriptGenerator(cfg)

	// Create schema with buckets
	s := &schema.Schema{
		Collections: map[string]*schema.Collection{},
		Buckets: map[string]*schema.Bucket{
			"uploads": {
				Name:    "uploads",
				Backend: "filesystem",
			},
			"avatars": {
				Name:    "avatars",
				Backend: "s3",
			},
		},
	}

	files, err := gen.Generate(s)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Find client.ts
	var clientContent string
	for _, f := range files {
		if f.Path == "client.ts" {
			clientContent = f.Content
			break
		}
	}

	if clientContent == "" {
		t.Fatal("client.ts not generated")
	}

	// Test 1: StorageClient class exists
	if !strings.Contains(clientContent, "export class StorageClient") {
		t.Error("StorageClient class not found in generated client")
	}

	// Test 2: FileMetadata interface exists
	if !strings.Contains(clientContent, "export interface FileMetadata") {
		t.Error("FileMetadata interface not found")
	}

	// Test 3: UploadOptions interface exists
	if !strings.Contains(clientContent, "export interface UploadOptions") {
		t.Error("UploadOptions interface not found")
	}

	// Test 4: SignedUrlOptions interface exists
	if !strings.Contains(clientContent, "export interface SignedUrlOptions") {
		t.Error("SignedUrlOptions interface not found")
	}

	// Test 5: TUSOptions interface exists
	if !strings.Contains(clientContent, "export interface TUSOptions") {
		t.Error("TUSOptions interface not found")
	}

	// Test 6: TUSUpload interface exists
	if !strings.Contains(clientContent, "export interface TUSUpload") {
		t.Error("TUSUpload interface not found")
	}

	// Test 7: SignedUrl interface exists
	if !strings.Contains(clientContent, "export interface SignedUrl") {
		t.Error("SignedUrl interface not found")
	}

	// Test 8: upload method exists
	if !strings.Contains(clientContent, "async upload(") {
		t.Error("upload method not found")
	}

	// Test 9: download method exists
	if !strings.Contains(clientContent, "async download(") {
		t.Error("download method not found")
	}

	// Test 10: getUrl method exists
	if !strings.Contains(clientContent, "async getUrl(") {
		t.Error("getUrl method not found")
	}

	// Test 11: delete method exists
	if !strings.Contains(clientContent, "async delete(") {
		t.Error("delete method not found")
	}

	// Test 12: list method exists
	if !strings.Contains(clientContent, "async list(") {
		t.Error("list method not found")
	}

	// Test 13: uploadResumable method exists
	if !strings.Contains(clientContent, "uploadResumable(") {
		t.Error("uploadResumable method not found")
	}

	// Test 14: AlyxClient has storage property
	if !strings.Contains(clientContent, "storage = new StorageClient(this)") {
		t.Error("AlyxClient.storage property not found")
	}
}

func TestTypeScriptGenerator_FileFieldType(t *testing.T) {
	cfg := &Config{
		ServerURL: "http://localhost:8090",
	}
	gen := NewTypeScriptGenerator(cfg)

	// Create schema with file field
	postsCollection := &schema.Collection{
		Name: "posts",
		Fields: map[string]*schema.Field{
			"id": {
				Name:    "id",
				Type:    schema.FieldTypeUUID,
				Primary: true,
			},
			"title": {
				Name:     "title",
				Type:     schema.FieldTypeString,
				Nullable: false,
			},
			"cover_image": {
				Name:     "cover_image",
				Type:     schema.FieldTypeFile,
				Nullable: false,
				File: &schema.FileConfig{
					Bucket: "uploads",
				},
			},
			"attachment": {
				Name:     "attachment",
				Type:     schema.FieldTypeFile,
				Nullable: true,
				File: &schema.FileConfig{
					Bucket: "uploads",
				},
			},
		},
	}
	postsCollection.SetFieldOrder([]string{"id", "title", "cover_image", "attachment"})

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": postsCollection,
		},
		Buckets: map[string]*schema.Bucket{
			"uploads": {
				Name:    "uploads",
				Backend: "filesystem",
			},
		},
	}

	files, err := gen.Generate(s)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Find types.ts
	var typesContent string
	for _, f := range files {
		if f.Path == "types.ts" {
			typesContent = f.Content
			break
		}
	}

	if typesContent == "" {
		t.Fatal("types.ts not generated")
	}

	// Test 1: File field generates as string (non-nullable)
	if !strings.Contains(typesContent, "cover_image: string;") {
		t.Error("File field not generated as string type")
		t.Logf("Types content:\n%s", typesContent)
	}

	// Test 2: Nullable file field generates as string | null
	if !strings.Contains(typesContent, "attachment?: string") {
		t.Error("Nullable file field not generated correctly")
		t.Logf("Types content:\n%s", typesContent)
	}
}

func TestTypeScriptGenerator_NoStorageWhenNoBuckets(t *testing.T) {
	cfg := &Config{
		ServerURL: "http://localhost:8090",
	}
	gen := NewTypeScriptGenerator(cfg)

	// Create schema without buckets
	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"users": {
				Name: "users",
				Fields: map[string]*schema.Field{
					"id": {
						Name:    "id",
						Type:    schema.FieldTypeUUID,
						Primary: true,
					},
					"email": {
						Name:     "email",
						Type:     schema.FieldTypeEmail,
						Nullable: false,
					},
				},
			},
		},
		Buckets: map[string]*schema.Bucket{},
	}

	files, err := gen.Generate(s)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Find client.ts
	var clientContent string
	for _, f := range files {
		if f.Path == "client.ts" {
			clientContent = f.Content
			break
		}
	}

	if clientContent == "" {
		t.Fatal("client.ts not generated")
	}

	// StorageClient should NOT be generated when no buckets
	if strings.Contains(clientContent, "export class StorageClient") {
		t.Error("StorageClient should not be generated when schema has no buckets")
	}

	// AlyxClient should NOT have storage property
	if strings.Contains(clientContent, "storage = new StorageClient") {
		t.Error("AlyxClient.storage should not exist when schema has no buckets")
	}
}
