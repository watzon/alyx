# Alyx Development Roadmap

This roadmap tracks the implementation progress of Alyx from foundation to production-ready MVP.

**Current Phase**: Phase 1 - Foundation  
**Last Updated**: January 2026

---

## Legend

- ✅ **Complete**: Implemented and tested
- 🚧 **In Progress**: Currently being worked on
- ⏳ **Planned**: Not yet started
- 🔜 **Next Up**: High priority for next sprint

---

## Phase 1: Foundation

**Goal**: Basic server that can CRUD data via HTTP API

### 1.1 Project Bootstrap ✅

- [x] Initialize Go module with dependencies
- [x] Set up project structure
- [x] Configure linting (golangci-lint)
- [x] Set up basic CI (GitHub Actions)
- [x] Create Makefile with common targets

### 1.2 Configuration System ✅

- [x] Define config structs
- [x] YAML config loading (alyx.yaml)
- [x] Environment variable overrides
- [x] Config validation

### 1.3 Schema System ✅

- [x] Define schema types (Collection, Field, Index, Rule)
- [x] YAML parser with validation
- [x] Schema → SQL DDL generator
- [x] Schema differ (compare two schemas)
- [x] Migration SQL generator
- [x] Migration runner with history tracking

### 1.4 Database Layer ✅

- [x] SQLite connection management
- [x] Query builder (parameterized queries)
- [x] CRUD operations
- [x] Transaction support
- [x] Change detection triggers

### 1.5 HTTP API ✅

- [x] Server setup with middleware
- [x] Collection CRUD endpoints
- [x] Query parameter parsing (filter, sort, limit)
- [x] Error handling & responses
- [x] Request logging

**Deliverable**: ✅ Server that loads schema.yaml, creates tables, exposes REST API

---

## Phase 2: Realtime Engine 🔜

**Goal**: WebSocket subscriptions with live updates

### 2.1 WebSocket Infrastructure ⏳

- [ ] WebSocket upgrade handler
- [ ] Client connection management
- [ ] Ping/pong keepalive
- [ ] Clean disconnection handling

### 2.2 Subscription System ⏳

- [ ] Subscription protocol implementation
- [ ] Filter parsing and validation
- [ ] Initial snapshot sending
- [ ] Subscription indexing for efficient matching

### 2.3 Change Broadcasting ⏳

- [ ] Change detector (poll _alyx_changes)
- [ ] Change → subscription matching
- [ ] Delta calculation
- [ ] Fan-out to subscribers

**Deliverable**: Clients can subscribe to queries and receive live updates

---

## Phase 3: Authentication ⏳

**Goal**: Complete auth system with CEL rules

### 3.1 Auth Infrastructure ⏳

- [ ] User table and session management
- [ ] Password hashing (bcrypt)
- [ ] JWT generation and validation
- [ ] Refresh token rotation

### 3.2 Auth Endpoints ⏳

- [ ] Register endpoint
- [ ] Login endpoint
- [ ] Refresh endpoint
- [ ] Logout endpoint
- [ ] Auth middleware

### 3.3 CEL Rules Engine ⏳

- [ ] CEL environment setup
- [ ] Rule compilation and caching
- [ ] Rule evaluation on CRUD operations
- [ ] Integration with realtime (filter by permission)

**Deliverable**: Protected endpoints with CEL-based access control

---

## Phase 4: Function Runtime ⏳

**Goal**: Container-based serverless functions

### 4.1 Container Management ⏳

- [ ] Docker/Podman client integration
- [ ] Container pool manager
- [ ] Container lifecycle (create, start, stop, remove)
- [ ] Health checking

### 4.2 Runtime Images ⏳

- [ ] Node.js runtime image + executor
- [ ] Python runtime image + executor
- [ ] Function SDK for each language
- [ ] Build and publish images

### 4.3 Function Execution ⏳

- [ ] Function discovery (scan functions/)
- [ ] Routing to appropriate runtime
- [ ] Request/response protocol
- [ ] Internal callback API for DB access
- [ ] Timeout and error handling

### 4.4 Function SDK Polish ⏳

- [ ] Transaction support
- [ ] Logging integration
- [ ] Environment variables
- [ ] TypeScript types for SDK

**Deliverable**: Invoke functions in Node.js/Python via HTTP

---

