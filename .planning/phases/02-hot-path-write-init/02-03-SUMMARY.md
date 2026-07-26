---
phase: 02-hot-path-write-init
plan: 03
subsystem: client
tags: [claude-code-skill, bash, curl, tar, unzip, sessionend-hook, dependency-free]

# Dependency graph
requires:
  - phase: 02-01
    provides: "POST /agent/register returning a bare text/plain UUID; id/name/project allowlist ^[a-zA-Z0-9_-]{1,64}$"
  - phase: 02-02
    provides: "POST /agent/{id}/memory (x-tar in) and GET /agent/{id}/init (zip out) with the memory/*, skills/* layout"
provides:
  - "mywholelife Claude Code skill folder (SKILL.md) documenting URL + init/push/curation usage"
  - "Dependency-free client scripts: init.sh (register+unpack), push.sh (curated x-tar push), session_end.sh (dumb-transport hook), install.sh (copy + idempotent hook registration)"
  - "hook/settings.snippet.json — verbatim SessionEnd block for manual/no-jq merge"
affects: [phase-03-remind, phase-04-dream]

# Tech tracking
tech-stack:
  added: [bash, curl, tar, unzip]
  patterns:
    - "Client is bash + curl + tar/unzip only; jq is an optional fast-path, never required"
    - "Local identity file ~/.mywholelife/agent.json parsed jq-free via grep/sed"
    - "SessionEnd hook is dumb transport; curation is the agent's job"

key-files:
  created:
    - client/mywholelife/SKILL.md
    - client/mywholelife/scripts/init.sh
    - client/mywholelife/scripts/push.sh
    - client/mywholelife/scripts/session_end.sh
    - client/mywholelife/scripts/install.sh
    - client/mywholelife/hook/settings.snippet.json
  modified: []

key-decisions:
  - "install.sh substitutes the correct session_end.sh command path per install mode (W3): ${CLAUDE_PROJECT_DIR}/... for local, absolute $HOME/... for --global"
  - "session_end.sh uses set -uo pipefail (drops -e) so a per-project push failure never hard-errors the ending session"
  - "Register name defaults to hostname, sanitized to the server allowlist (tr -c to '-', truncated to 64) so first-run register never 400s"

patterns-established:
  - "jq-free JSON read: grep -oE + sed extraction of id/name/service_url from agent.json"
  - "Idempotent hook registration: grep command-path guard, then jq-append-if-present else create-if-absent, never clobber existing settings.json"

requirements-completed: [WRITE-02, INIT-02, CLIENT-01]

# Metrics
duration: 12min
completed: 2026-07-26
---

# Phase 2 Plan 3: mywholelife Client Skill Summary

**A dependency-free Claude Code skill (SKILL.md + four bash scripts + a SessionEnd hook snippet) that registers agent identity, reconstitutes the init bundle into native memory/skills locations, and pushes curated project memory on session end — bash + curl + tar/unzip only, no jq required.**

## Performance

- **Duration:** ~12 min
- **Tasks:** 2 completed
- **Files created:** 6

## Accomplishments
- `SKILL.md` documents the service URL, init/curation/push flow, and the Pitfall-5 restart caveat, with a `description` brief of 675 chars (<=1024, D-08/CLIENT-01).
- `init.sh` registers on first run (jq-free bare-UUID parse), writes `~/.mywholelife/agent.json`, and unzips `memory/*` -> `~/.mywholelife/memory/` and `skills/*` -> `.claude/skills/` (`--global` -> `~/.claude/skills/`) (INIT-02, D-06/D-07/D-10).
- `push.sh` tars a staged outbox folder and POSTs it as raw `application/x-tar`, clearing the outbox only after a successful curl (D-09).
- `session_end.sh` is dumb transport: iterate outbox subdirs -> `push.sh` -> clear; no curation/"remember" logic (WRITE-02, D-09).
- `install.sh` copies the skill and idempotently registers the SessionEnd hook with the correct script path per mode (W3), preserving existing settings.

