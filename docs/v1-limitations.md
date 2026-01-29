# Alyx V1 Known Limitations

This document lists known limitations in Alyx V1 that are being actively addressed before the V1 release.

**Context:** Alyx is inspired by PocketBase's simplicity while adding enterprise features (webhooks, scheduling, polyglot functions). Most limitations below represent features that don't exist in PocketBase at all - they're V1 implementations that need hardening before production release.

**Status:** All limitations marked below are targeted for resolution before V1 ships.

## Transaction Support

**Status:** 🚧 Not Implemented  
**PocketBase:** Also lacks transaction API  
**Impact:** Cannot execute atomic multi-record operations via API

Database transactions via the internal API are not implemented. The endpoint returns HTTP 501 (Not Implemented).

**Planned Fix:** Implement session-based transaction tracking with BEGIN/COMMIT/ROLLBACK endpoints.

## Webhook Delivery

**Status:** 🚧 Fire-and-forget (no retry)  
**PocketBase:** No outbound webhooks at all  
**Impact:** Lost webhook events on endpoint failure or network issues

Webhook calls are fire-and-forget with no retry mechanism on failure. Failed webhook invocations are logged but not reattempted.

**Planned Fix:** Add retry queue with exponential backoff, configurable max attempts, and dead letter queue for failed deliveries.

## Scheduler Persistence

**Status:** 🚧 In-memory only  
**PocketBase:** No scheduler feature  
**Impact:** Scheduled tasks and execution state lost on server restart

Scheduled tasks use in-memory tracking only. All pending and running scheduled tasks are lost when the server restarts.

**Planned Fix:** Persist scheduler state to SQLite with automatic recovery and catch-up for missed executions.

## Single-Instance Deployment

**Status:** ✅ By Design (matches PocketBase)  
**PocketBase:** Also single-instance (SQLite-based)  
**Impact:** Cannot scale horizontally, no clustering support

Alyx V1 is designed for single-instance deployment only. Rate limiting, brute force protection, and real-time subscriptions are maintained per-instance with no distributed state sharing.

**Note:** This is an architectural choice for V1 simplicity. Multi-instance support with distributed state (Redis, etc.) is planned for V2.

## Hook Registry API

**Status:** 🚧 Manifest-only  
**PocketBase:** Hooks are compiled Go code only  
**Impact:** Cannot dynamically enable/disable hooks without redeploying

Runtime hook management via API is not available. Hooks must be defined in function manifest files. There is no endpoint to register, update, or remove hooks at runtime.

**Planned Fix:** Add API endpoints for runtime hook registration, updates, and removal with hot-reload support.