## Phase 5: CLI & Developer Experience ⏳

**Goal**: Complete development workflow

### 5.1 CLI Commands ⏳

- [ ] `alyx init` with templates
- [ ] `alyx migrate` commands
- [ ] `alyx db` utilities

### 5.2 Dev Mode ⏳

- [ ] File watcher implementation
- [ ] Schema change detection + auto-migrate
- [ ] Function change detection + hot-reload
- [ ] Dev server with all features

### 5.3 Code Generation ⏳

- [ ] TypeScript client generator
- [ ] Go client generator
- [ ] Python client generator
- [ ] Integration with dev mode (auto-regenerate)

### 5.4 Deploy Command ⏳

- [ ] Bundle preparation
- [ ] Remote diff checking
- [ ] Deployment execution
- [ ] Rollback support

**Deliverable**: Complete `alyx dev` workflow with codegen

---

## Phase 6: Polish & Documentation ⏳

**Goal**: Production-ready MVP

### 6.1 Admin UI ⏳

- [ ] Basic Svelte app structure
- [ ] Schema viewer
- [ ] Collection browser (CRUD)
- [ ] Function list and logs
- [ ] Embed in binary

### 6.2 Error Handling & Observability ⏳

- [ ] Structured error responses
- [ ] Request ID tracing
- [ ] Metrics endpoint
- [ ] Health check endpoint

### 6.3 Documentation ⏳

- [ ] Getting started guide
- [ ] Schema reference
- [ ] Functions guide
- [ ] Client SDK docs
- [ ] Deployment guide

### 6.4 Examples ⏳

- [ ] Blog example
- [ ] Todo app example
- [ ] README polish

**Deliverable**: MVP ready for public use

---

## Progress Summary

### Overall Completion: ~19% (Phase 1 Complete)

| Phase | Status | Completion |
|-------|--------|------------|
| **Phase 1: Foundation** | ✅ Complete | 100% |
| **Phase 2: Realtime** | 🔜 Next | 0% |
| **Phase 3: Authentication** | ⏳ Planned | 0% |
| **Phase 4: Functions** | ⏳ Planned | 0% |
| **Phase 5: CLI & DX** | ⏳ Planned | 0% |
| **Phase 6: Polish** | ⏳ Planned | 0% |

---

## Post-MVP Improvements

These features are planned for future releases after the initial MVP is complete.

### Near-term (v1.1 - v1.3)

- **Multi-tenancy**: Database-per-tenant isolation with resource limits
- **File Storage**: Local filesystem and S3-compatible backends
- **Scheduled Functions**: Cron-based function execution
- **Background Jobs**: Queue system for async processing
- **OAuth Expansion**: Apple, Microsoft, Discord, custom OIDC

### Medium-term (v1.4 - v2.0)

- **GraphQL API**: Auto-generated from schema with subscriptions
- **Edge Deployment**: Turso + Fly.io integration for global distribution
- **Webhooks**: Event-based HTTP callbacks
- **Plugins**: Custom auth, field types, lifecycle hooks

### Long-term (v2.0+)

- **Clustering**: Horizontal scaling with shared state
- **Version Control Integration**: Git-based schema, branch deployments
- **Visual Schema Editor**: Drag-and-drop interface
- **AI Features**: Natural language to schema, query optimization

---

## Risk Mitigation

### High-Priority Concerns

1. **Container Cold Start Performance**
   - Mitigation: Pre-warm pools, snapshot optimization
   - Monitor: Target < 500ms cold start

2. **Schema Migration Safety**
   - Mitigation: Conservative auto-migrations, manual for destructive changes
   - Testing: Comprehensive edge case coverage

3. **CEL Rule Complexity**
   - Mitigation: Limit rule complexity, extensive test suite
   - Documentation: Security best practices guide

---

## How to Contribute

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

**Current Focus**: Phase 2 (Realtime Engine) - WebSocket infrastructure and subscription system

**High-Impact Areas**:
- Real-time subscription indexing optimization
- Container pool management strategies
- Client SDK generator templates

---

## References

- [PLANNING.md](PLANNING.md) - Comprehensive technical planning document
- [AGENTS.md](AGENTS.md) - Development guidelines and code style
- [README.md](README.md) - Project overview and quick start

---

_Last Updated: January 23, 2026_  
_Document Version: 1.0_
