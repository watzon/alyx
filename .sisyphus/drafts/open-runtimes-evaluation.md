# Draft: Open-Runtimes Evaluation for Alyx

## Original Request
User wants to evaluate whether adopting open-runtimes as a base for Alyx's serverless function execution would make sense, similar to how Appwrite uses it.

## Research Findings

### Current Alyx Implementation
- **Custom runtimes**: Node.js and Python with custom executors
- **Architecture**: Docker containers with simple HTTP servers
- **SDK**: Custom `@alyx/functions` SDK with database client, logging, validation
- **Container management**: Custom Go-based pool manager with container lifecycle management
- **Protocol**: JSON over HTTP with custom request/response format

### Open-Runtimes Architecture

**Components:**
1. **Executor** (PHP/Swoole) - Manages container lifecycle, handles builds, routes requests
2. **Runtimes** - Pre-built Docker images for each language (node, python, php, go, dart, deno, bun, ruby, swift, .NET, java, kotlin, C++)
3. **Proxy** (optional) - Load balancing and scaling across multiple executors

**Supported Runtimes (18+):**
| Language | Versions |
|----------|----------|
| Node.js | Multiple versions |
| Python | Multiple + ML variants |
| PHP | Multiple versions |
| Go | Multiple versions |
| Dart | Multiple versions |
| Deno | Multiple versions |
| Bun | Multiple versions |
| Ruby | Multiple versions |
| Swift | Multiple versions |
| .NET | Multiple versions |
| Java | Multiple versions |
| Kotlin | Multiple versions |
| C++ | Multiple versions |

**Key Features:**
- Cold starts < 100ms
- Execution time < 1ms
- Standardized function interface across all languages
- Build system for packaging functions with dependencies
- S3-compatible storage support for function artifacts

### How Appwrite Uses Open-Runtimes
- `openruntimes-executor` container handles all function execution
- Workers queue builds and executions via Redis
- Functions are stored in S3-compatible storage (or filesystem)
- Build pipeline compiles/packages functions before execution
- Executor manages container lifecycle (create, execute, cleanup)

## Open Questions

### Technical Fit
1. **Protocol compatibility**: Open-runtimes has its own request/response format. Would need adapter or migration.
2. **SDK integration**: Alyx's custom SDK features (db client, auth context) would need to be built for each open-runtime
3. **PHP dependency**: Executor is written in PHP/Swoole - adds operational complexity vs pure Go

### Trade-offs Identified

**Pros of Adopting Open-Runtimes:**
1. **Instant language support**: 13 languages, 18+ runtimes immediately available
2. **Battle-tested**: Used in production by Appwrite (large user base)
3. **Maintained by dedicated team**: Less burden on Alyx to maintain runtime images
4. **Build system included**: Handles dependency installation, compilation
5. **Production-ready**: Cold start optimization, container recycling, health checks
6. **Active development**: Regular updates, new runtime versions

**Cons of Adopting Open-Runtimes:**
1. **Loss of control**: Tied to open-runtimes release cycle and decisions
2. **Protocol translation**: Alyx's custom format differs from open-runtimes format
3. **SDK maintenance burden**: Would need to maintain SDK for EACH language (vs just 2 now)
4. **PHP executor**: Adds PHP/Swoole to stack (vs Go-only)
5. **Custom features harder**: Alyx-specific features (db client, auth context) need per-language implementation
6. **Debugging complexity**: More layers between Alyx and function execution

**Hybrid Option:**
- Keep Alyx's executor model but adopt open-runtimes Docker images
- Get language support without adopting their executor
- Maintain protocol control and SDK design
- May not work cleanly - open-runtimes images expect specific entrypoints

## Decision Points Needed

1. **Priority**: More languages vs simpler architecture?
2. **SDK strategy**: Thin SDK (just function interface) vs rich SDK (db, auth, etc.)?
3. **Build system**: Adopt open-runtimes builds or keep separate?
4. **Executor**: Use their PHP executor or maintain Go executor?
