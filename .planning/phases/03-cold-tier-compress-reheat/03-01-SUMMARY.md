---
phase: 03-cold-tier-compress-reheat
plan: 01
subsystem: database
tags: [zstd, klauspost-compress, sqlite, blob-store, compression, index]

# Dependency graph
requires:
  - phase: 01-store-foundation
    provides: withTx, Touch, Get, blob put/get/delete confinement, GetIndex/PutIndex, Compress/Reheat stubs
  - phase: 02-write-init
    provides: single-.tar blob layout, validKey, bundle tar caps
provides:
  - "store/zstd.go: []byte zstdCompress/zstdDecompress (sole zstd importer, D-01 seam)"
  - "store/index.go: structured long-term-memory.md parse/upsert/remove/validate helpers"
  - "Compress: recent .tar -> verified .tar.zst, state=long_term, index upserted, source deleted last"
  - "Reheat: .tar.zst -> verified .tar, state=recent, access_time=now, index line removed, idempotent Touch on recent"
affects: [remind-endpoint, dream-job, forgetting, phase-4]

# Tech tracking
tech-stack:
  added: [github.com/klauspost/compress v1.19.1]
  patterns:
    - "Write-Verify-Commit-Delete: source artifact deleted only after read-back round-trip verify AND DB pointer commit (D-02)"
    - "zstd confined to store/zstd.go (compressor seam, D-01)"
    - "Structured markdown index: idempotent upsert/remove, memId-last, 1:1 validator (D-06/D-07)"

key-files:
  created:
    - store/zstd.go
    - store/index.go
    - store/index_test.go
    - store/cold_test.go
  modified:
    - store/sqlite.go
    - store/store_test.go
    - go.mod
    - go.sum

key-decisions:
  - "Round-trip verify reads the persisted .tar.zst back (not the in-memory comp) before deleting the source — catches write truncation/corruption, and makes the negative path fault-injectable"
  - "Used one-shot EncodeAll/DecodeAll instead of streaming NewWriter/Close (dodges the Close()-truncation footgun; blob is already a single []byte)"
  - "WithDecoderMaxMemory(32MB) + a belt-and-suspenders len(out) cap as the decompression-bomb guard (T-03-02)"
  - "index name derived by stripping a leading YYYYMMDD- date prefix from memId; hook from Brief, sanitized and capped at 120 chars"

patterns-established:
  - "Pattern 1 (Write-Verify-Commit-Delete): the D-02 never-lose-a-memory invariant for both Compress and Reheat"
  - "Pattern 2 (idempotent structured index): stable header, memId-last lines, byte-identical re-upsert"

requirements-completed: [COMP-01, COMP-02, RECALL-02, INDEX-01]

# Metrics
duration: 6min
completed: 2026-07-26
---

# Phase 3 Plan 01: Cold-tier zstd archival + structured index Summary

**Lossless `.tar`↔`.tar.zst` archival with read-back round-trip verify-before-delete, plus an idempotent structured `long-term-memory.md`, filling the Phase-1 Compress/Reheat stubs.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-07-26T18:17:27Z
- **Completed:** 2026-07-26T18:22:57Z
- **Tasks:** 2 (both TDD)
- **Files modified:** 8

## Accomplishments
- `store/zstd.go` — the ONLY file importing `klauspost/compress/zstd`; one-shot `EncodeAll`/`DecodeAll` `[]byte` helpers with a 32 MB decompression-bomb cap (D-01 seam intact).
- `store/index.go` — `parseIndex`/`upsertIndexLine`/`removeIndexLine`/`validateIndex` for the structured `- <name> | <hook> | <memId>` index; idempotent, memId-last, 1:1 validator (D-06/D-07).
- `Compress` — recent `.tar` → verified `.tar.zst`, `state=long_term`, `access_time` untouched (aging), index upserted, source `.tar` deleted LAST (COMP-01, COMP-02).
- `Reheat` — inverse: verified `.tar`, `state=recent`, `access_time=now`, index line removed, `.tar.zst` deleted LAST; idempotent `Touch` when already recent (RECALL-02).
- Blocker fix: replaced the `ErrNotImplemented`-asserting `TestCompressReheatStubs` with real round-trip / verify-failure / idempotency / not-found coverage in `store/cold_test.go`.

## Task Commits

TDD tasks — each has a test (RED) then feat (GREEN) commit:

