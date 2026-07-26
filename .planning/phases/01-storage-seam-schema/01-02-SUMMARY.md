---
phase: 01-storage-seam-schema
plan: 02
subsystem: database
tags: [storage, sqlite, memorystore, access-time, seam, uuid, tdd]

# Dependency graph
requires:
  - phase: 01-01
    provides: "MemoryStore/BlobStore contracts, Agent/Memory types, schema.sql, Open() + withTx() driver-confined adapter"
provides:
  - "Full sqliteStore implementation of MemoryStore (RegisterAgent/GetAgent/Put/Get/List/Touch/Forget/GetIndex/PutIndex + Compress/Reheat stubs)"
  - "In-transaction access_time stamping on every write path (D-07)"
  - "Pinned global-recent exemption from the aging/forgetting query (D-08)"
  - "YYYYMMDD-projectName same-day collision suffixing (D-06)"
  - "Compile-time conformance assertion var _ MemoryStore = (*sqliteStore)(nil)"
  - "Automated driver-confinement seam test (STORE-02)"
affects: [Phase 1 Plan 03 (localBlobStore), Phase 2 (HTTP+bundler calls MemoryStore), Phase 3 (Compress/Reheat bodies), Phase 4 (dream)]

# Tech tracking
tech-stack:
  added:
    - "github.com/google/uuid v1.6.0 promoted to a direct dependency (agent UUIDv4 id)"
    - "modernc.org/sqlite/lib (sqlite3) for the SQLITE_CONSTRAINT_UNIQUE extended code constant"
  patterns:
    - "Every write routed through withTx so access_time is stamped in the same transaction (D-07)"
    - "Aging query carries WHERE pinned = 0 AND scope <> 'global' as the belt-and-suspenders pin exemption (D-08)"
    - "In-transaction check-then-suffix mem_id allocation (never PK-error-driven retry, Pitfall 5)"
    - "Constraint classification via errors.As against a local interface{ Code() int } + extended code 2067"
    - "Seam enforced by a Go test that parses repo imports (mechanical CI check, not review)"

key-files:
  created:
    - "store/store_test.go"
    - "store/seam_test.go"
  modified:
    - "store/sqlite.go"
    - "go.mod"
    - "go.sum"

key-decisions:
  - "Blank driver import (registration/seam marker) kept in sqlite.go; the extended-code constant comes from the named modernc.org/sqlite/lib import, and constraint errors are matched via a local interface{ Code() int } so the main driver package is not imported by name"
  - "Put allocates the collision-free project mem_id inside its own withTx via nextProjectMemID; global/reserved keys upsert idempotently (ON CONFLICT DO UPDATE)"
  - "Forget is a CHECK-guarded UPDATE with pinned=0 AND scope<>'global'; exempt rows are a silent no-op, missing rows return ErrNotFound"
  - "Added agingCandidates as the internal aging query now (used by the D-07/D-08 tests and inherited by Phase 4 forgetting)"
  - "Seam test named TestSeamDriverConfinement so the plan's `-run Seam` verify command actually exercises it"

patterns-established:
  - "MemoryStore write paths never touch access_time outside withTx"
  - "Nullable brief/rel_path scanned via sql.NullString"

requirements-completed: [STORE-01, STORE-02, STORE-03, STORE-04, STORE-05, STORE-06]

# Metrics
duration: 14min
completed: 2026-07-26
---

# Phase 1 Plan 02: Full sqliteStore Index Surface + Seam Test Summary

**Complete `sqliteStore` MemoryStore implementation — UUIDv4 agent identity, in-transaction access_time stamping, YYYYMMDD-projectName collision suffixing, pinned global-recent aging exemption, and typed Compress/Reheat stubs — proven by a temp-file-DB test suite and a mechanical driver-confinement seam test.**

## Performance

- **Duration:** ~14 min
- **Started:** 2026-07-26T16:33:00Z
- **Completed:** 2026-07-26T16:47:06Z
- **Tasks:** 3 (2 TDD)
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- `sqliteStore` now implements the entire `MemoryStore` surface; `var _ MemoryStore = (*sqliteStore)(nil)` compiles (D-02 complete except the Phase-3 Compress/Reheat bodies).
- `RegisterAgent` generates a UUIDv4 id and maps a duplicate-name UNIQUE violation (extended code 2067) to the typed `ErrDuplicateName`.
- Every write path (`Put`, `Touch`, `Forget`) runs through `withTx` and stamps `access_time = now` in the same transaction — the D-07 regression proves aging keys on `access_time`, not `created_at`.
- The aging query (`agingCandidates`) and `Forget` both carry `WHERE pinned = 0 AND scope <> 'global'`; a pinned global-recent is never an aging candidate and is never tombstoned (D-08).
- `Put` allocates collision-free `YYYYMMDD-projectName` keys in-transaction (`-2`, `-3`, …) via check-then-suffix (D-06/STORE-06).
- `GetIndex`/`PutIndex` round-trip bytes through the injected `BlobStore` interface (no concrete adapter imported — keeps this plan independent of Plan 03).
- `store/seam_test.go` parses every repo `.go` file and fails if the driver module is imported outside `store/` or if more than one file blank-imports it (verified to catch a leak via a probe).

## Task Commits

1. **Task 1: Agent identity + in-transaction writes** (TDD)
   - `aac6c03` test — failing agent/write/touch/access-time/collision/open tests
   - `898ec5b` feat — RegisterAgent, GetAgent, Put, Touch, nextProjectMemID, agingCandidates
2. **Task 2: Listing/lifecycle/index/stubs — full conformance** (TDD)
   - `e167c35` test — Get/List/Forget/pin-exemption/index/stub tests
   - `382a9bc` feat — Get, List, Forget, GetIndex/PutIndex, Compress/Reheat stubs, conformance assertion
