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

