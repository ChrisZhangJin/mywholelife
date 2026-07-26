---
phase: 04-dream-consolidation-forgetting
plan: 01
subsystem: database
tags: [sqlite, blob-store, index, soft-delete, tombstone, atomic-write, consistency-scan, migration]

# Dependency graph
requires:
  - phase: 01-storage-seam-schema
    provides: withTx, scanMemory, Get/List, blob resolve/confinement, GetIndex/PutIndex, MemoryStore/BlobStore contracts
  - phase: 03-cold-tier-compress-reheat
    provides: Compress/Reheat verify-before-delete, store/index.go parse/upsert/remove/validate helpers
provides:
  - "memories.deleted_at nullable column + idempotent PRAGMA-guarded ALTER migration in Open (D-05)"
  - "validateIndex widened to accept long_term AND tombstone rows so the D-06 gate never fails-loud on a legit tombstone"
  - "Reheat clears deleted_at (soft-delete is reversible, D-07)"
  - "Compress(ctx, agentID, memID, hook) seam — non-empty hook routed through m.Brief -> upsertIndexLine sanitize/cap (D-02); hook=='' preserves Phase-3 brief-derived line"
  - "SoftDelete(ctx, agentID, memID, when) — explicit-timestamp T3 mark, deleted_at IS NULL grace-clock guard, pinned/global exempt (D-04)"
  - "HardDelete(ctx, agentID, memID) — verify-before-destroy line->blob->row, each step idempotent (D-04/D-07)"
  - "CommitIndex — validate-before-rename; writeIndexAtomic single index-write path with .bak + tmp+rename; PutIndex delegates (D-06)"
  - "localBlobStore.PutFolder is now atomic (tmp+rename); BlobStore.Walk enumerates blobs for orphan detection"
  - "ScanConsistency(ctx, agentID) ScanReport — read-only FS/DB/index reconciliation (orphan blob, dangling row, torn compress, index mismatch)"
