# CLAUDE.md

This file provides guidance to work with code in this repository.

## Commands

```bash
# Build server (kvd) and CLI client (kvcli)
make build

# Run all tests
make test

# Run tests with race detector
make race

# Run a single test
go test -v ./internal/core/... -run TestServiceAutoSnapshotWithoutFurtherWrites

# Run tests in a specific package
go test -v ./internal/storage/...

# Build GUI (requires: go install github.com/wailsapp/wails/v2/cmd/wails@latest)
make gui

# Clean build artifacts
make clean
```

## Architecture

The project is a Redis-like key-value store with three binaries (`cmd/kvd`, `cmd/kvcli`, `cmd/kvgui`) built on shared internal packages.

### Layer Structure

**`internal/storage/`** — Storage primitives, no business logic:
- `ConcurrentMap` (`concurrent_map.go`): Sharded hash map. Key distribution uses SHA256 (first 4 bytes) masked to shard index. Each shard has its own `sync.RWMutex`. Set/Delete return `int64` memory delta for tracking.
- `AOFPersister` (`persistence.go`): Append-only file. Format is tab-separated: `SET\tkey\tb64value\texpiresAt\n` and `DEL\tkey\n`. Supports async background rewrite (triggered by file size threshold) that snapshots current state + buffers incremental writes during rewrite.
- `RDBManager` (`rdb.go`): Snapshot persistence using Go's `encoding/gob`. Saves/loads a `[]rdbEntry` slice atomically via a temp file + rename.

**`internal/core/`** — Business logic:
- `Service` (`service.go`): Central coordinator. Holds references to `ConcurrentMap`, `TTLManager`, `AOFPersister`, and `RDBManager`. Tracks memory usage via `atomic.Int64` (incremented/decremented by the delta returned from storage operations). Tracks hits/misses/requests with atomics.
- `TTLManager` (`ttl_heap.go`): Min-heap of `TTLItem` sorted by `ExpiresAt`. Background goroutine sleeps until the soonest expiry, then calls `onExpire(key)` which deletes from storage and adjusts memory counter.

**`internal/server/`** — HTTP layer:
- Uses stdlib `net/http` with Go 1.22+ method-pattern routing (`"GET /v1/key"`, `"PUT /v1/key"`, etc.).
- `Middleware` type (`func(http.Handler) http.Handler`) applied in reverse order for correct chaining.
- HTTP values are **base64-encoded** — callers must encode values before PUT and decode after GET.

**`pkg/protocol/`** — Shared protocol types:
- All responses use envelope: `{code: int, data: any, msg: string}`.
- Error codes are numeric constants (e.g., `CodeKeyNotFound = 1001`). `Service.ErrorToCode()` maps Go errors to codes.

**`pkg/client/`** — Go SDK client for programmatic access.

### Key Behaviors

**Startup data loading** (`Service.loadOnStartup`): AOF takes priority — if the AOF file exists, it replays AOF and skips RDB. If no AOF file, loads RDB snapshot.

**Auto-snapshot** (`maybeAutoSnapshot`): Called after every Set/Delete. Also runs on a 1-second ticker. Checks configured `save_rules` (seconds elapsed + minimum changes) to decide whether to trigger `Snapshot()`.

**Memory enforcement**: `Set` checks `currentMem + estimatedDelta > maxMemory` *before* writing, returning `ErrMemoryFull`. Memory delta accounting uses the value returned by `ConcurrentMap.Set/Delete` (positive for new/larger, negative for deletions).

**Persistence priority**: If both AOF and RDB are enabled, AOF is the source of truth on startup. RDB snapshots are still taken for faster cold-start fallback.

### Code Conventions

- Imports: stdlib → external → internal (three groups).
- Logging: `log/slog` with structured key-value pairs (not the `log` package).
- Errors: package-level `var Err... = errors.New(...)`, wrapped with `fmt.Errorf("%w", err)`, compared with `errors.Is()`.
- Mutexes always unlocked via `defer` immediately after locking.
- Module path: `github.com/shinerio/gopher-kv`

## Documentation Maintenance

- Before adding or modifying any feature, read @specs/design.md and @specs/spec.md to understand existing design and requirements.
- After completing new feature development or modifying existing features, update CLAUDE.md, @specs/design.md, and @specs/spec.md to reflect the changes.
