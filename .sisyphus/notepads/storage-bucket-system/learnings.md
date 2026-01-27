# Learnings - Storage Bucket System

This file tracks conventions, patterns, and best practices discovered during implementation.

---

## Bucket Schema Parsing Implementation

**Date**: 2026-01-27
**Task**: Add bucket schema parsing to Alyx's schema system

### Patterns Followed

1. **Struct Design** (from `Collection` pattern in `types.go`):
   - `Bucket` struct with `Name` field marked `yaml:"-"` (set during parsing)
   - Embedded `Rules` pointer for access control (reused from collections)
   - Configuration fields: `Backend`, `MaxFileSize`, `MaxTotalSize`, `AllowedTypes`, `Compression`

2. **Parser Pattern** (from `parseCollection()` in `parser.go`):
   - Created `rawBucket` struct for YAML unmarshaling
   - Added `parseBucket()` function following same structure as `parseCollection()`
   - Updated `rawSchema` to include `Buckets map[string]*rawBucket`
   - Updated `Parse()` to iterate over buckets and call `parseBucket()`

3. **Validation Pattern** (from `validateCollection()` in `parser.go`):
   - Created `validateBucket()` function with same signature pattern
   - Reused `identifierRegex` for bucket name validation
   - Same reserved prefix check (`_alyx`) as collections
   - Added bucket-specific validations:
     - Required `backend` field
     - Non-negative file size limits
     - MIME type format validation (must be `type/subtype`)

4. **Test-Driven Development**:
   - Wrote comprehensive tests FIRST (RED phase)
   - Implemented structs and functions (GREEN phase)
   - All tests passing with clean LSP diagnostics

### Key Decisions

1. **Backend as String Reference**: Stored backend name as string, not loading actual backend config. This keeps schema layer pure and defers backend resolution to config layer.

2. **Reused Rules Struct**: Instead of creating `BucketRules`, reused existing `Rules` struct since CRUD operations are identical for both collections and buckets.

3. **MIME Type Validation**: Simple format check (`type/subtype`) with support for wildcards (`image/*`). Does not validate against official MIME type registry.

4. **Zero Values for Unlimited**: `MaxFileSize` and `MaxTotalSize` of 0 means unlimited (no restrictions).

### Test Coverage

- ✅ Valid bucket parsing with all configuration fields
- ✅ Minimal bucket configuration (only required `backend`)
- ✅ Multiple buckets in single schema
- ✅ Invalid bucket names (uppercase, numbers, special chars, reserved prefix)
- ✅ MIME type format validation
- ✅ Missing required `backend` field
- ✅ Negative file size validation

### Files Modified

- `internal/schema/types.go`: Added `Bucket` struct, updated `Schema` struct
- `internal/schema/parser.go`: Added `rawBucket`, `parseBucket()`, `validateBucket()`
- `internal/schema/bucket_test.go`: Comprehensive test suite (new file)

### Next Steps

This foundation enables:
- Backend configuration loading (config layer)
- Storage service implementation (storage layer)
- File upload/download handlers (server layer)
- Bucket management API endpoints


## File Field Type Implementation

**Date**: 2026-01-27
**Task**: Add `file` field type to Alyx's schema system with bucket reference support

### Patterns Followed

1. **FieldType Constant Addition** (from existing field types in `types.go`):
   - Added `FieldTypeFile = "file"` constant to FieldType enum
   - Updated all type switch statements: `IsValid()`, `SQLiteType()`, `GoType()`, `TypeScriptType()`, `PythonType()`
   - Followed TEXT storage pattern (stores UUID reference to file)

2. **Config Struct Pattern** (from `SelectConfig` and `RelationConfig`):
   - Created `FileConfig` struct with public API docstring
   - Placed before `Field` struct to avoid forward reference errors
   - Fields: `Bucket string`, `MaxSize int64`, `AllowedTypes []string`, `OnDelete OnDeleteAction`
   - Added `File *FileConfig` pointer to `Field` struct

3. **Validation Pattern** (from `validateFieldSelect()` and `validateFieldRelation()`):
   - Created `validateFieldFile()` function with schema reference parameter
   - Validates file config only used with file field type
   - Validates file field type requires file config
   - Validates bucket reference exists in schema
   - Validates OnDelete action compatibility with nullable
   - Validates MaxSize is non-negative

4. **Test-Driven Development**:
   - Wrote comprehensive tests FIRST (RED phase)
   - Implemented types and validation (GREEN phase)
   - All tests passing with clean build

### Key Decisions

1. **Storage as UUID Reference**: File field stores TEXT (UUID) in SQLite, not the actual file data. The UUID references a file in the storage backend.

2. **Type Mapping**:
   - SQLite: `TEXT` (UUID reference)
   - Go: `string` (single), `*string` (nullable)
   - TypeScript: `string` (single), `string | null` (nullable)
   - Python: `str` (single), `Optional[str]` (nullable)

3. **Bucket Reference Validation**: File field must reference an existing bucket in the schema. This ensures referential integrity at schema level.

