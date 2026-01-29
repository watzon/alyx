# Alyx V1 Readiness Report
**Generated**: 2026-01-29
**Commit**: 0ad7fbe
**Branch**: main

## Executive Summary

**Overall Status**: 🟡 **NOT READY FOR V1** - Critical gaps in testing, stability, and security

**Completion Estimate**: ~60% ready for production V1 release

### Critical Blockers (Must Fix)
1. **Test Suite Broken** - 9/22 packages fail to compile due to config API changes
2. **Test Coverage** - Only 33.2% overall, 7 packages have ZERO tests
3. **Security Gaps** - SQL injection risks, missing rate limiting, no input validation
4. **Stability Issues** - 5 panic/log.Fatal calls, 21 potential goroutine leaks
5. **Missing Features** - Function runtime execution incomplete (TODO in code)

---

## 1. Test Suite Status 🔴 CRITICAL

### Broken Tests (9 packages)
All test failures are due to **config API changes** - tests use struct fields that are now methods:

**Affected packages**:
- `internal/database` - 6 field errors
- `internal/events` - 7 field errors  
- `internal/executions` - 7+ field errors
- `internal/hooks` - 7+ field errors
- `internal/integration` - 7 field errors
- `internal/realtime` - 5 field errors
- `internal/scheduler` - 7 field errors
- `internal/server/handlers` - 6+ field errors
- `internal/webhooks` - 10+ field errors

**Root cause**: `DatabaseConfig` changed from struct fields to methods:
```go
// OLD (tests still use this)
cfg := config.DatabaseConfig{
    WALMode: true,
    ForeignKeys: true,
    CacheSize: -64000,
}

// NEW (actual API)
cfg := config.DatabaseConfig{Path: "./test.db"}
// Access via methods: cfg.WALMode(), cfg.ForeignKeys(), etc.
```

**Fix**: Update all test files to use method calls instead of struct initialization.

### Test Coverage by Package

| Package | Coverage | Status | Notes |
|---------|----------|--------|-------|
| `auth` | 51.7% | 🟡 Medium | Missing edge cases |
| `codegen` | 27.0% | 🟠 Low | Only TypeScript tested |
| `config` | 59.7% | 🟡 Medium | |
| `database` | BROKEN | 🔴 Critical | Test compilation fails |
| `database/migrations` | 80.0% | 🟢 Good | |
| `functions` | 54.5% | 🟡 Medium | Runtime execution untested |
| `openapi` | 93.0% | 🟢 Excellent | |
| `rules` | 79.2% | 🟢 Good | CEL evaluation covered |
| `schema` | 36.8% | 🟠 Low | Missing migration edge cases |
| `server/requestlog` | 60.2% | 🟡 Medium | |
| `storage` | 67.9% | 🟢 Good | Best coverage in project |
| **ZERO TESTS** | | | |
| `cli` | 0% | 🔴 Critical | 10 commands untested |
| `deploy` | 0% | 🔴 Critical | Rollback untested |
| `metrics` | 0% | 🟠 Low | Prometheus metrics |
| `requestctx` | 0% | 🟠 Low | Context helpers |
| `adminui` | 0% | 🟠 Low | UI handler |
| `sdk` | 0% | 🟠 Low | SDK generation |
| `server` | 0% | 🔴 Critical | Server orchestration |

**Overall**: 33.2% coverage (49 test files, 120 source files)

---

## 2. Security Vulnerabilities 🔴 CRITICAL

### SQL Injection Risks (8 instances)

**HIGH RISK** - String concatenation in SQL queries:

1. **`internal/cli/db.go:233`**
   ```go
   rows, queryErr := db.Query(fmt.Sprintf("SELECT * FROM %s", collectionName))
   ```
   **Risk**: User-controlled collection name → arbitrary table access

2. **`internal/schema/migrator.go:600,639,642,737`**
   - Multiple `fmt.Sprintf` with table/column names
   - Used during migrations (admin-only, but still risky)

3. **`internal/auth/service.go:703`**
   ```go
   query := fmt.Sprintf("UPDATE _alyx_users SET %s WHERE id = ?", strings.Join(updates, ", "))
   ```
   **Risk**: Dynamic column updates without validation

**Mitigation**: Use allowlists for table/column names, validate against schema.

### Authentication & Authorization

