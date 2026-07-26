---
phase: 01-storage-seam-schema
plan: 03
subsystem: infra
tags: [storage, blobstore, filesystem, path-traversal, security, seam, tdd]

# Dependency graph
requires:
  - phase: 01-01
    provides: "BlobStore interface (PutFolder/GetFolder/Delete/Exists), ErrNotFound, D-09 layout contract"
provides:
  - "localBlobStore filesystem adapter implementing BlobStore over the D-09 on-disk layout"
  - "rel_path confinement: filepath.Clean + reject absolute and ..-escaping paths before any disk write (ErrUnsafePath)"
  - "NewBlobStore(root) constructor rooting the adapter at an absolute data root"
  - "Compile-time conformance assertion var _ BlobStore = (*localBlobStore)(nil)"
  - "/data/ gitignored so runtime agent folders/archives are never committed"
affects: [Phase 2 (HTTP+bundler PutFolder/GetFolder with client-supplied keys), Phase 3 (Compress/Reheat layer zstd over BlobStore)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single internal resolve(relPath) gate applied by every method — clean + confine before touching disk"
    - "Cleaned absolute path must equal the data root or be prefixed by root+os.PathSeparator, else ErrUnsafePath"
    - "os/path/filepath imports confined to the blob adapter within store/ (disk half of the STORE-02 firewall)"

key-files:
  created:
    - "store/blob_fs.go"
    - "store/blob_test.go"
  modified:
    - ".gitignore"

key-decisions:
  - "resolve() rejects absolute rel_paths outright and confines cleaned paths via a root-prefix check (RESEARCH V12)"
  - "Delete is idempotent (os.RemoveAll) — removing a missing path is a no-op, not an error; tested explicitly"
  - "GetFolder maps os.ErrNotExist to the shared store.ErrNotFound, matching the map-backed fake used by 01-02"
  - "Constructor named NewBlobStore returning *localBlobStore; no helpers beyond resolve (CLAUDE.md lean rule)"

patterns-established:
  - "Every filesystem-touching method routes rel_path through resolve() before any I/O"
  - "Path-traversal defense unit-tested for .., nested-escape, and absolute inputs across all four methods"

requirements-completed: [STORE-02]

# Metrics
duration: 7min
completed: 2026-07-26
---

# Phase 1 Plan 03: localBlobStore Filesystem Adapter Summary

**Disk half of the STORE-02 seam: a `localBlobStore` FS adapter that lays out folders per the D-09 layout (`agents/<id>/global`, `agents/<id>/projects/<YYYYMMDD-projectName>`) and confines every `rel_path` under the data root, rejecting absolute and `..`-escaping paths before any write.**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-07-26T16:49:00Z
- **Completed:** 2026-07-26T16:56:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- `localBlobStore` implements the full `BlobStore` interface; `var _ BlobStore = (*localBlobStore)(nil)` compiles — the concrete disk backend behind `GetIndex`/`PutIndex` and future client uploads.
- A single internal `resolve(relPath)` cleans and confines every path: absolute paths are rejected, and a cleaned path must stay prefixed by the data root, so no `memId`/key can escape the root (path-traversal defense for Phase 2 uploads, T-03-01).
- Folder bytes round-trip through `PutFolder`/`GetFolder`/`Exists`/`Delete` under the D-09 layout with `rel_path` relative to the data root; `GetFolder` maps missing files to the shared `ErrNotFound`, and `Delete` is idempotent.
- `store/blob_fs.go` is the only non-test file in the repo importing `os`/`path/filepath` for the data root — the disk half of the STORE-02 firewall.
- `/data/` added to `.gitignore` so runtime agent folders and archives (T-03-02) are never committed.

## Task Commits

TDD cycle (RED → GREEN) plus a scoped chore commit:

1. **Task 1: localBlobStore FS adapter with path confinement** (TDD)
   - `d20162e` test — failing round-trip, Exists/Delete, and traversal-rejection tests against `NewBlobStore`/`ErrUnsafePath`
   - `960dd96` feat — `localBlobStore` (NewBlobStore, resolve, PutFolder/GetFolder/Delete/Exists, conformance assertion)
   - `6621ca8` chore — gitignore runtime `/data/`

**Plan metadata:** committed with STATE.md/ROADMAP.md/REQUIREMENTS.md in the final docs commit.

## Files Created/Modified

- `store/blob_fs.go` — `localBlobStore` adapter: `NewBlobStore(root)`, `resolve()` (clean + confine, `ErrUnsafePath`), `PutFolder` (MkdirAll parent + WriteFile), `GetFolder` (ReadFile → `ErrNotFound` on missing), `Delete` (idempotent RemoveAll), `Exists` (Stat), and the `var _ BlobStore` assertion.
- `store/blob_test.go` — `t.TempDir()` data root; `TestBlobRoundTrip` (global + projects/<key> round-trip, byte-identical, on-disk D-09 path assertion), `TestBlobExistsAndDelete` (Exists/Delete semantics incl. idempotent delete + `ErrNotFound`), `TestBlobTraversalRejected` (`..`, nested-escape, absolute rejected across all four methods; no escaped write on disk).
- `.gitignore` — appended `/data/`.

## Decisions Made

- **Confinement via prefix check:** `resolve()` cleans `filepath.Join(root, relPath)` and requires the result to equal `root` or be prefixed by `root + os.PathSeparator`; absolute `relPath` is rejected up front. Chosen over `filepath.Rel` parsing for a direct, auditable check (RESEARCH Security Domain V12).
- **Idempotent Delete:** `os.RemoveAll` — deleting a non-existent path is a no-op, not an error (behavior-block "pick one"); explicitly asserted.
- **Constructor `NewBlobStore`:** mirrors the `store.Open` style; returns the unexported `*localBlobStore`, keeping the concrete type private while satisfying the exported `BlobStore` interface. No helpers beyond `resolve` (CLAUDE.md).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. RED failed to compile (undefined `NewBlobStore`/`ErrUnsafePath`) as expected; GREEN passed all Blob tests and the full `store` suite on first run.

## Threat Register Compliance

- **T-03-01 (Tampering/EoP via rel_path):** mitigated — `resolve()` rejects absolute and `..`-escaping paths before any disk I/O; `TestBlobTraversalRejected` proves `../escape.txt`, `a/../../b`, an absolute path, and `/etc/passwd` are all rejected across PutFolder/GetFolder/Delete/Exists, and that nothing was written outside the data root.
- **T-03-02 (Information disclosure via committed runtime data):** mitigated — `/data/` added to `.gitignore`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 2 (HTTP + bundler) can wire `NewBlobStore(dataRoot)` into `store.Open` as the concrete `BlobStore`; client-supplied keys are already traversal-hardened.
- Phase 3 (`Compress`/`Reheat`) can layer zstd over `PutFolder`/`GetFolder` without changing the interface.
- STORE-02 (both index and disk halves) is complete; no blockers introduced.

## Verification

- `CGO_ENABLED=0 go test ./store/... -run Blob -count=1` — passes (3 Blob tests).
- `CGO_ENABLED=0 go test ./store/... -count=1` — passes (15 tests: 12 existing + 3 new, no regression).
- `go vet ./store/...` — clean. `gofmt -l store/` — clean. `CGO_ENABLED=0 go build ./...` — clean.
- `grep -rl 'path/filepath\|"os"' store/*.go | grep -v _test.go` → only `store/blob_fs.go` (filesystem imports confined).

## Self-Check: PASSED

- store/blob_fs.go — FOUND
- store/blob_test.go — FOUND
- .gitignore contains /data/ — FOUND
- Commits d20162e, 960dd96, 6621ca8 — FOUND

---
*Phase: 01-storage-seam-schema*
*Completed: 2026-07-26*