3. **Task 3: Seam enforcement test**
   - `33c7aef` test — driver-confinement walk
   - `9f248d0` test — rename to match `-run Seam` filter

## Files Created/Modified

- `store/sqlite.go` — added RegisterAgent/GetAgent/Put/Get/List/Touch/Forget/GetIndex/PutIndex, Compress/Reheat stubs, nextProjectMemID, agingCandidates, constraintCode, scanMemory, indexPath, ErrDuplicateName, conformance assertion.
- `store/store_test.go` — temp-file-DB suite with a map-backed fake BlobStore, raw-insert helper, and 11 tests covering register/lookup, writes, Touch, D-07 regression, collision, Get/List, Forget matrix + CHECK rejection, D-08 pin exemption, index round-trip, stubs, idempotent Open (W1).
- `store/seam_test.go` — go/parser-based STORE-02 driver-confinement test.
- `go.mod` / `go.sum` — `google/uuid` and `modernc.org/sqlite` promoted to direct requires; `go mod tidy`.

## Decisions Made

- **Driver imports:** kept the blank `_ "modernc.org/sqlite"` (registration + seam marker) and added a named `sqlite3 "modernc.org/sqlite/lib"` for the extended constraint-code constant. Constraint errors are classified via `errors.As` against a local `interface{ Code() int }`, avoiding a redundant named import of the main driver package. Both driver-module imports live in `store/sqlite.go`; only the blank import is a driver registration.
- **Project key allocation lives in Put:** `Put` computes the free key in its own transaction for project scope, so callers get server-guaranteed uniqueness through the fixed `Put(Memory) error` signature; global/reserved keys upsert idempotently.
- **agingCandidates added now:** the aging/forgetting query with the pin exemption is implemented in this plan (used by the D-07/D-08 tests) rather than deferred, giving Phase 4 forgetting a ready query and satisfying the `pinned = 0` verification grep.

## Deviations from Plan

### Auto-fixed / adjustments

**1. [Rule 3 - Blocking] Seam test renamed to match the documented verify command**
- **Found during:** Task 3
- **Issue:** The plan's verify command is `go test ./store/... -run Seam`. The initial test name `TestDriverConfinedToStore` did not contain "Seam", so `-run Seam` reported "no tests to run" (a false pass that never executed the assertion).
- **Fix:** Renamed to `TestSeamDriverConfinement`; `-run Seam` now runs it and passes.
- **Files modified:** store/seam_test.go
- **Verification:** `go test ./store/... -run Seam -v` shows `--- PASS: TestSeamDriverConfinement`.
- **Committed in:** `9f248d0`

**2. [Rule 2 - Missing Critical] NULL-safe scanning of brief/rel_path**
- **Found during:** Task 2
- **Issue:** `brief`/`rel_path` are nullable columns; scanning a NULL into a `string` errors. Rows written by `Put` are non-NULL, but defensive correctness for any NULL row (and future writers) requires it.
- **Fix:** `scanMemory` reads both via `sql.NullString`.
- **Files modified:** store/sqlite.go
- **Verification:** Full suite green.
- **Committed in:** `382a9bc` (Task 2 commit)

---

**Total deviations:** 2 (1 blocking verify-command alignment, 1 missing-critical robustness). No architectural changes (no Rule 4). No scope creep.

## Threat Register Compliance

- **T-02-01 (SQL tampering):** every statement is `?`-parameterized; no identifier is concatenated into SQL. The `scope <> 'global'` / `state = 'recent'` literals are fixed schema values, not caller input.
- **T-02-02 (weak agent id):** id is `uuid.NewString()` (crypto/rand-backed), never `math/rand`.
- **T-02-03 (pinned/global aging EoP):** `pinned = 0 AND scope <> 'global'` on both `agingCandidates` and `Forget`; regression-tested by `TestPinnedGlobalExemption`.

## Issues Encountered

- Stray `path`/`time` imports surfaced as build failures at the Task 1 GREEN step (`path` is only used by the Task 2 `indexPath`); resolved by adding `path` back with the Task 2 implementation. No functional impact.

## User Setup Required

None — no external service configuration required.

## Known Stubs

- `Compress`/`Reheat` return `ErrNotImplemented` by design (D-02, Pattern 4); real zstd bodies land in Phase 3. Callers assert `errors.Is(err, ErrNotImplemented)`. Not a blocking stub — the plan's goal (index behavior) is fully delivered.

## Verification

- `CGO_ENABLED=0 go test ./store/...` — passes (11 tests, static-build guard).
- `go test ./store/... -run Seam` — passes; a driver-leak probe outside `store/` was confirmed to fail the test.
- `go vet ./store/...` — clean. `gofmt -l store/` — clean.
- `grep -n 'pinned = 0' store/sqlite.go` — present in `agingCandidates` and `Forget`.

## Next Phase Readiness

- Plan 03 (`localBlobStore`) can proceed: `GetIndex`/`PutIndex` already exercise the `BlobStore` interface and a map-backed fake proves the round-trip.
- Phase 2 callers can build against the now-complete, conformance-asserted `MemoryStore`.
- No blockers introduced.

## Self-Check: PASSED

- store/sqlite.go — FOUND
- store/store_test.go — FOUND
- store/seam_test.go — FOUND
- .planning/phases/01-storage-seam-schema/01-02-SUMMARY.md — FOUND
- Commits aac6c03, 898ec5b, e167c35, 382a9bc, 33c7aef, 9f248d0 — FOUND

---
*Phase: 01-storage-seam-schema*
*Completed: 2026-07-26*
