---
phase: 02-hot-path-write-init
plan: 02
subsystem: api
tags: [gin, tar, zip, archive, skill-as-memory, bundler, path-traversal]

requires:
  - phase: 01-storage-seam-schema
    provides: store.MemoryStore/BlobStore interfaces, PutFolder/GetFolder single-blob ops, Put (state=recent + access_time=now in one withTx, project mem_id suffixing), List, GetIndex, ErrNotFound
  - phase: 02-hot-path-write-init
    provides: NewRouter(store.MemoryStore, store.BlobStore), register-first POST /agent/register, serve.go store wiring
provides:
  - "bundle package: ValidateAndBrief (tar path-guard + entry/byte caps + SKILL.md description brief) and WriteInitZip (stored .tar -> init .zip transcode)"
  - "POST /agent/:id/memory: 404 register-first, MaxBytesReader-bounded tar ingest, project/global scopes -> recent index row + one .tar blob"
  - "GET /agent/:id/init: 404 register-first, streamed application/zip bundle (memory/global, memory/long-term-memory.md stub, skills/<memId>/*)"
  - "agent-scoped global mem_id (global-<id>) resolving the plain-PK collision between two agents' global rows"
affects: [02-03, client-skill, session-end-hook, 03-remind-compression, 04-dream]

tech-stack:
  added: []
  patterns:
    - "single-.tar-blob per skill folder: whole folder serialized to one PutFolder blob (PutFolder is os.WriteFile, not a folder walker)"
    - "buffer-then-validate tar ingest: MaxBytesReader-bounded body read in full, validated for traversal + archive-bomb before any store write"
    - "streamed zip response: headers set before the first zip write; one memory's tar resident at a time"
    - "defense-in-depth path guard: safeEntryName rejects at ingest (400) AND filters at zip transcode"

key-files:
  created:
    - bundle/tar.go
    - bundle/bundle.go
    - bundle/tar_test.go
    - server/memory.go
    - server/init.go
    - server/roundtrip_test.go
  modified:
    - server/router.go

key-decisions:
  - "W1 fix: global mem_id is agent-scoped (global-<id>), not the literal 'global' — memories.mem_id is a plain PK, so a shared constant lets one agent's ON CONFLICT overwrite another's global row"
  - "server computes project mem_id = time.Now().Format(20060102)+'-'+project (OQ-3); client sends only ?project="
  - "global write is pinned=true at agents/<id>/global.tar; bundler fetches it by rel_path (agent-namespaced), never via the index"
  - "long-term-memory.md kept as a placeholder-stub slot in the zip (Phase 4 fills it)"

patterns-established:
  - "bundle package holds tar-validate + zip-transcode off the HTTP path (unit-testable over []byte), imports only stdlib + store interfaces"
  - "handlers stay thin (~20-40 lines): GetAgent guard -> validate -> PutFolder + Put / WriteInitZip"

requirements-completed: [SKILL-01, WRITE-01, INIT-01]

duration: 18min
completed: 2026-07-26
---

# Phase 2 Plan 02: Write + Init Hot Loop Summary

**A `bundle` package that validates an incoming skill-folder tar (path-traversal + archive-bomb guards, SKILL.md `description` brief) and transcodes stored `.tar` blobs into a streamed init `.zip`, wired to thin `POST /agent/:id/memory` and `GET /agent/:id/init` handlers — proven end-to-end over a real temp-dir store, with the agent-scoped global mem_id fixing a cross-agent overwrite.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-07-26T17:45:00Z
- **Completed:** 2026-07-26T18:03:00Z
- **Tasks:** 3
- **Files modified:** 7 (6 created, 1 modified)

## Accomplishments
- `bundle.ValidateAndBrief`: iterates the tar, rejects any entry that is empty/absolute/`..`-escaping, enforces a 4096-entry and 32 MiB cumulative cap (archive-bomb guard), and extracts the SKILL.md frontmatter `description` (single-line, quoted, and folded `>`/`|` block scalar — no YAML dep).
- `bundle.WriteInitZip`: streams the D-06 layout — `memory/global/…` (by rel_path), `memory/long-term-memory.md` (placeholder stub on ErrNotFound), and `skills/<memId>/…` for every recent project skill — re-applying `safeEntryName` at the transcode boundary.
- `POST /agent/:id/memory`: register-first 404 guard, `http.MaxBytesReader` (8 MiB) body bound, validate → `PutFolder` one `.tar` blob → `store.Put` recent row; project vs global scope, missing project → 400, unknown scope → 400.
- `GET /agent/:id/init`: register-first 404 guard, zip headers set before the first write, streamed bundle.
- **W1 latent-bug fix:** global mem_id is `global-<id>` (agent-scoped) so two agents' global rows no longer collide on the plain `memories.mem_id` PK; regression test asserts each agent owns exactly one `global-<id>` row and each init returns its own global content.

## Task Commits

Each task was committed atomically (TDD tasks: test → feat):