**MISSING**:
- ❌ Rate limiting on auth endpoints (login, register, password reset)
- ❌ Token revocation/blacklist (logout doesn't invalidate tokens)
- ❌ Brute force protection
- ❌ Account lockout after failed attempts
- ❌ CSRF protection for state-changing operations

**PRESENT**:
- ✅ JWT token generation and validation
- ✅ Password hashing (bcrypt)
- ✅ OAuth 2.0 with PKCE
- ✅ CEL-based access control rules

### Input Validation

**MISSING**:
- ❌ WebSocket message size limits (found: `maxMessageSize = 512KB` but not enforced everywhere)
- ❌ Request body size validation (server has `MaxBodySize` but not validated in handlers)
- ❌ File upload validation (MIME type, magic bytes)
- ❌ Collection/field name validation (allows SQL keywords)

### CORS Configuration

**Location**: `internal/server/middleware.go`

**Status**: ✅ Implemented but needs review
- Allows configurable origins
- Supports credentials
- Hardcoded allowed methods/headers

**Risk**: Misconfiguration could allow unauthorized cross-origin requests.

### Secrets Management

**FOUND**: No hardcoded secrets in codebase ✅

**CONCERN**: JWT secret in config file (should use env var or secrets manager)

### Path Traversal

**Storage layer** (`internal/storage/filesystem.go`):
- ✅ Uses `filepath.Clean()` and validates paths
- ✅ Checks for `..` in paths
- ✅ Restricts to bucket directories

**Risk**: LOW - properly mitigated

---

## 3. Stability Issues 🟠 HIGH

### Panic Calls (1 instance)

**`internal/database/database.go:135`**
```go
defer func() {
    if p := recover(); p != nil {
        _ = tx.Rollback()
        panic(p)  // ⚠️ Re-panics after recovery
    }
}()
```
**Impact**: Can crash the entire server on transaction errors.
**Fix**: Log error and return instead of re-panicking.

### Log.Fatal / os.Exit (4 instances)

**These will abruptly terminate the server without cleanup**:

1. `cmd/alyx/main.go:11` - `os.Exit(1)` in main
2. `internal/cli/dev.go:69` - No schema file found
3. `internal/cli/dev.go:79` - Failed to parse schema
4. `internal/cli/dev.go:88` - Failed to open database

**Impact**: No graceful shutdown, connections/resources not cleaned up.
**Fix**: Return errors instead of calling `log.Fatal()`.

### Goroutine Leaks (21 goroutines)

**HIGH RISK** - Goroutines without proper cleanup tracking:

**Realtime system** (5 goroutines):
- `internal/realtime/broker.go:66-67` - Detector + processChanges
- `internal/realtime/client.go:54-55` - writePump + pingPump per client
- `internal/realtime/detector.go:38` - Polling loop

**Event bus** (2 goroutines):
- `internal/events/bus.go:79-80` - processLoop + cleanupLoop

**Background workers** (10 goroutines):
- Scheduler, executions logger, storage cleanup, function watcher, token cleanup, OAuth state cleanup, CLI watcher (2 loops), database hooks

**Risk**: Memory leaks, goroutines continue running after context cancellation.
**Fix**: Add `sync.WaitGroup` tracking and proper shutdown in `Stop()` methods.

### Resource Leaks (4 instances)

**Missing `defer` on Close()**:

1. `internal/server/handlers/functions.go:81` - `file.Close()` without defer
2. `internal/schema/migrator.go:487,495,544,551` - `indexRows.Close()` without defer
3. `internal/cli/db.go:240` - `rows.Close()` without defer
4. `internal/storage/compression.go:69` - `rc.Close()` without defer

**Risk**: File descriptors/connections leak on error paths.

### Ignored Errors (13 instances)

**Database operations**:
- `internal/database/database.go:114` - WAL checkpoint error ignored
- `internal/schema/migrator.go:333` - PRAGMA foreign_keys error ignored in defer

**Write operations**:
- `internal/server/middleware.go:33` - HTTP write error ignored
- `internal/server/handlers/docs.go:51,72` - Response write errors ignored

**Risk**: Silent failures, especially in database operations.

---

## 4. Missing Features & Technical Debt

### Incomplete Features (from TODO comments)

1. **`internal/hooks/database.go:157`**
   ```go
   // TODO: Execute function via function runtime
   ```
   **Impact**: Database hooks don't actually invoke functions yet!

2. **`internal/server/router.go:162`**
   ```go
   // TODO: Event system routes will be enabled when components are initialized in server
   ```
   **Impact**: Event API endpoints not exposed.

3. **`internal/server/handlers/internal.go:264`**
   ```go
   // TODO: Implement transaction support with session-based TX tracking
   ```
   **Impact**: No multi-operation transactions.

### Packages Without Tests (7 packages)

**CRITICAL** (core functionality):
1. **`internal/cli/`** - All 10 commands untested (dev, migrate, init, deploy, etc.)
2. **`internal/deploy/`** - Bundle creation, rollback, versioning untested
3. **`internal/server/`** - Server lifecycle, middleware chain, routing untested

**MEDIUM** (supporting features):
4. **`internal/metrics/`** - Prometheus metrics
5. **`internal/requestctx/`** - Request context helpers
6. **`internal/adminui/`** - Admin UI handler
7. **`internal/sdk/`** - SDK generation

### Edge Cases Not Tested

**Boundary conditions**:
- Empty inputs (collection names, zero-length files)
- Nil pointer checks
- Max limits (file size, connections, body size)
- Unicode/special characters in names

**Concurrent operations**:
- Only 1 test for concurrent file access
- No tests for concurrent schema updates
- No tests for concurrent function invocations
- No tests for concurrent realtime subscriptions

**Error conditions**:
- Network failures (S3 errors)
- Disk full scenarios
- Database locks (SQLite busy/locked)
- Subprocess crashes (function runtime)

**Failure modes**:
- Partial failures (half-written files, interrupted migrations)
- Rollback failures
- Cleanup failures (orphaned resources)

---

## 5. Security Best Practices (from Research)

### Recommendations from BaaS Research

**CRITICAL (Must Implement)**:
1. ✅ SQLite foreign keys enabled (via method)
2. ✅ Prepared statements (mostly used, but see SQL injection risks above)
3. ⚠️ JWT short-lived tokens (15 min) - **VERIFY CURRENT TTL**
4. ❌ Token refresh rotation - **NOT IMPLEMENTED**
5. ❌ Rate limiting - **NOT IMPLEMENTED**
6. ✅ WAL mode enabled (via method)
7. ❌ Token revocation/blacklist - **NOT IMPLEMENTED**

**HIGH PRIORITY**:
1. ❌ WebSocket message validation - **PARTIAL** (size limit exists but not enforced)
2. ❌ API rate limiting per-IP and per-user - **NOT IMPLEMENTED**
3. ❌ Input validation framework - **MISSING**
4. ✅ Graceful shutdown - **NEEDS TESTING**
5. ❌ Security headers middleware - **NOT FOUND**

**MEDIUM PRIORITY**:
1. ❌ Fuzzing tests - **NOT IMPLEMENTED**
2. ❌ Property-based testing - **NOT IMPLEMENTED**
3. ❌ API key management - **NOT IMPLEMENTED**
4. ❌ Audit logging - **PARTIAL** (execution logs exist)
5. ❌ Secure delete pragma - **NOT ENABLED**

### Function Runtime Security (from Research)

**Subprocess isolation** (for polyglot functions):
1. ❌ Linux namespaces - **NOT IMPLEMENTED**
2. ❌ cgroups v2 resource limits - **NOT IMPLEMENTED**
3. ❌ Network isolation (SSRF protection) - **NOT IMPLEMENTED**
4. ❌ Filesystem restrictions - **NOT IMPLEMENTED**
5. ❌ Environment variable validation - **NOT IMPLEMENTED**
6. ❌ Dependency scanning - **NOT IMPLEMENTED**

**Current status**: Functions run as subprocesses with NO isolation.

**Risk**: HIGH - Malicious functions can:
- Access host filesystem
- Make arbitrary network requests (SSRF)
- Consume unlimited CPU/memory
- Read environment variables (secrets)

---

## 6. Recommendations by Priority

### P0: BLOCKERS (Must fix before V1)

1. **Fix broken tests** (1-2 days)
   - Update all test files to use `DatabaseConfig` methods
   - Verify all tests pass: `go test ./...`

2. **Implement rate limiting** (2-3 days)
   - Add token bucket rate limiter
   - Apply to auth endpoints (login, register, password reset)
   - Apply to API endpoints (per-IP and per-user)

3. **Fix SQL injection risks** (1 day)
   - Add table/column name validation against schema
   - Use allowlists for dynamic identifiers

4. **Complete function runtime** (3-5 days)
   - Implement `TODO` in `hooks/database.go:157`
   - Add basic subprocess isolation (at minimum: timeouts, resource limits)
   - Test end-to-end function execution

5. **Fix stability issues** (1-2 days)
   - Replace `panic()` with error returns
   - Replace `log.Fatal()` with error returns
   - Add `defer` to all `Close()` calls

### P1: CRITICAL (Should fix before V1)

6. **Add server tests** (3-4 days)
   - Server lifecycle (startup, shutdown)
   - Middleware chain execution
   - Router registration and path matching

7. **Add CLI tests** (2-3 days)
   - Test all 10 commands
   - Test file watching and hot-reload
   - Test migration safety

8. **Implement token revocation** (1-2 days)
   - In-memory blacklist with TTL
   - Logout endpoint
   - Revoke on password change

9. **Add input validation** (2-3 days)
   - Request body size enforcement
   - File upload validation (MIME, magic bytes)
   - Collection/field name validation

10. **Fix goroutine leaks** (2-3 days)
    - Add `sync.WaitGroup` to all background workers
    - Test graceful shutdown

### P2: HIGH (Nice to have for V1)

11. **Add security headers middleware** (1 day)
    - X-Frame-Options, X-Content-Type-Options, CSP, HSTS

12. **Improve test coverage to 70%** (5-7 days)
    - Focus on critical paths: auth, database, schema, functions
    - Add edge case tests (nil, empty, max limits)
    - Add concurrent operation tests

13. **Add deployment tests** (2-3 days)
    - Bundle creation and diffing
    - Rollback functionality
    - Migration versioning

14. **Add function runtime isolation** (5-7 days)
    - Linux namespaces (user, PID, network, mount)
    - cgroups v2 (CPU, memory limits)
    - Network restrictions (block private IPs)

15. **Add audit logging** (2-3 days)
    - Log all auth events (login, logout, failed attempts)
    - Log all admin operations (schema changes, user management)

### P3: MEDIUM (Post-V1)

16. **Add fuzzing tests** (3-5 days)
    - Schema parser
    - SQL query builder
    - Function input validation

17. **Add benchmarks** (2-3 days)
    - Query builder performance
    - Realtime message throughput
    - Function invocation latency

18. **Implement API key management** (3-4 days)
    - Generate, hash, store API keys
    - Scope-based permissions
    - Expiration support

19. **Add property-based testing** (3-5 days)
    - Token generation properties
    - Query builder properties
    - Schema migration properties

20. **Improve function runtime security** (7-10 days)
    - Seccomp profiles
    - Dependency scanning
    - SBOM generation

---

## 7. Estimated Timeline to V1

**Current completion**: ~60%

**Remaining work**:
- P0 (Blockers): 8-13 days
- P1 (Critical): 10-14 days
- P2 (High): 21-30 days

**Total**: 39-57 days (8-11 weeks) with 1 developer

**Recommended approach**:
1. **Sprint 1 (2 weeks)**: Fix P0 blockers
2. **Sprint 2 (2 weeks)**: Complete P1 critical items
3. **Sprint 3 (2 weeks)**: High-priority P2 items (security, testing)
4. **Sprint 4 (1 week)**: Final testing, documentation, polish

**Realistic V1 target**: 7-8 weeks from now

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| SQL injection in production | Medium | Critical | P0: Add validation |
| Server crash from panic | Medium | Critical | P0: Replace panics |
| Goroutine memory leak | High | High | P1: Add WaitGroup |
| Function runtime exploit | High | Critical | P1: Add isolation |
| Broken tests block development | High | Medium | P0: Fix config API |
| Missing rate limiting → DoS | High | High | P0: Implement |
| Token theft (no revocation) | Medium | High | P1: Add blacklist |
| Incomplete features shipped | Medium | Medium | P0: Complete TODOs |

---

## 9. Next Steps

### Immediate Actions (This Week)

1. **Fix broken tests** - Update `DatabaseConfig` usage in all test files
2. **Run full test suite** - Verify 100% of tests pass
3. **Fix SQL injection** - Add table/column validation
4. **Replace panic/log.Fatal** - Return errors instead
5. **Add missing defer** - Fix resource leaks

### Short-term (Next 2 Weeks)

6. **Implement rate limiting** - Token bucket for auth + API
7. **Complete function runtime** - Finish TODO in database hooks
8. **Add server tests** - Lifecycle, middleware, routing
9. **Fix goroutine leaks** - WaitGroup + proper shutdown
10. **Add input validation** - Body size, file uploads, names

### Medium-term (Next 4-6 Weeks)

11. **Improve test coverage to 70%** - Focus on critical paths
12. **Add security headers** - Middleware for all responses
13. **Implement token revocation** - Blacklist + logout
14. **Add CLI tests** - All commands + hot-reload
15. **Add function isolation** - Namespaces + cgroups

---

## 10. Conclusion

**Alyx is NOT ready for V1 production release** due to:
- Broken test suite (9 packages)
- Low test coverage (33.2%)
- Critical security gaps (SQL injection, no rate limiting)
- Stability issues (panics, goroutine leaks)
- Incomplete features (function runtime)

**However**, the foundation is solid:
- ✅ Schema system works well
- ✅ Database layer is functional
- ✅ Storage layer has excellent coverage
- ✅ OAuth implementation is complete
- ✅ Realtime subscriptions work

**Recommendation**: Focus on P0 and P1 items over the next 4-6 weeks to reach a production-ready V1.

**Confidence level**: With focused effort on the priorities above, Alyx can be V1-ready in **7-8 weeks**.
