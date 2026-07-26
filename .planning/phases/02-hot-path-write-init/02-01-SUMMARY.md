---
phase: 02-hot-path-write-init
plan: 01
subsystem: api
tags: [gin, cobra, http, register, uuid, cli]

requires:
  - phase: 01-storage-seam-schema
    provides: store.MemoryStore/BlobStore interfaces, RegisterAgent (server-minted UUID), Open/NewBlobStore, ErrDuplicateName, seam confinement test
provides:
  - "mywholelife binary with cobra serve + reserved dream stub"
  - "env-driven server.Config (MWL_ADDR/MWL_DATA_ROOT/MWL_DB_PATH)"
  - "Gin router (NewRouter) constructed over store interfaces"
  - "POST /agent/register returning a bare text/plain UUID (register-first identity)"
affects: [02-02, 02-03, write-handler, init-bundler, client-skill]

tech-stack:
  added: [github.com/gin-gonic/gin v1.12.0, github.com/spf13/cobra v1.10.2]
  patterns:
    - "thin Gin handlers over store interfaces (D-01 seam intact)"
    - "register-first identity: server mints UUID, no endpoint accepts a caller id (OQ-1/T-02-01)"
    - "bare text/plain UUID response so the client parses jq-free (D-10)"

key-files:
  created:
    - cmd/mywholelife/main.go
    - cmd/mywholelife/serve.go
    - server/config.go
    - server/router.go
    - server/register.go
    - server/register_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "register-first (OQ-1 resolution A): dedicated POST /agent/register; RegisterAgent mints its own UUID, no caller id accepted"
  - "register returns the raw UUID as text/plain (A2/D-10) so the client needs no JSON parser"
  - "router this plan wires exactly one route; data routes deferred to 02-02 to keep the package compiling"

patterns-established:
  - "NewRouter(store.MemoryStore, store.BlobStore) *gin.Engine — dependency injection of store interfaces, never the driver"
  - "handler closures: func registerAgent(st store.MemoryStore) gin.HandlerFunc"

requirements-completed: [WRITE-01]

duration: 12min
completed: 2026-07-26
---

# Phase 2 Plan 01: Server Foundation Summary

**cobra `mywholelife` CLI (serve + reserved dream), env-driven Config, a Gin router over the Phase-1 store interfaces, and a register-first `POST /agent/register` returning a bare text/plain UUID.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-07-26T17:30:00Z
- **Completed:** 2026-07-26T17:42:00Z
- **Tasks:** 2
- **Files modified:** 8 (6 created, 2 modified)

## Accomplishments
- Buildable, runnable `mywholelife` binary: `serve` starts the Gin API on `MWL_ADDR`; `dream` is a reserved stub returning "not implemented until phase 4".
- `server.Config` + `FromEnv()` reading `MWL_ADDR` (`:8080`), `MWL_DATA_ROOT` (`./data`), `MWL_DB_PATH` (`<data_root>/mywholelife.db`).
- `POST /agent/register` resolves OQ-1 via register-first: mints a server UUID through `store.RegisterAgent`, returns it as a bare `text/plain` body; 409 on duplicate name, 400 on missing `X-Agent-Name`.
- Seam preserved — nothing outside `store/` imports the sqlite driver; the Phase-1 confinement test stays green.

## Task Commits

1. **Task 1: Add Gin + cobra, build the CLI, config, router and register endpoint** - `6e83a46` (feat)
2. **Task 2: Register-endpoint httptest + seam regression** - `3b0585c` (test)

## Files Created/Modified
- `cmd/mywholelife/main.go` - cobra root wiring serve + dream stub; calls Execute
- `cmd/mywholelife/serve.go` - serve subcommand: FromEnv → NewBlobStore → store.Open → NewRouter → Run
- `server/config.go` - Config struct + FromEnv with env defaults
- `server/router.go` - NewRouter over store interfaces; wires POST /agent/register
- `server/register.go` - registerAgent handler; bare-UUID text/plain, 400/409 paths
- `server/register_test.go` - httptest over a real temp-dir store (200+UUID, 409 dup, 400 missing)
- `go.mod` / `go.sum` - gin v1.12.0, cobra v1.10.2 pinned

## Decisions Made
- **register-first (OQ-1 → A):** a dedicated register endpoint rather than register-on-write; `store.RegisterAgent` mints its own UUID and no endpoint accepts a caller-supplied id (mitigates T-02-01 spoofing).
- **bare text/plain UUID (A2):** keeps the client dependency-free (no jq), per D-10.
- **single route this plan:** `router.go` wires only `/agent/register`; the `/agent/:id/memory` and `/agent/:id/init` handlers are 02-02's scope, left unwired so the package compiles cleanly (no references to undefined handlers).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None. `go mod tidy` pulled gin/cobra transitive deps from goproxy.cn cleanly; build stayed CGO-free (modernc pure-Go).

## Verification
- `CGO_ENABLED=0 go build ./...` — passes.
- `go vet ./cmd/... ./server/...` — clean.
- `go run ./cmd/mywholelife --help` — lists both `serve` and `dream`.
- `go test ./... -count=1` — green (`server` register cases + Phase-1 `store`/seam tests).
- Smoke: `serve` + `curl -X POST -H 'X-Agent-Name: alice' /agent/register` → bare UUID; duplicate → 409; missing name → 400.

## Threat Model Compliance
- **T-02-01 (Spoofing, caller id):** mitigated — register-first, server-minted UUID only.
- **T-02-03 (Tampering, duplicate-name takeover):** mitigated — `RegisterAgent` UNIQUE constraint surfaces as 409; no overwrite.
- **T-02-SC (module installs):** gin v1.12.0 + cobra v1.10.2 pinned in go.mod/go.sum, verified live on goproxy.cn per 02-RESEARCH.

## Next Phase Readiness
- Router + Config + seam-clean server package ready for 02-02 to add `POST /agent/:id/memory` and `GET /agent/:id/init` (tar-in / zip-out) over the same `NewRouter` signature.
- `store.NewBlobStore`/`store.Open` wiring in `serve.go` already passes both store handles into `NewRouter`, so the data handlers need no new construction plumbing.

## Self-Check: PASSED

All created files present on disk; both task commits (`6e83a46`, `3b0585c`) exist in history.

---
*Phase: 02-hot-path-write-init*
*Completed: 2026-07-26*
