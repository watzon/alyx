# Storage Bucket System

## Context

### Original Request
Build a comprehensive storage bucket system comparable to Appwrite. Users should be able to define buckets in their schema alongside collections with configuration options (access rules, filetype/filesize limits). Support multiple backends (filesystem, S3, S3-compatible like R2/minio). Add a new schema field type for connecting bucket files to records.

### Interview Summary
**Key Discussions**:
- File-record cardinality: Both single (`file`) and multi-file (`file[]`) field types
- Image transforms: Design API structure now, implement actual transforms in V2
- Access rules: Bucket-level CEL defaults + optional file-level overrides with separate `download` operation
- Backend config: Backends defined in `alyx.yml`, buckets reference them in `schema.yaml`
- Metadata storage: Dedicated `_alyx_files` system table + `_alyx_uploads` for TUS state
- Large uploads: Resumable/chunked uploads using TUS protocol from day one
- URL structure: `/api/files/{bucket}/{file_id}` with `/download`, `/view` variants
- Versioning: Design for it (version field), implement later
- Security: File type validation + compression in V1; encryption/antivirus hooks designed but deferred
- Signed URLs: Include temporary access URLs with expiry (default 15min)
- Orphan handling: `onDelete: cascade|keep` configuration on file fields

**Research Findings**:
- Alyx has 15 field types in `internal/schema/types.go` - new `file` type follows same pattern
- Handlers use DI via `handlers.New(db, schema, cfg, rules)` - storage handlers follow this
- Routes registered with `r.mux.HandleFunc("METHOD /path", r.wrap(handler))`
- CEL rules engine in `internal/rules/engine.go` - add `OpDownload` as 5th operation for buckets
- No file upload handling exists yet - clean slate for TUS implementation
- Appwrite uses bucket-first architecture, Content-Range chunks, permissions array

### Metis Review
**Identified Gaps** (addressed):
- Multi-file syntax clarified: `file[]` array notation
- Orphan handling: `onDelete` config on file field
- TUS state storage: `_alyx_uploads` system table for restart resilience
- Duplicate filenames: Reject with clear error (no overwrite, no auto-rename)
- Upload abandonment: 24h TTL, cleanup job
- Zero-byte files: Allow (valid use case for placeholders)
- MIME validation: Magic bytes, not Content-Type header
- Signed URL security: Use existing `auth.JWTConfig.Secret` for HMAC

---

## Work Objectives

### Core Objective
Implement a production-grade storage bucket system that integrates with Alyx's schema-first architecture, supporting pluggable storage backends and resumable uploads.

### Concrete Deliverables
- `buckets:` section in `schema.yaml` parser
- `file` and `file[]` field types
- `_alyx_files` and `_alyx_uploads` system tables
- `StorageBackend` interface with filesystem and S3 implementations
- TUS protocol endpoints for resumable uploads
- File CRUD endpoints with CEL access control
- Signed URL generation for temporary access
- Compression support (gzip/zstd)
- TypeScript SDK generation for file operations

### Definition of Done
- [ ] `make test` passes with new storage tests
- [ ] `make lint` passes
- [ ] Upload 100MB file via TUS, download matches checksum
- [ ] Switch backend via config only (no code changes)
- [ ] File field expansion returns metadata in API response
- [ ] Admin UI displays file fields with upload widget

### Must Have
- Schema-level bucket definitions with CEL access rules
- Filesystem and S3 backend implementations
- TUS protocol for resumable uploads
- File type validation via magic bytes
- Signed URLs with configurable expiry
- `file` and `file[]` field types with `onDelete` behavior

### Must NOT Have (Guardrails)
- NO file content stored in SQLite (metadata only)
- NO image transformation implementation (design API only)
- NO CDN integration
- NO virus scanning implementation (hook point only)
- NO deduplication/content-addressable storage
- NO folder hierarchy (flat namespace with `path` field for display)
- NO cross-bucket file moves (DELETE + CREATE pattern only)
- NO trusting Content-Type header for validation
- NO unlimited file sizes (enforce at upload start)
- NO `..` or null bytes in file paths