1. **Task 1: bundle package (tar validate, brief, zip transcode)** — `d129029` (test, RED) → `8147085` (feat, GREEN)
2. **Task 2: write + init handlers, wire routes** — `96f23e0` (feat)
3. **Task 3: httptest round-trip + 404 + traversal + global-isolation** — `48fa252` (test)

## Files Created/Modified
- `bundle/tar.go` - safeEntryName, ValidateAndBrief (path guard + entry/byte caps + brief), frontmatterDescription
- `bundle/bundle.go` - WriteInitZip + copyTarIntoZip (stored .tar → init .zip transcode)
- `bundle/tar_test.go` - safeEntryName / ValidateAndBrief (traversal, absolute, over-count, over-byte) / frontmatterDescription cases
- `server/memory.go` - writeMemory handler (404, MaxBytesReader, validate, PutFolder + Put, agent-scoped global mem_id)
- `server/init.go` - initBundle handler (404, zip headers before first write, WriteInitZip)
- `server/router.go` - added POST /agent/:id/memory and GET /agent/:id/init to NewRouter
- `server/roundtrip_test.go` - round-trip, global bundle, unknown-id 404, traversal 400, W1 per-agent global isolation

## Decisions Made
- **W1 agent-scoped global mem_id** (mandated fix): `memories.mem_id` is a plain PRIMARY KEY (verified in schema.sql), so the literal `"global"` constant would let agent B's `Put` (`ON CONFLICT(mem_id) DO UPDATE`) overwrite agent A's row's rel_path while leaving agent B with no global row. Using `global-<id>` keeps rows per-agent; the blob path stays `agents/<id>/global.tar` and the bundler still maps it to `memory/global/…`.
- **Server-computed project mem_id** (OQ-3): `YYYYMMDD-projectName` built server-side from `time.Now()`; the client sends only `?project=`. The store's `nextProjectMemID` handles same-day suffixing.
- **rel_path uses the un-suffixed base mem_id** as the plan specifies — a single write per project per day round-trips byte-exact; same-day same-project re-writes share the blob path (accepted for MVP, per D-06 suffixing note).
- **long-term-memory.md placeholder stub** written on `ErrNotFound` so the zip slot is always present (Phase 4 fills real content).

## Deviations from Plan

None - plan executed exactly as written (the W1 agent-scoped global mem_id was a mandated fix carried in the plan prompt, applied in Task 2 with its regression test in Task 3).

## Issues Encountered
- A literal U+FEFF BOM character in a `strings.TrimLeft` string literal (for stripping a leading BOM off frontmatter) produced `invalid BOM in the middle of the file` at build. Replaced the literal with a Go string escape (backslash-u-feff); build and tests then passed. Resolved within Task 1 before the GREEN commit.

## Verification
- `go test ./bundle/ -count=1` — passes (safeEntryName, traversal/absolute reject, over-count + over-byte archive-bomb caps, frontmatter single-line/quoted/folded).
- `CGO_ENABLED=0 go build ./...` — passes.
- `go vet ./...` — clean.
- `go test ./... -count=1` — green: bundle unit tests + server round-trip/global/404/traversal/W1-isolation + Phase-1 store/seam tests (no regression).

## Threat Model Compliance
- **T-02-04 (tar/zip path traversal):** mitigated — `safeEntryName` rejects `..`/absolute at ingest (`ValidateAndBrief` → 400) AND filters at transcode (`copyTarIntoZip`); traversal test asserts a `../evil.md` entry never reaches the init zip.
- **T-02-05 (oversized upload / archive bomb):** mitigated — `http.MaxBytesReader` (8 MiB) on the body + 4096-entry and 32 MiB cumulative caps in `ValidateAndBrief`.
- **T-02-06 (unknown/forged id):** mitigated — `GetAgent` guard → 404 on both routes (no auto-create); FK(agent_id) blocks orphan writes. Cross-agent overwrite via a forged *known* id remains accepted for single-tenant MVP (D-02).
- **T-02-07 (cross-agent disclosure):** accept — id-as-scope; List/GetFolder are agent-namespaced by id in rel_path and query.

## Next Phase Readiness
- The recent-only hot loop is closed and proven: write → single `.tar` blob + recent index row → init `.zip`. 02-03 (client skill + SessionEnd hook + init/push scripts) can consume `POST /memory` (`Content-Type: application/x-tar`, `?scope=&project=`) and `GET /init` (unzip 1:1 into memory dir + `.claude/skills/`).
- Phase 3's `.tar` → `.tar.zst` is a rename-and-compress over the same blob path; Phase 4 fills the `long-term-memory.md` slot already present in the bundle.

## Self-Check: PASSED

All six created files present on disk; router.go modified; all task commits (`d129029`, `8147085`, `96f23e0`, `48fa252`) exist in history.

---
*Phase: 02-hot-path-write-init*
*Completed: 2026-07-26*