4. **OnDelete Behavior**: Supports `restrict` (prevent deletion), `cascade` (delete file), `set null` (orphan file). Follows same pattern as relation fields.

5. **Zero Values for Unlimited**: `MaxSize` of 0 means unlimited file size (no restriction).

### Test Coverage

- ✅ File field SQLiteType returns TEXT
- ✅ File field GoType returns string/nullable string
- ✅ File field TypeScriptType returns string/nullable string
- ✅ File config parsing with all fields (bucket, max_size, allowed_types, on_delete)
- ✅ File config parsing with minimal config (only bucket)
- ✅ Validation fails if file config missing
- ✅ Validation fails if bucket not specified
- ✅ Validation fails if bucket doesn't exist in schema
- ✅ OnDelete action validation (restrict, cascade, set null)

### Files Modified

- `internal/schema/types.go`: Added `FieldTypeFile` constant, `FileConfig` struct, updated all type methods
- `internal/schema/parser.go`: Added `validateFieldFile()`, updated `validateField()`, updated error message
- `internal/schema/file_test.go`: Comprehensive test suite (new file)

### Next Steps

This foundation enables:
- File upload/download API endpoints
- File metadata storage in database
- Storage backend integration (local, S3, etc.)
- File validation (size, MIME type) at runtime
- Cascade deletion of files when parent record deleted


## System Tables for File Metadata and TUS Upload State

**Date**: 2026-01-27
**Task**: Define system tables for file metadata and TUS upload state

### Patterns Followed

1. **Table Definition Pattern** (from `internal/database/migrations/sql/*.sql`):
   - Created `internal/storage/tables.go` with SQL generation functions
   - Followed existing migration SQL patterns (CREATE TABLE IF NOT EXISTS, TEXT timestamps, INTEGER sizes)
   - Used `_alyx_` prefix for system tables (reserved namespace)
   - Separated table creation from index creation (different functions)

2. **Function Organization** (from existing store patterns):
   - Individual functions for each table: `FilesTableSQL()`, `UploadsTableSQL()`
   - Individual functions for each index set: `FilesTableIndexes()`, `UploadsTableIndexes()`
   - Aggregate functions: `AllStorageTables()`, `AllStorageIndexes()`
   - All functions return strings or string slices (ready for migration execution)

3. **Test-Driven Development**:
   - Wrote comprehensive tests FIRST (RED phase)
   - Implemented table SQL generation (GREEN phase)
   - All tests passing with clean LSP diagnostics

### Key Decisions

1. **Table Structure**:
   - `_alyx_files`: Stores permanent file metadata (id, bucket, name, path, mime_type, size, checksum, compression info, metadata JSON, version, timestamps)
   - `_alyx_uploads`: Stores temporary TUS upload state (id, bucket, filename, size, offset, metadata, expires_at, created_at)

2. **Constraints and Indexes**:
   - UNIQUE constraint on `(bucket, path)` in `_alyx_files` prevents duplicate files in same location
   - Index on `bucket` in `_alyx_files` for efficient bucket-scoped queries
   - Index on `expires_at` in `_alyx_uploads` for efficient cleanup queries (delete expired uploads)

3. **Data Types**:
   - TEXT for all string fields (id, bucket, name, path, mime_type, checksum, compression_type, metadata, timestamps)
   - INTEGER for numeric fields (size, offset, original_size, version)
   - BOOLEAN for flags (compressed) - stored as INTEGER in SQLite (0/1)
   - TEXT for timestamps (ISO 8601 format via time.RFC3339)
   - TEXT for metadata (JSON serialized)

4. **Nullable Fields**:
   - Required: id, bucket, name, path, mime_type, size (files); id, bucket, size, offset (uploads)
   - Optional: checksum, compression_type, original_size, metadata, filename, expires_at
   - Defaults: compressed=FALSE, offset=0, version=1

### Test Coverage

- ✅ FilesTableSQL generates correct table structure
- ✅ FilesTableSQL includes all required fields with correct types
- ✅ FilesTableSQL includes UNIQUE constraint on (bucket, path)
- ✅ FilesTableIndexes includes bucket index
- ✅ UploadsTableSQL generates correct table structure
- ✅ UploadsTableSQL includes all required fields with correct types
- ✅ UploadsTableIndexes includes expires_at index
- ✅ AllStorageTables returns both tables
- ✅ AllStorageIndexes returns all indexes

### Files Created

- `internal/storage/tables.go`: Table SQL generation functions
- `internal/storage/tables_test.go`: Comprehensive test suite

### Next Steps

This foundation enables:
- Migration creation (add SQL to `internal/database/migrations/sql/`)
- Store implementation (CRUD operations for files and uploads)
- TUS protocol implementation (resumable uploads)
- File cleanup service (delete expired uploads)
- Storage backend integration (local, S3, etc.)

### Notes

- Tables are NOT created yet (that's a separate migration task)
- No CRUD operations implemented (that's a separate store task)
- Follows exact specification from plan (all fields, constraints, indexes)
- Ready for integration into migration system