affects: [dream-job, 04-02-PLAN, 04-03-PLAN, forgetting, index-validation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single index-write path: writeIndexAtomic (.bak + tmp+rename); CommitIndex validates BEFORE touching disk (D-06)"
    - "Explicit-timestamp durable ops (SoftDelete when int64) so an injected clock reaches the durable layer deterministically"
    - "Verify-before-destroy, per-step idempotent HardDelete (line -> blob -> row) — interrupted run converges on re-run"
    - "Untrusted LLM hook confined to a single sanitize/cap path via m.Brief; no second index-write path, no TOCTOU"

key-files:
  created:
    - store/dream_store_test.go
  modified:
    - store/schema.sql
    - store/memorystore.go
    - store/sqlite.go
    - store/index.go
    - store/blob_fs.go
    - store/store_test.go
    - store/cold_test.go
    - store/index_test.go
    - server/remind_test.go

key-decisions:
  - "writeIndexAtomic routes .bak through the BlobStore interface (GetFolder prior -> PutFolder .bak -> PutFolder content) and makes localBlobStore.PutFolder itself tmp+rename atomic, rather than type-asserting blobs.resolve — keeps the seam + fake-store tests working while giving real-FS crash-safety for ALL blob writes"
  - "BlobStore gained a Walk method (needed by ScanConsistency); faultBlobStore inherits it via embedding, fakeBlobStore implements a map-prefix scan"
  - "Memory.DeletedAt is sql.NullInt64 (preserves NULL vs 0) so HardDelete's deleted_at-NOT-NULL precondition and the Reheat-clears-to-NULL assertion are exact"

patterns-established:
  - "Pattern 1 (validate-before-rename): CommitIndex asserts validateIndex against live rows, then writeIndexAtomic — a bad index never lands on disk, prior index + .bak intact"
  - "Pattern 2 (graduated reversible forgetting): tombstone keeps blob+line; SoftDelete stamps grace clock (guarded); HardDelete is the only irreversible step and is verify-before-destroy"

requirements-completed: []

# Metrics
duration: 14min
completed: 2026-07-26
---

# Phase 4 Plan 01: Dream Store Durable Layer Summary

**The surgical store primitives the dream job orchestrates: a nullable `deleted_at` soft-delete column with idempotent migration, a widened tombstone-aware `validateIndex`, a reversible `Reheat`, the untrusted-LLM `Compress(...,hook)` seam, `SoftDelete`/`HardDelete`, a validate-before-rename atomic `CommitIndex`, and a read-only `ScanConsistency`.**

## Performance

- **Duration:** ~14 min
- **Started:** 2026-07-26T19:04:00Z
- **Completed:** 2026-07-26T19:18:00Z
- **Tasks:** 3 (Task 3 test-only)
- **Files modified:** 10 (1 created)

## Accomplishments
- `deleted_at INTEGER` added to fresh DBs (schema.sql) AND to pre-existing DBs via a `pragma_table_info` guarded `ALTER TABLE ... ADD COLUMN` in `Open()` — no CHECK rebuild, idempotent on reopen (D-05).
- The two required corrections: `validateIndex` now accepts `{long_term, tombstone}` (the highest-risk integration bug — the gate would otherwise fail-loud on every tombstone), and `Reheat` clears `deleted_at` so a reminded soft-deleted tombstone is genuinely un-deleted (D-06/D-07).
- `Compress` widened to `Compress(ctx, agentID, memID, hook)` — the seam DREAM-02 depends on. A non-empty hook is set on `m.Brief` before the existing `upsertIndex`, so it flows through the SAME `capLen(sanitizeField(...),120)` path; `hook==""` is byte-for-byte the Phase-3 brief-derived behavior. No separate index-write path (no TOCTOU).
- `SoftDelete` (explicit `when`, `deleted_at IS NULL` grace-clock guard, pinned/global exempt) and verify-before-destroy `HardDelete` (line → blob → row, each idempotent).
- `CommitIndex` validates against live rows BEFORE any disk write; `writeIndexAtomic` keeps a `.bak` and `localBlobStore.PutFolder` now does tmp+rename — every index write is crash-safe (D-06).
- `ScanConsistency` reports orphan blobs, dangling rows, torn-compress pairs, and index/row mismatches read-only (D-07), backed by a new `BlobStore.Walk`.
- Seam intact: no `go-openai`/driver/compressor import escaped into or beyond `store/`.

## Task Commits

Each task committed atomically:

1. **Task 1: deleted_at column + migration + two required corrections** — `4be6188` (feat)
2. **Task 2: Compress hook seam + Soft/HardDelete + atomic CommitIndex + ScanConsistency** — `dcc2ff7` (feat)
3. **Task 3: store-layer tests for the new primitives** — `bd27e2f` (test)

**Plan metadata:** (docs commit — this file + STATE + ROADMAP)

## Files Created/Modified
- `store/schema.sql` - `deleted_at INTEGER` on fresh `memories`
- `store/memorystore.go` - `Memory.DeletedAt`, widened `Compress`, new `SoftDelete`/`HardDelete`/`CommitIndex`/`ScanConsistency` interface methods, `BlobStore.Walk`, `ScanReport`/`Inconsistency` types
- `store/sqlite.go` - `Open` migration guard; `deleted_at` in scanMemory/Get/List; Reheat clears deleted_at; Compress hook seam; `CommitIndex`/`writeIndexAtomic`/`SoftDelete`/`HardDelete`/`ScanConsistency`; `PutIndex` delegates to atomic path
- `store/index.go` - `validateIndex` widened to long_term+tombstone
- `store/blob_fs.go` - `PutFolder` tmp+rename atomic; new `Walk`
- `store/store_test.go` - `fakeBlobStore.Walk` (interface satisfaction)
- `store/cold_test.go` - 3 `Compress` call sites updated to new arity
- `store/index_test.go` - `TestValidateIndexTombstone`
- `store/dream_store_test.go` - migration, hook seam, Reheat-clears-deleted_at, SoftDelete, HardDelete, CommitIndex, ScanConsistency tests
- `server/remind_test.go` - 2 `Compress` call sites updated to new arity

## Decisions Made
- **Atomic index write via the interface, not `blobs.resolve`:** the RESEARCH sketch calls `s.blobs.resolve(...)` + raw `os` calls, but `s.blobs` is the `BlobStore` interface (fake in tests, no `resolve`). Instead `writeIndexAtomic` reads the prior index and writes `<path>.bak` through the interface, and `localBlobStore.PutFolder` itself became tmp+rename atomic. This keeps the seam + `fakeBlobStore` tests intact while giving real-FS crash-safety to every blob write, and satisfies the RESEARCH "single index-write path" recommendation.
- **`Memory.DeletedAt` is `sql.NullInt64`:** preserves the NULL-vs-0 distinction that `HardDelete`'s precondition and the reversibility assertion depend on. This adds a `database/sql` import to `memorystore.go`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `BlobStore` gained a `Walk` method → `fakeBlobStore` must implement it**
- **Found during:** Task 2 (ScanConsistency needs blob enumeration)
- **Issue:** `ScanConsistency` requires enumerating blobs; the plan directs adding `Walk` to `BlobStore`/`localBlobStore`. Adding it to the interface breaks `fakeBlobStore` (in `store/store_test.go`, not in the plan's file list) which implements each method explicitly.
- **Fix:** Implemented `fakeBlobStore.Walk` as a map-prefix scan; `faultBlobStore` inherits `Walk` via its embedded `BlobStore`.
- **Files modified:** store/store_test.go
- **Verification:** `go build ./...` + full `go test ./...` green.
- **Committed in:** `dcc2ff7`

**2. [Rule 3 - Blocking] Out-of-`store/` `Compress` call sites updated for the new arity**
- **Found during:** Task 2 (Compress signature widened)
- **Issue:** `server/remind_test.go` has 2 `st.Compress(ctx, id, memID)` calls that would fail to compile with the new 4-arg signature.
- **Fix:** Passed `""` (preserves the brief-derived assertions).
- **Files modified:** server/remind_test.go
- **Verification:** `go test ./server/... -count=1` green.
- **Committed in:** `dcc2ff7`

---

**Total deviations:** 2 auto-fixed (both Rule 3 - blocking compile fixes from the interface widening).
**Impact on plan:** Both are mechanical consequences of the planned signature/interface changes. No scope creep.

## Issues Encountered
None — all three tasks' verification gates passed on first run after implementation.

## Requirements Status

INDEX-02, DREAM-03, DREAM-04 are the phase-spanning requirements this plan **provides the durable primitives for** (index validate-before-commit + atomic write; tombstone/soft-delete/hard-delete lifecycle with grace clock; per-item idempotent recoverability + consistency scan). They are **not marked complete** in REQUIREMENTS.md yet — full satisfaction requires the `dream/` orchestrator (Plan 02) and the `dream` subcommand + rate-limit/grace wiring (Plan 02/03). Left as `Pending` to avoid closing multi-plan requirements prematurely.

## TDD Gate Compliance
Task 3 is a test-only task characterizing behavior implemented in Tasks 1-2 (feat commits precede the test commit). Its `<files>` are all `_test.go`, so it is not behavior-adding — the MVP+TDD runtime gate does not apply.

## Threat Flags
None — no new security surface beyond the plan's `<threat_model>`. T-04-01 (atomic validate-before-rename index write) and T-04-07 (hook confined to the sanitize/cap path) are mitigated as planned; T-04-02/T-04-03 (verify-before-destroy HardDelete, grace-clock guard) are in the store layer, with rate-limit/grace comparison deferred to the caller per plan.

## Self-Check: PASSED
- `store/dream_store_test.go` created; all modified files present.
- Commits `4be6188`, `dcc2ff7`, `bd27e2f` all in `git log`.
- `CGO_ENABLED=0 go build ./...`, `go vet ./...`, `go test ./... -count=1` all green.
- Seam: `rg "go-openai" store/` → none.

## Next Phase Readiness
- All durable primitives the dream job drives are in place and tested: soft/hard delete lifecycle, reversible Reheat, atomic validate-before-rename index commit, tombstone-aware validator, the `Compress` hook seam, and the consistency scan.
- Wave 2 (Plan 02) can now build `dream/` as a pure orchestrator over `store.MemoryStore` with a fake `HookGen` + injected clock; the `when int64` SoftDelete signature lets the fake clock reach durable state.
- Rate limit (`DREAM_MAX_DELETIONS`) and grace-window (`now - deleted_at > GRACE`) comparisons are intentionally NOT in the store — they are the dream job's orchestration (Plan 02).

---
*Phase: 04-dream-consolidation-forgetting*
*Completed: 2026-07-26*
