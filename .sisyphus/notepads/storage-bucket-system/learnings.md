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