1. **Task 1 (RED): failing zstd + index tests** — `35178bb` (test)
2. **Task 1 (GREEN): zstd helpers + structured index** — `444a7be` (feat)
3. **Task 2 (RED): failing Compress/Reheat tests** — `bf828e1` (test)
4. **Task 2 (GREEN): Compress/Reheat bodies + stub-test removal** — `fc416dd` (feat)

## Files Created/Modified
- `store/zstd.go` - sole zstd importer; `zstdCompress`/`zstdDecompress` + 32 MB cap
- `store/index.go` - structured long-term index parse/upsert/remove/validate
- `store/index_test.go` - zstd round-trip/corrupt/bomb + index idempotency/validation
- `store/cold_test.go` - Compress↔Reheat round-trip, verify-before-delete negative path, access_time rules, idempotent-recent, not-found
- `store/sqlite.go` - real `Compress`/`Reheat` bodies + `upsertIndex`/`removeIndex` helpers; `bytes`/`strings` imports
- `store/store_test.go` - removed `TestCompressReheatStubs` (stubs replaced)
- `go.mod` / `go.sum` - `github.com/klauspost/compress v1.19.1` (now direct)

## Decisions Made
- **Read-back verify:** the round-trip verify decompresses the *persisted* `.tar.zst` (re-read via the blob store), not the in-memory `comp`. Stronger than the RESEARCH sketch (catches write-side truncation), and it makes the required verify-failure negative path fault-injectable via a `BlobStore` wrapper. Deviation from the literal sketch, consistent with D-02's intent.
- **One-shot codec:** `EncodeAll`/`DecodeAll` over the streaming `NewWriter`/`Close` form named loosely in D-01 (explicitly permitted by RESEARCH discretion; avoids the Close()-truncation footgun).
- **Bomb guard:** `WithDecoderMaxMemory(32<<20)` plus a defensive `len(out)` check (T-03-02).

## Deviations from Plan

### Auto-fixed / design refinements

**1. [Rule 1 - Correctness] Verify against the persisted artifact, not in-memory bytes**
- **Found during:** Task 2 (Compress/Reheat + negative-path test)
- **Issue:** The RESEARCH sketch verifies `zstdDecompress(comp)` (in-memory), which can never detect a bad *write* — the exact failure that would cause data loss when the source is deleted. It is also not fault-injectable via a `BlobStore` wrapper (the plan's preferred negative-path mechanism).
- **Fix:** Verify reads the just-written `.tar.zst`/`.tar` back through the blob store, decompresses, and `bytes.Equal`-compares before committing the pointer move and deleting the source.
- **Files modified:** store/sqlite.go
- **Verification:** `TestCompressVerifyFailureKeepsSource` (fault-injecting `BlobStore` corrupts the `.tar.zst`) asserts source survives, bad artifact removed, state stays recent.
- **Committed in:** fc416dd

---

**Total deviations:** 1 design refinement (correctness-strengthening).
**Impact on plan:** Strengthens the D-02 guarantee and enables the required negative-path test. No scope creep.

## Issues Encountered
- `klauspost/compress` landed as `// indirect` after `go get`; `go mod tidy` promoted it to a direct require once `store/zstd.go` imported it. Seam check confirms it is the only importer.

## Threat Flags
None — no security surface introduced beyond the plan's `<threat_model>`. T-03-01 (delete ordering) and T-03-02 (decompression bomb) are both mitigated as planned.

## Self-Check: PASSED
- store/zstd.go, store/index.go, store/index_test.go, store/cold_test.go present.
- Commits 35178bb, 444a7be, bf828e1, fc416dd all in `git log`.
- `CGO_ENABLED=0 go build ./...`, `go vet ./...`, `go test ./... -count=1` all green.
- `grep -rl "klauspost/compress" --include=*.go .` → only `store/zstd.go`.

## Next Phase Readiness
- `Compress`/`Reheat` mechanisms are ready for Plan 02 (remind endpoint over `store.Reheat`) and Plan 03 (client `remind.sh`).
- Index upsert/remove are idempotent and the format is stable, so Phase 4's dream job can layer full consolidation on top cleanly.
- RECALL-01 / RECALL-03 (remind endpoint + client) are Plan 02/03, not covered here.

---
*Phase: 03-cold-tier-compress-reheat*
*Completed: 2026-07-26*