---

## Verification Strategy (MANDATORY)

### Test Decision
- **Infrastructure exists**: YES (Go standard testing)
- **User wants tests**: TDD
- **Framework**: Go `testing` package

### TDD Workflow
Each TODO follows RED-GREEN-REFACTOR:
1. **RED**: Write failing test first
2. **GREEN**: Implement minimum code to pass
3. **REFACTOR**: Clean up while keeping green

---

## Task Flow

```
Phase 1: Schema + Types
  1 → 2 → 3 → 4 (sequential - each builds on prior)

Phase 2: Storage Abstraction
  5 → 6 (interface first)
  5 → 7 (parallel with 6)

Phase 3: API Endpoints
  8 → 9 → 10 → 11 (sequential - CRUD then TUS)

Phase 4: Integration
  12, 13, 14 (parallel)
  → 15 (depends on all above)
```

## Parallelization

| Group | Tasks | Reason |
|-------|-------|--------|
| A | 6, 7 | Independent backend implementations |
| B | 12, 13, 14 | Independent integration points |

| Task | Depends On | Reason |
|------|------------|--------|
| 2 | 1 | Field type needs bucket definition |
| 3 | 2 | System tables need field type for foreign key |
| 4 | 3 | Migration depends on table definitions |
| 6, 7 | 5 | Backends implement interface |
| 8 | 5, 6 | Handlers need storage service |
| 9 | 8 | TUS extends file handlers |
| 10 | 8 | Signed URLs need file service |
| 11 | 8 | CEL rules extend handler pattern |
| 15 | 12, 13, 14 | Final integration test |

---

## TODOs

### Phase 1: Schema + Types

