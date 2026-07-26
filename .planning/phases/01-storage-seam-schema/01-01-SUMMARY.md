---
phase: 01-storage-seam-schema
plan: 01
subsystem: store
tags: [storage, sqlite, seam, schema, go-module]
requires: []
provides:
  - "MemoryStore + BlobStore interface contracts (the storage seam)"
  - "Agent/Memory model types, scope/state consts, ErrNotImplemented/ErrNotFound sentinels"
  - "store.Open() confined SQLite adapter with DSN pragmas + withTx helper"
  - "idempotent two-table schema (agents, memories) + idx_memories_aging"
affects:
  - "Phase 1 Plan 02 (implements MemoryStore method bodies against this contract)"
  - "Phase 1 Plan 03 (localBlobStore implements BlobStore)"
  - "Phases 2-4 (call MemoryStore only; never import adapters)"
tech-stack:
  added:
    - "modernc.org/sqlite v1.54.0 (pure-Go SQLite driver, CGO_ENABLED=0)"
    - "github.com/google/uuid v1.6.0 (agent UUIDv4 id; wired in Plan 02)"
  patterns:
    - "Ports-and-adapters seam: driver blank-import confined to store/sqlite.go (D-01)"
    - "DSN _pragma= params (not per-Exec) + SetMaxOpenConns(1) for pool-wide correctness"
    - "//go:embed schema.sql applied idempotently on Open (D-10)"
    - "withTx (BEGIN IMMEDIATE via _txlock=immediate) as the single write entrypoint (D-07)"
key-files:
  created:
    - "go.mod"
    - "go.sum"
    - "store/memorystore.go"
    - "store/schema.sql"
    - "store/sqlite.go"
  modified: []
decisions:
  - "Adopted google/uuid (per CLAUDE.md no-one-time-helper) over the crypto/rand snippet for the agent id"
  - "Deferred the var _ MemoryStore = (*sqliteStore)(nil) conformance assertion to Plan 02 (partial impl must still compile)"
metrics:
  duration: 2min
  tasks: 2
  files: 5
  completed: 2026-07-26
---

# Phase 1 Plan 01: Storage Seam + Schema Foundation Summary

Bootstrapped the `mywholelife` Go module and laid the storage seam's foundation: the `MemoryStore`/`BlobStore` interface contracts, the idempotent two-table SQLite schema, and the single driver-confined `Open`/`withTx` path — the stable contract every later plan and phase implements against.

## What Shipped

- **`go.mod` / `go.sum`** — `module mywholelife`, Go 1.25.9; requires `modernc.org/sqlite v1.54.0` and `github.com/google/uuid v1.6.0` (both Approved in the RESEARCH Package Legitimacy Audit; resolved via goproxy.cn).
- **`store/memorystore.go`** — pure contract file (imports only `context`/`errors`, no driver/os/database-sql): full D-02 `MemoryStore` interface (11 methods) and D-03 `BlobStore` interface (4 methods), `Agent`/`Memory` model types with `int64` epoch-second timestamps (D-05), scope (`global`/`project`) and state (`recent`/`long_term`/`tombstone`) string consts matching the schema CHECK values, and `ErrNotImplemented`/`ErrNotFound` sentinels.
- **`store/schema.sql`** — idempotent DDL (all `IF NOT EXISTS`): `agents(id PK, name UNIQUE NOT NULL, created_at)`, `memories(mem_id PK, agent_id FK, scope CHECK, state CHECK, access_time, pinned DEFAULT 0, brief, rel_path, created_at)`, and `idx_memories_aging` on `(agent_id, scope, state, access_time)`.
- **`store/sqlite.go`** — the sole file blank-importing `_ "modernc.org/sqlite"` (D-01 seam firewall). `Open(dbPath, blobs)` opens with all four DSN pragmas (`busy_timeout(5000)`, `journal_mode(WAL)`, `foreign_keys(ON)`, `_txlock=immediate`), calls `SetMaxOpenConns(1)`, and applies the `//go:embed schema.sql` once via `ExecContext`. `withTx` wraps `BeginTx`/`Commit`/`Rollback` with panic-safe rollback — the intended single write entrypoint so `access_time` is stamped in-transaction (D-07). The `blobs BlobStore` field is injected so index-file ops route through the interface. Conformance assertion intentionally deferred to Plan 02.

## Tasks

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | Bootstrap module + define contracts + schema | 401eac4 | go.mod, go.sum, store/memorystore.go, store/schema.sql |
| 2 | DB open path + withTx + driver confinement | a6e3fd8 | store/sqlite.go |

## Verification

- `CGO_ENABLED=0 go build ./...` — passes (pure-Go static build / cgo + driver-name guard).
- `go vet ./store/...` — clean.
- Seam firewall: exactly one `*.go` file references `modernc.org/sqlite`, and it is `store/sqlite.go` (0 hits outside `store/`).
- Idempotency: an ephemeral test opened the same temp-file DB twice — second `Open` re-applied the schema with no error (removed after confirming; unit tests are Plan 02 per D-10).

## Deviations from Plan

### Verification-command portability (informational, no code change)

The Task 2 verify command `grep -rl ... . | grep -vc '^./store/'` assumes GNU grep emits a `./store/` prefix. On this machine `grep -r .` emits `store/sqlite.go` with no leading `./`, so the literal `^./store/` anchor mismatches and the raw command reports `1`. The seam is in fact correctly confined — the only match is `store/sqlite.go` inside `store/`. Confirmed with a prefix-tolerant check (`grep -vE '^(\./)?store/'` → 0). No code or file change; the acceptance criterion (driver imported in exactly one file under `store/`) is satisfied.

Otherwise: plan executed exactly as written. No Rule 1-4 auto-fixes were required.

## Known Stubs

`MemoryStore.Compress` and `MemoryStore.Reheat` are defined in the interface but have no bodies in this plan (no concrete `sqliteStore` methods yet). This is intentional and on-plan: all `MemoryStore` method bodies — including the `Compress`/`Reheat` `ErrNotImplemented` stubs (D-02, Pattern 4) — land in Plan 02, with the real compression bodies deferred to Phase 3. The `sqliteStore.blobs` field is injected but unread until Plan 02 wires index-file ops.

## Self-Check: PASSED

- go.mod — FOUND
- go.sum — FOUND
- store/memorystore.go — FOUND
- store/schema.sql — FOUND
- store/sqlite.go — FOUND
- Commit 401eac4 — FOUND
- Commit a6e3fd8 — FOUND
