# Draft: Storage Bucket System

## Requirements (confirmed)
- Schema-level bucket definitions (alongside collections)
- Configuration options: access rules, filetype limits, filesize limits
- Multiple storage backends: filesystem, S3, S3-compatible (R2, minio)
- New schema field type for connecting files to records
- **File-record cardinality**: Both single-file and multi-file field types
- **Image transforms**: Design API for future transforms, implement later
- **Access rules**: Bucket-level defaults + optional file-level overrides

## Technical Decisions
- **Backend config**: Backends defined in `alyx.yml`, buckets reference them in `schema.yaml`
- **Metadata storage**: Dedicated `_alyx_files` system table (id, bucket, name, mime, size, checksum, metadata)
- **Large uploads**: Resumable uploads from the start (TUS protocol or custom chunked)
- **URL structure**: `/api/files/{bucket}/{file_id}` with `/download`, `/view` variants
- **Versioning**: Design for it (version field in schema), implement later
- **Field type name**: `file` (single) and `file[]` or `files` for multiple

## Research Findings

### Schema System (Alyx codebase)
- **15 field types** in `types.go` with SQLite/Go/TypeScript/Python mappings
- **YAML collections** parsed in `parser.go` with validation
- **CEL access rules** with auth.*, doc.*, request.* contexts
- **Blob field** exists but stores data directly in SQLite (not suitable for large files)
- **Schema diffing** in `differ.go` for migrations
- Key file: `internal/schema/types.go` for adding new types

### Handler Patterns (Alyx codebase)
- **Handlers struct** with dependency injection: `New(db, schema, cfg, rules)`
- **Route registration**: `r.mux.HandleFunc("METHOD /path", r.wrap(handler))`
- **Middleware**: Recovery → RequestID → Metrics → Logging → CORS
- **Access control**: `h.checkAccess(r, collection, op, doc)` with CEL
- **Response helpers**: `JSON()`, `Error()`, `ErrorWithDetails()`
- **NO multipart/file upload handling currently exists** - gap to fill
- Key files: `internal/server/handlers/handlers.go`, `internal/server/router.go`

### Appwrite Storage (librarian research)
- **Bucket config**: maximumFileSize, allowedFileExtensions, compression (none/gzip/zstd), encryption, antivirus, fileSecurity (file-level perms)
- **API structure**: `/v1/storage/buckets/{bucketId}/files/{fileId}` with separate /download, /view, /preview endpoints
- **Chunked uploads**: 5MB chunks via Content-Range header
- **File-document relationship**: Store file ID in document field, not embedded - opt-in loading to avoid payload bloat
- **Storage backends**: local, s3, dospaces, backblaze, linode, wasabi - configured via env var
- **Image transforms**: width, height, gravity, quality, borderWidth/Color/Radius, opacity, rotation, background, output format
- **Security**: Encryption at rest, antivirus scanning, validation - all skipped for files >20MB
- **Permissions**: `read("any")`, `create("any")`, etc. - both bucket-level and file-level when fileSecurity=true

## Test Strategy
- **Approach**: TDD (test-driven development)
- **Infrastructure**: Go standard testing (existing in project)
- **Coverage**: Unit tests for backends, handlers, schema parser; integration tests for full flow

## Open Questions
- (all resolved)

## Scope Boundaries

### INCLUDE (V1)
- Schema-level bucket definitions with access rules, filetype limits, filesize limits
- Multiple backends: filesystem, S3, S3-compatible (R2, minio)
- New `file` field type for single/multiple file references
- Dedicated `_alyx_files` system table for metadata
- Resumable/chunked uploads (TUS protocol)
- File type validation (MIME + extension matching)
- Compression (gzip/zstd, configurable per bucket)
- Bucket-level CEL access rules + optional file-level overrides
- URL structure: `/api/files/{bucket}/{file_id}` with /download, /view
- Signed URLs with expiry (for temporary access)
- Design for versioning (field in schema, implement later)
- Design for image transforms (API structure, implement later)
- Design for encryption/antivirus (hooks, implement later)

### EXCLUDE (V1)
- CDN integration
- Video transcoding
- Cross-bucket file moves
- Actual encryption at rest implementation (deferred)
- Actual antivirus scanning implementation (deferred)
- Actual image transformation implementation (deferred)
- Actual versioning implementation (deferred)