## Task Commits

1. **Task 1: SKILL.md + init.sh + push.sh** - `6c67217` (feat)
2. **Task 2: session_end.sh + install.sh + settings snippet** - `e5b3b28` (feat)

## Files Created/Modified
- `client/mywholelife/SKILL.md` - Skill brief + body (URL, init, curation-into-outbox, push, restart caveat, remind-forthcoming note)
- `client/mywholelife/scripts/init.sh` - Register-if-needed (jq-free) + download/unzip init bundle into native dirs; `--global` support
- `client/mywholelife/scripts/push.sh` - tar outbox folder -> curl --data-binary @- as x-tar -> clear on success
- `client/mywholelife/scripts/session_end.sh` - Dumb-transport SessionEnd hook; delegates each outbox project to push.sh
- `client/mywholelife/scripts/install.sh` - Copy skill + idempotent, jq-optional, mode-aware SessionEnd registration
- `client/mywholelife/hook/settings.snippet.json` - Verbatim SessionEnd block for manual/no-jq merge

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Sanitize register name to the server allowlist**
- **Found during:** Task 1
- **Issue:** Plan 02-01 validates `X-Agent-Name` against `^[a-zA-Z0-9_-]{1,64}$` and 400s otherwise. A default name from `hostname` commonly contains `.` (FQDN), which would make first-run registration fail.
- **Fix:** `init.sh` sanitizes the name via `tr -c 'a-zA-Z0-9_-' '-'` and truncates to 64 chars before registering.
- **Files modified:** client/mywholelife/scripts/init.sh
- **Commit:** 6c67217

**2. [Rule 1 - Robustness] session_end.sh uses `set -uo pipefail` (not `-e`)**
- **Found during:** Task 2
- **Issue:** SessionEnd is non-blocking; a hard `set -e` abort on one project's push failure would skip remaining projects and emit an error notice.
- **Fix:** Dropped `-e` so the loop keeps going and logs failures to stderr, then exits 0.
- **Files modified:** client/mywholelife/scripts/session_end.sh
- **Commit:** e5b3b28

## Plan-Review Warning #3 (W3) — Resolved

`install.sh` computes the registered hook command from the install mode:
- Local: `${CLAUDE_PROJECT_DIR}/.claude/skills/mywholelife/scripts/session_end.sh`
- `--global`: absolute `$HOME/.claude/skills/mywholelife/scripts/session_end.sh`

Verified by installing in both modes: the global registration points at the actually-installed, executable script (not a nonexistent project-local path). Both modes are idempotent (re-run yields a single SessionEnd entry) and the jq-append branch preserves pre-existing hooks.

## Verification

- `bash -n` clean on init.sh, push.sh, session_end.sh, install.sh.
- No hard `jq` dependency: `grep -qw jq` finds none in init/push/session_end; install.sh uses `command -v jq` as an optional fast-path only.
- `SKILL.md` `description` = 675 bytes (<=1024).
- `settings.snippet.json` is valid JSON with a `SessionEnd` hook pointing at `session_end.sh`.
- `session_end.sh` contains no `remember`/`curat` logic (dumb transport).
- install.sh smoke-tested: local + global registration, idempotent re-run (1 entry), jq-append preserves existing PreToolUse hook.
- Go suite untouched and green: `go build ./...` ok; `go test ./...` -> bundle, server, store all `ok` (no Go changed).

## Known Stubs

None. This plan ships client scripts only; no stubbed data paths.

## Threat Flags

None. No new server-side surface introduced (client-only). Threat register items T-02-08/09/10 are addressed: unpack targets are dedicated dirs; install.sh never clobbers user settings (grep-guard + create-if-absent); the hook pushes only staged outbox content to the single-tenant server.

## Self-Check: PASSED

All 6 client files + SUMMARY.md present on disk; task commits `6c67217` and `e5b3b28` present in git history.
