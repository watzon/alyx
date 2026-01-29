# Alyx V1 Known Limitations

This document lists known limitations in Alyx V1. These are architectural constraints, not bugs.

## Transaction Support

Database transactions via the internal API are not implemented. The endpoint returns HTTP 501 (Not Implemented).

## Webhook Delivery

Webhook calls are fire-and-forget with no retry mechanism on failure. Failed webhook invocations are logged but not reattempted.

## Scheduler Persistence

Scheduled tasks use in-memory tracking only. All pending and running scheduled tasks are lost when the server restarts.

## Single-Instance Deployment

Alyx V1 is designed for single-instance deployment only. Rate limiting, brute force protection, and real-time subscriptions are maintained per-instance with no distributed state sharing.

## Hook Registry API

Runtime hook management via API is not available. Hooks must be defined in function manifest files. There is no endpoint to register, update, or remove hooks at runtime.
