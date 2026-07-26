---
phase: 03-cold-tier-compress-reheat
plan: 02
subsystem: api
tags: [gin, remind, zstd, tar, httptest, reheat]

# Dependency graph
requires:
  - phase: 03-01
    provides: real store.Compress/Reheat, zstd helpers, structured long-term-memory.md index
  - phase: 02-02
    provides: writeMemory/initBundle handler patterns, validKey, single-.tar blob layout
provides:
  - "GET /agent/:id/remind?mem= endpoint — reverses the time arrow (long_term → recent)"
  - "Thin remind handler over store.Reheat streaming application/x-tar"
  - "httptest coverage: round-trip, 400/404 contract, D-08 init-reflects-index"
affects: [phase-04-dream, client-remind.sh]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Remind handler mirrors writeMemory/initBundle: validKey → GetAgent 404 → store op → c.Data"
    - "Handler tier imports no compressor — seam holds; only store owns zstd"

key-files:
  created:
    - server/remind.go
    - server/remind_test.go
  modified:
    - server/router.go

key-decisions:
  - "Handler streams the reheated .tar via m.RelPath (read after Reheat) — never reconstructs the path"
  - "validKey(id) && validKey(mem) both gate before any store call — rejects path traversal in the query param"
  - "access_time bump lives entirely in store.Reheat; the handler never touches it"

patterns-established:
  - "Remind endpoint: thin gin handler delegating cold-tier mechanics to store.Reheat"
  - "httptest asserts .tar body via archive/tar parse + state/access_time via st.Get"

requirements-completed: [RECALL-01, RECALL-02, INDEX-01]

# Metrics
duration: 9min
completed: 2026-07-26
---

# Phase 3 Plan 02: Remind Endpoint Summary

**`GET /agent/:id/remind?mem=` reheats a cold memory via store.Reheat and streams the restored skill folder back as application/x-tar, with validKey/404 guards and full httptest coverage.**

## Performance

- **Duration:** ~9 min
- **Started:** 2026-07-26T18:25:00Z
- **Completed:** 2026-07-26T18:34:00Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- Thin `remind` handler mirroring `writeMemory`/`initBundle`: validates `id`+`mem`, 404s unknown agent/memory, calls `store.Reheat`, streams the restored `.tar` with `Content-Disposition`.
- Route `GET /agent/:id/remind` registered in `NewRouter` alongside the existing memory/init routes.
- httptest suite: happy-path round-trip (200 + application/x-tar + SKILL.md byte-match + recent state + bumped access_time + init reflects memory back in skills/ and dropped from long-term index), 400 on bad id/mem, 404 on unknown agent/memory, and a dedicated D-08 test that init's long-term-memory.md carries the compressed line (not the placeholder).
- Seam verified: `grep klauspost/compress server/` returns nothing.

## Task Commits

1. **Task 1: Add remind handler and register the route** - `9d28a10` (feat)
2. **Task 2: httptest coverage for remind + init-reflects-index** - `b1abf3b` (test)

_Task 2 is a test-only file exercising the Task 1 implementation; the plan structures the impl (Task 1) ahead of its coverage (Task 2), so there is a single test commit rather than a RED/GREEN pair._

## Files Created/Modified
- `server/remind.go` - `remind(st, bl)` gin.HandlerFunc: validKey guard, GetAgent→404, Reheat→404, Get+GetFolder, c.Data(200, application/x-tar)
- `server/router.go` - registers `r.GET("/agent/:id/remind", remind(st, bl))`
- `server/remind_test.go` - round-trip / 400 / 404 / D-08 init-reflects-index httptests, reusing setup/register/makeTar/postMemory/getInit

## Decisions Made
- Read `m.RelPath` after `Reheat` to locate the restored `.tar` rather than reconstructing the path by convention (anti-pattern avoided).
- Resolved the project memId in tests via `st.List(scope=project)` rather than reconstructing the `YYYYMMDD-` date string — robust to the store's collision-suffixing.
- access_time assertion uses `>=` (sub-second test runs), matching RESEARCH guidance.

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Server-side recall path (RECALL-01/02) complete and green. INDEX-01 exercised end-to-end through init (D-08).
- Remaining in Phase 3: client `remind.sh` + SKILL.md docs and the mid-session reload story (RECALL-03, Plan 03).
- Full suite green: `CGO_ENABLED=0 go build ./...`, `go vet ./...`, `go test ./... -count=1` all pass.

---
*Phase: 03-cold-tier-compress-reheat*
*Completed: 2026-07-26*

## Self-Check: PASSED

All created files exist; both task commits (9d28a10, b1abf3b) present; route registered.