- [x] 1. Add bucket schema parsing

  **What to do**:
  - Add `Bucket` and `BucketRules` structs to `internal/schema/types.go`
  - Add `rawBucket` struct to `internal/schema/parser.go` for YAML unmarshaling
  - Implement `parseBucket()` function following `parseCollection()` pattern
  - Add `Buckets map[string]*Bucket` field to `Schema` struct
  - Validate bucket names (same rules as collections)
  - Support bucket configuration: `backend`, `max_file_size`, `max_total_size`, `allowed_types`, `compression`, `rules`

  **Must NOT do**:
  - DO NOT implement backend loading (just store backend name as string reference)
  - DO NOT validate backend existence yet (that's config layer)

  **Parallelizable**: NO (foundation task)

  **References**:
  - `internal/schema/types.go:Collection` struct - pattern for `Bucket` struct
  - `internal/schema/parser.go:parseCollection()` - pattern for `parseBucket()`
  - `internal/schema/parser.go:rawSchema` - add `Buckets` field
  - `internal/schema/schema_test.go` - test patterns

  **Acceptance Criteria**:
  - [ ] Test: `schema/bucket_test.go` - parse valid bucket YAML
  - [ ] Test: reject invalid bucket names (uppercase, special chars)
  - [ ] Test: validate `allowed_types` format
  - [ ] `go test ./internal/schema/...` → PASS

  **Commit**: YES
  - Message: `feat(schema): add bucket definition parsing`
  - Files: `internal/schema/schema.go`, `internal/schema/parser.go`, `internal/schema/bucket_test.go`
  - Pre-commit: `go test ./internal/schema/...`

---

- [x] 2. Add file field type

  **What to do**:
  - Add `FieldTypeFile = "file"` constant to `internal/schema/types.go`
  - Implement `SQLiteType()` → `TEXT` (stores UUID reference)
  - Implement `GoType()` → `string` for single, `[]string` for array
  - Implement `TypeScriptType()` → `string` for single, `string[]` for array
  - Add `FileConfig` struct: `Bucket string`, `MaxSize int64`, `AllowedTypes []string`, `OnDelete OnDeleteAction`
  - Add `File *FileConfig` field to `Field` struct
  - Detect array syntax: `file[]` sets `Multiple: true` internally
  - Validate: `file` field must have `bucket` specified
  - Validate: referenced bucket must exist in schema

  **Must NOT do**:
  - DO NOT implement actual file storage logic
  - DO NOT add migration logic yet

  **Parallelizable**: NO (depends on 1)

  **References**:
  - `internal/schema/types.go:FieldType*` constants - follow exact pattern
  - `internal/schema/types.go:SelectConfig` - pattern for `FileConfig`
  - `internal/schema/types.go:SQLiteType()`, `GoType()`, `TypeScriptType()` - add cases
  - `internal/schema/parser.go:parseField()` - add file config parsing

  **Acceptance Criteria**:
  - [ ] Test: `file` type returns correct SQLite/Go/TypeScript types
  - [ ] Test: `file[]` array syntax detected and handled
  - [ ] Test: validation fails if bucket not specified
  - [ ] Test: validation fails if bucket doesn't exist in schema
  - [ ] `go test ./internal/schema/...` → PASS

  **Commit**: YES
  - Message: `feat(schema): add file field type with bucket reference`
  - Files: `internal/schema/types.go`, `internal/schema/parser.go`
  - Pre-commit: `go test ./internal/schema/...`

---

- [x] 3. Define system tables for files and uploads

  **What to do**:
  - Create `internal/storage/tables.go` with table definitions
  - Define `_alyx_files` table: `id TEXT PRIMARY KEY`, `bucket TEXT NOT NULL`, `name TEXT NOT NULL`, `path TEXT NOT NULL`, `mime_type TEXT NOT NULL`, `size INTEGER NOT NULL`, `checksum TEXT`, `compressed BOOLEAN DEFAULT FALSE`, `compression_type TEXT`, `original_size INTEGER`, `metadata TEXT` (JSON), `version INTEGER DEFAULT 1`, `created_at TEXT`, `updated_at TEXT`
  - Define `_alyx_uploads` table (TUS state): `id TEXT PRIMARY KEY`, `bucket TEXT NOT NULL`, `filename TEXT`, `size INTEGER NOT NULL`, `offset INTEGER DEFAULT 0`, `metadata TEXT`, `expires_at TEXT`, `created_at TEXT`
  - Add unique constraint on `(_alyx_files.bucket, _alyx_files.path)`
  - Add index on `_alyx_files.bucket`
  - Add index on `_alyx_uploads.expires_at` (for cleanup queries)

  **Must NOT do**:
  - DO NOT create the tables in database yet (that's migration task)
  - DO NOT implement any CRUD operations

  **Parallelizable**: NO (depends on 2)

  **References**:
  - `internal/database/migrations.go` - existing system table patterns
  - `internal/webhooks/store.go` - store pattern to follow
  - `internal/scheduler/store.go` - store pattern to follow

  **Acceptance Criteria**:
  - [ ] Test: table SQL generates correctly
  - [ ] Test: constraints and indexes defined
  - [ ] `go test ./internal/storage/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): define system tables for files and uploads`
  - Files: `internal/storage/tables.go`, `internal/storage/tables_test.go`
  - Pre-commit: `go test ./internal/storage/...`

---

- [ ] 4. Add storage table migrations

  **What to do**:
  - Add migration to create `_alyx_files` and `_alyx_uploads` tables
  - Follow existing migration pattern in `internal/database/migrations.go`
  - Ensure migrations run on startup with other system migrations
  - Tables should be created regardless of schema (system tables)

  **Must NOT do**:
  - DO NOT modify existing migrations
  - DO NOT add data to tables

  **Parallelizable**: NO (depends on 3)

  **References**:
  - `internal/database/migrations.go` - migration execution pattern
  - `internal/database/database.go:Initialize()` - where migrations run
  - `internal/storage/tables.go` - table definitions from task 3

  **Acceptance Criteria**:
  - [ ] Test: fresh database has `_alyx_files` and `_alyx_uploads` tables
  - [ ] Test: migration is idempotent (running twice doesn't fail)
  - [ ] `go test ./internal/database/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): add migrations for system tables`
  - Files: `internal/database/migrations.go`
  - Pre-commit: `go test ./internal/database/...`

---

### Phase 2: Storage Abstraction

- [ ] 5. Create storage backend interface

  **What to do**:
  - Create `internal/storage/backend.go` with `Backend` interface
  - Interface methods: `Put(ctx, bucket, key string, r io.Reader, size int64) error`, `Get(ctx, bucket, key string) (io.ReadCloser, error)`, `Delete(ctx, bucket, key string) error`, `Exists(ctx, bucket, key string) (bool, error)`
  - Create `BackendConfig` struct for common settings
  - Create `NewBackend(cfg BackendConfig) (Backend, error)` factory function
  - Add compression wrapper: `CompressedBackend` that wraps any backend

  **Must NOT do**:
  - DO NOT implement bucket creation/listing in interface (buckets are schema-defined)
  - DO NOT add backend-specific methods

  **Parallelizable**: NO (foundation for 6, 7)

  **References**:
  - `internal/database/database.go:DB` interface pattern
  - `io.Reader`, `io.ReadCloser` for streaming interface
  - `context.Context` for cancellation

  **Acceptance Criteria**:
  - [ ] Test: interface compiles with mock implementation
  - [ ] Test: `CompressedBackend` wraps backend correctly
  - [ ] `go test ./internal/storage/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): define backend interface and compression wrapper`
  - Files: `internal/storage/backend.go`, `internal/storage/backend_test.go`
  - Pre-commit: `go test ./internal/storage/...`

---

- [ ] 6. Implement filesystem backend

  **What to do**:
  - Create `internal/storage/filesystem.go` implementing `Backend`
  - Configure base path from config: `storage.filesystem.path`
  - Organize files as: `{base_path}/{bucket}/{key}`
  - Create directories automatically on `Put`
  - Implement proper file locking for concurrent access
  - Sanitize all paths: reject `..`, null bytes, absolute paths
  - Use `filepath.Clean()` and explicit validation

  **Must NOT do**:
  - DO NOT create buckets as directories on startup (lazy creation on first write)
  - DO NOT implement compression here (use wrapper)

  **Parallelizable**: YES (with 7)

  **References**:
  - `internal/storage/backend.go` - interface from task 5
  - `os.Create`, `os.Open`, `os.Remove` - file operations
  - `filepath.Clean`, `filepath.Join` - path handling

  **Acceptance Criteria**:
  - [ ] Test: Put file, Get returns same content
  - [ ] Test: Delete removes file
  - [ ] Test: Exists returns correct status
  - [ ] Test: path traversal attempts rejected
  - [ ] Test: concurrent access works correctly
  - [ ] `go test ./internal/storage/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): implement filesystem backend`
  - Files: `internal/storage/filesystem.go`, `internal/storage/filesystem_test.go`
  - Pre-commit: `go test ./internal/storage/...`

---

- [ ] 7. Implement S3 backend

  **What to do**:
  - Create `internal/storage/s3.go` implementing `Backend`
  - Use AWS SDK v2: `github.com/aws/aws-sdk-go-v2`
  - Configure from config: `storage.s3.endpoint`, `storage.s3.region`, `storage.s3.access_key`, `storage.s3.secret_key`, `storage.s3.bucket_prefix`
  - Support S3-compatible services (minio, R2) via custom endpoint
  - Key format: `{bucket_prefix}/{bucket}/{key}`
  - Handle multipart uploads for large files internally

  **Must NOT do**:
  - DO NOT create S3 buckets programmatically
  - DO NOT implement presigned URLs here (that's handler layer)

  **Parallelizable**: YES (with 6)

  **References**:
  - `internal/storage/backend.go` - interface from task 5
  - AWS SDK v2 S3 client documentation
  - `internal/config/config.go` - add S3 config section

  **Acceptance Criteria**:
  - [ ] Test: Put/Get/Delete with localstack or minio
  - [ ] Test: custom endpoint configuration works
  - [ ] Test: large file multipart upload works
  - [ ] `go test ./internal/storage/...` → PASS (may need integration test tag)

  **Commit**: YES
  - Message: `feat(storage): implement S3 backend with S3-compatible support`
  - Files: `internal/storage/s3.go`, `internal/storage/s3_test.go`, `internal/config/config.go`
  - Pre-commit: `go test ./internal/storage/...`

---

### Phase 3: API Endpoints

- [ ] 8. Implement file service and CRUD handlers

  **What to do**:
  - Create `internal/storage/service.go` with `Service` struct
  - Inject: backend, db, schema, config
  - Methods: `Upload()`, `Download()`, `GetMetadata()`, `Delete()`, `List()`
  - Create `internal/server/handlers/files.go` with `FileHandlers` struct
  - Follow existing handler pattern: dependency injection via `NewFileHandlers()`
  - Implement endpoints:
    - `POST /api/files/{bucket}` - Upload file (multipart)
    - `GET /api/files/{bucket}` - List files in bucket
    - `GET /api/files/{bucket}/{id}` - Get file metadata
    - `GET /api/files/{bucket}/{id}/download` - Download file content
    - `GET /api/files/{bucket}/{id}/view` - View inline (no Content-Disposition: attachment)
    - `DELETE /api/files/{bucket}/{id}` - Delete file
  - Register routes in `internal/server/router.go`
  - Validate MIME type using magic bytes (`http.DetectContentType` + enhancement)
  - Enforce file size limits at upload start
  - Store metadata in `_alyx_files` table

  **Must NOT do**:
  - DO NOT implement TUS endpoints here (separate task)
  - DO NOT implement signed URLs here (separate task)
  - DO NOT handle file field updates (that's integration task)

  **Parallelizable**: NO (depends on 5, 6)

  **References**:
  - `internal/server/handlers/handlers.go` - CRUD pattern
  - `internal/server/handlers/auth.go` - service injection pattern
  - `internal/server/router.go:setupRoutes()` - route registration
  - `r.FormFile()`, `multipart.FileHeader` - file upload handling

  **Acceptance Criteria**:
  - [ ] Test: upload file, verify in `_alyx_files` table
  - [ ] Test: download returns same content as uploaded
  - [ ] Test: delete removes file and metadata
  - [ ] Test: list returns correct files for bucket
  - [ ] Test: MIME type validation rejects mismatched content
  - [ ] Test: file size limit enforced
  - [ ] `go test ./internal/storage/... ./internal/server/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): implement file service and CRUD handlers`
  - Files: `internal/storage/service.go`, `internal/server/handlers/files.go`, `internal/server/router.go`
  - Pre-commit: `go test ./internal/storage/... ./internal/server/...`

---

- [ ] 9. Implement TUS protocol endpoints

  **What to do**:
  - Add TUS 1.0.0 protocol support to file handlers
  - Endpoints:
    - `POST /api/files/{bucket}/tus` - Create upload (returns upload URL)
    - `HEAD /api/files/{bucket}/tus/{upload_id}` - Get upload offset
    - `PATCH /api/files/{bucket}/tus/{upload_id}` - Upload chunk
    - `DELETE /api/files/{bucket}/tus/{upload_id}` - Cancel upload
  - Store upload state in `_alyx_uploads` table
  - On final chunk: move to `_alyx_files`, delete from `_alyx_uploads`
  - Support `Upload-Length`, `Upload-Offset`, `Upload-Metadata` headers
  - Default chunk size: 5MB (configurable)
  - Store partial uploads in temp location
  - Implement upload expiry: 24h default (configurable)

  **Must NOT do**:
  - DO NOT implement TUS extensions (checksum, concatenation, etc.) in V1
  - DO NOT stream directly to S3 (buffer through server)

  **Parallelizable**: NO (depends on 8)

  **References**:
  - TUS protocol spec: https://tus.io/protocols/resumable-upload.html
  - `internal/storage/tables.go:_alyx_uploads` - state storage
  - `internal/server/handlers/files.go` - extend handlers

  **Acceptance Criteria**:
  - [ ] Test: create upload returns valid upload URL
  - [ ] Test: upload 10MB file in 3 chunks, verify complete
  - [ ] Test: resume after disconnect (restart offset query)
  - [ ] Test: cancel deletes partial file
  - [ ] Test: expired uploads cleaned up
  - [ ] `go test ./internal/storage/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): implement TUS resumable upload protocol`
  - Files: `internal/storage/tus.go`, `internal/server/handlers/files.go`
  - Pre-commit: `go test ./internal/storage/...`

---

- [ ] 10. Implement signed URLs

  **What to do**:
  - Add `GET /api/files/{bucket}/{id}/sign` endpoint
  - Parameters: `expiry` (duration, default 15m), `operation` (download/view)
  - Generate HMAC-signed token using `auth.JWTConfig.Secret`
  - Token contains: `file_id`, `bucket`, `operation`, `expires_at`, `user_id`
  - Add `GET /api/files/{bucket}/{id}/download?token=...` handling
  - Validate: token signature, expiry, file existence
  - Return 404 if file deleted (not 403)
  - Optional: allow signed URL generation for unauthenticated access

  **Must NOT do**:
  - DO NOT allow indefinite expiry
  - DO NOT expose internal storage paths

  **Parallelizable**: NO (depends on 8)

  **References**:
  - `internal/auth/jwt.go` - JWT/HMAC patterns
  - `crypto/hmac` - signature generation
  - `internal/server/handlers/files.go` - extend handlers

  **Acceptance Criteria**:
  - [ ] Test: generate signed URL, access without auth works
  - [ ] Test: expired URL returns 401
  - [ ] Test: tampered URL returns 401
  - [ ] Test: deleted file with valid URL returns 404
  - [ ] `go test ./internal/storage/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): implement signed URLs with expiry`
  - Files: `internal/storage/signed.go`, `internal/server/handlers/files.go`
  - Pre-commit: `go test ./internal/storage/...`

---

- [ ] 11. Implement CEL access rules for buckets

  **What to do**:
  - Add `OpDownload` to `internal/rules/engine.go` as 5th operation
  - Update `CheckAccess()` to handle bucket rules
  - Bucket rules context variables:
    - `auth.*` - same as collections
    - `file.*` - file metadata (id, name, mime_type, size, bucket)
    - `request.*` - same as collections
  - Parse bucket rules from schema into rules engine
  - Support file-level rule overrides (stored in `_alyx_files.metadata`)
  - When `file_security: true` on bucket, check file rules first, then bucket rules

  **Must NOT do**:
  - DO NOT modify collection rule behavior
  - DO NOT add bucket-specific CEL functions

  **Parallelizable**: NO (depends on 8)

  **References**:
  - `internal/rules/engine.go` - existing CEL implementation
  - `internal/server/handlers/handlers.go:checkAccess()` - integration pattern
  - `internal/schema/schema.go:BucketRules` - rule definitions

  **Acceptance Criteria**:
  - [ ] Test: bucket create rule blocks unauthorized upload
  - [ ] Test: bucket read rule blocks unauthorized metadata access
  - [ ] Test: bucket download rule blocks unauthorized download (separate from read)
  - [ ] Test: file-level override takes precedence when enabled
  - [ ] `go test ./internal/rules/... ./internal/storage/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): implement CEL access rules for buckets`
  - Files: `internal/rules/engine.go`, `internal/storage/service.go`
  - Pre-commit: `go test ./internal/rules/... ./internal/storage/...`

---

### Phase 4: Integration

- [ ] 12. Integrate file field with record CRUD

  **What to do**:
  - Update `internal/server/handlers/handlers.go` to handle file fields
  - On create/update with file field: validate file exists in referenced bucket
  - On `?expand=field_name`: include file metadata in response (not just ID)
  - On delete with `onDelete: cascade`: delete referenced files
  - On update file field: handle orphan based on `onDelete` setting
  - Prevent setting file field to non-existent file ID

  **Must NOT do**:
  - DO NOT auto-upload files on record create (files must exist first)
  - DO NOT allow cross-bucket file references

  **Parallelizable**: YES (with 13, 14)

  **References**:
  - `internal/server/handlers/handlers.go:CreateDocument()` - integration point
  - `internal/server/handlers/handlers.go:handleExpand()` - expand pattern
  - `internal/schema/types.go:FileConfig` - onDelete setting

  **Acceptance Criteria**:
  - [ ] Test: create record with valid file ID succeeds
  - [ ] Test: create record with invalid file ID fails
  - [ ] Test: expand returns file metadata
  - [ ] Test: delete record cascades to file when configured
  - [ ] `go test ./internal/server/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): integrate file field with record CRUD`
  - Files: `internal/server/handlers/handlers.go`
  - Pre-commit: `go test ./internal/server/...`

---

- [ ] 13. Add file operations to TypeScript SDK generator

  **What to do**:
  - Update SDK generator to include file operations
  - Generate `storage.upload(bucket, file, options)` method
  - Generate `storage.download(bucket, fileId)` method
  - Generate `storage.getUrl(bucket, fileId, options)` for signed URLs
  - Generate `storage.delete(bucket, fileId)` method
  - Generate `storage.list(bucket, options)` method
  - Add TUS client wrapper for resumable uploads
  - File field types generate as `string` (ID reference)

  **Must NOT do**:
  - DO NOT implement actual TUS client (reference tus-js-client)
  - DO NOT generate image transform helpers (V2)

  **Parallelizable**: YES (with 12, 14)

  **References**:
  - `internal/codegen/typescript.go` - SDK generation
  - `generated/` - output directory
  - Appwrite SDK patterns for reference

  **Acceptance Criteria**:
  - [ ] Generated SDK compiles without TypeScript errors
  - [ ] File operations present in generated client
  - [ ] File field types correct in generated types
  - [ ] `make generate` succeeds

  **Commit**: YES
  - Message: `feat(codegen): add file operations to TypeScript SDK`
  - Files: `internal/codegen/typescript.go`
  - Pre-commit: `make generate`

---

- [ ] 14. Add upload abandoned cleanup job

  **What to do**:
  - Create `internal/storage/cleanup.go` with cleanup service
  - Query `_alyx_uploads` for expired entries (`expires_at < now`)
  - Delete partial files from storage backend
  - Delete entries from `_alyx_uploads` table
  - Run as background goroutine with configurable interval (default: 1 hour)
  - Log cleanup statistics

  **Must NOT do**:
  - DO NOT delete completed uploads (those move to `_alyx_files`)
  - DO NOT run on startup (wait for first interval)

  **Parallelizable**: YES (with 12, 13)

  **References**:
  - `internal/scheduler/scheduler.go` - background job pattern
  - `internal/storage/service.go` - service integration
  - `internal/storage/tables.go:_alyx_uploads` - table structure

  **Acceptance Criteria**:
  - [ ] Test: expired uploads cleaned up after interval
  - [ ] Test: active uploads not deleted
  - [ ] Test: partial files removed from storage
  - [ ] `go test ./internal/storage/...` → PASS

  **Commit**: YES
  - Message: `feat(storage): add cleanup job for abandoned uploads`
  - Files: `internal/storage/cleanup.go`, `internal/storage/cleanup_test.go`
  - Pre-commit: `go test ./internal/storage/...`

---

- [ ] 15. End-to-end integration test

  **What to do**:
  - Create `internal/integration/storage_test.go`
  - Test complete flow:
    1. Parse schema with bucket and collection with file field
    2. Create bucket via schema
    3. Upload file via TUS (multiple chunks)
    4. Create record referencing file
    5. Query record with expand, verify file metadata
    6. Generate signed URL, access without auth
    7. Delete record, verify cascade behavior
    8. Switch to S3 backend via config, repeat upload/download
  - Test error cases:
    - Upload to non-existent bucket
    - Reference non-existent file
    - Exceed file size limit
    - Invalid MIME type

  **Must NOT do**:
  - DO NOT test Admin UI (separate task)
  - DO NOT require external S3 (use minio in CI)

  **Parallelizable**: NO (final integration)

  **References**:
  - `internal/integration/` - existing integration tests
  - `internal/integration/events_test.go` - test setup pattern
  - docker-compose for minio setup

  **Acceptance Criteria**:
  - [ ] All integration tests pass
  - [ ] Test with filesystem backend passes
  - [ ] Test with S3 backend passes (minio)
  - [ ] `go test ./internal/integration/... -tags=integration` → PASS

  **Commit**: YES
  - Message: `test(storage): add end-to-end integration tests`
  - Files: `internal/integration/storage_test.go`
  - Pre-commit: `go test ./internal/integration/... -tags=integration`

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 1 | `feat(schema): add bucket definition parsing` | schema/* | `go test ./internal/schema/...` |
| 2 | `feat(schema): add file field type with bucket reference` | schema/* | `go test ./internal/schema/...` |
| 3 | `feat(storage): define system tables for files and uploads` | storage/* | `go test ./internal/storage/...` |
| 4 | `feat(storage): add migrations for system tables` | database/* | `go test ./internal/database/...` |
| 5 | `feat(storage): define backend interface and compression wrapper` | storage/* | `go test ./internal/storage/...` |
| 6 | `feat(storage): implement filesystem backend` | storage/* | `go test ./internal/storage/...` |
| 7 | `feat(storage): implement S3 backend with S3-compatible support` | storage/*, config/* | `go test ./internal/storage/...` |
| 8 | `feat(storage): implement file service and CRUD handlers` | storage/*, handlers/*, router.go | `go test ./internal/storage/... ./internal/server/...` |
| 9 | `feat(storage): implement TUS resumable upload protocol` | storage/*, handlers/* | `go test ./internal/storage/...` |
| 10 | `feat(storage): implement signed URLs with expiry` | storage/*, handlers/* | `go test ./internal/storage/...` |
| 11 | `feat(storage): implement CEL access rules for buckets` | rules/*, storage/* | `go test ./internal/rules/... ./internal/storage/...` |
| 12 | `feat(storage): integrate file field with record CRUD` | handlers/* | `go test ./internal/server/...` |
| 13 | `feat(codegen): add file operations to TypeScript SDK` | codegen/* | `make generate` |
| 14 | `feat(storage): add cleanup job for abandoned uploads` | storage/* | `go test ./internal/storage/...` |
| 15 | `test(storage): add end-to-end integration tests` | integration/* | `go test ./internal/integration/... -tags=integration` |

---

## Success Criteria

### Verification Commands
```bash
# All tests pass
make test                     # Expected: PASS

# Lint passes
make lint                     # Expected: No errors

# Generate SDK
make generate                 # Expected: Success

# Manual verification - TUS upload
curl -X POST http://localhost:8080/api/files/avatars/tus \
  -H "Upload-Length: 104857600" \
  -H "Upload-Metadata: filename YXZhdGFyLnBuZw==" \
  -H "Authorization: Bearer $TOKEN"
# Expected: 201 with Location header

# Manual verification - Download
curl http://localhost:8080/api/files/avatars/{file_id}/download \
  -H "Authorization: Bearer $TOKEN" \
  -o downloaded_file
# Expected: 200 with file content

# Manual verification - Signed URL
curl http://localhost:8080/api/files/avatars/{file_id}/sign \
  -H "Authorization: Bearer $TOKEN"
# Expected: 200 with { "url": "...", "expires_at": "..." }
```

### Final Checklist
- [ ] All 15 tasks completed
- [ ] All "Must Have" features present
- [ ] All "Must NOT Have" guardrails enforced
- [ ] All tests pass (unit + integration)
- [ ] SDK generates correctly
- [ ] Documentation updated (if applicable)
