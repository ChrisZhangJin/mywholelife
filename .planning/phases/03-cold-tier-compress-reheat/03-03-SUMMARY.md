---
phase: 03-cold-tier-compress-reheat
plan: 03
subsystem: client
tags: [bash, curl, tar, remind, skills, mid-session-reload]

# Dependency graph
requires:
  - phase: 03-02
    provides: "GET /agent/:id/remind?mem= endpoint streaming application/x-tar"
  - phase: 02-03
    provides: "init.sh/push.sh bash+curl+tar patterns, agent.json read_field helper"
provides:
  - "client/mywholelife/scripts/remind.sh — dependency-free recall client"
  - "SKILL.md documentation of remind usage + mid-session reload story"
affects: [phase-04-dream]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "remind.sh mirrors init.sh: arg parsing (--global/--url), agent.json read_field, mktemp+trap tmpdir"
    - "memId sanitized to server allowlist charset before use as path + URL query value (mirrors init.sh's name sanitize)"

key-files:
  created:
    - client/mywholelife/scripts/remind.sh
  modified:
    - client/mywholelife/SKILL.md

key-decisions:
  - "Load-bearing D-05 fallback: remind.sh always echoes the reinstalled SKILL.md body to stdout, independent of CC version, before printing the /reload-skills hint to stderr"
  - "remind.sh takes memId as a positional arg (not --name); sanitized via tr -c 'a-zA-Z0-9_-' '-' capped at 64 chars, same discipline as init.sh's NAME sanitize (T-03-08)"
  - "Dependency-free: bash + curl + tar only, no jq — read_field grep/sed helper copied verbatim from init.sh/push.sh"

patterns-established:
  - "remind.sh: fetch .tar → mkdir -p + tar -x into $SKILLS_DIR/$MEM → cat SKILL.md to stdout → reload hint to stderr"

requirements-completed: [RECALL-03]

# Metrics
duration: 5min
completed: 2026-07-26
---

# Phase 3 Plan 03: Client remind.sh + SKILL.md Recall Docs Summary

**Dependency-free `remind.sh` fetches `GET /agent/:id/remind?mem=`, untars into `.claude/skills/<memId>/` (or `--global`), and echoes the reinstalled SKILL.md body to stdout as the version-independent mid-session fallback for CC's new-skill-subdir watch gap (#31559).**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-07-26T18:29:20Z
- **Completed:** 2026-07-26T18:31:44Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments
- `client/mywholelife/scripts/remind.sh`: mirrors `init.sh`'s structure (`set -euo pipefail`, `read_field` grep/sed helper, `mktemp -d` + `trap` cleanup, `--global`/`--url` flags). Accepts `memId` as `$1` (usage error + exit 2 if missing), sanitizes it to the `^[a-zA-Z0-9_-]{1,64}$` allowlist charset before using it as both a path component and a URL query value (T-03-08 mitigation).
- Resolves `ID`/`SERVICE_URL` from `~/.mywholelife/agent.json`, exits 1 with a clear message if `agent.json` is missing (mirrors `push.sh`'s "run init.sh first" guard) or no id resolves.
- Fetches `curl -sf "$SERVICE_URL/agent/$ID/remind?mem=$MEM" -o "$tmp/mem.tar"`, `mkdir -p` + `tar -C ... -xf` into `$SKILLS_DIR/$MEM/` (project-local `.claude/skills` by default, `~/.claude/skills` with `--global`).
- Load-bearing D-05 fallback: prints `----- BEGIN RECALLED SKILL.md -----` / `cat "$SKILLS_DIR/$MEM/SKILL.md"` / `----- END ... -----` to stdout unconditionally (works on every CC version), then a `/reload-skills` (CC 2.1.152+) or restart hint to stderr, plus the next-session note.
- `SKILL.md`: replaced the `## Forthcoming` stub with `## Recall a long-term memory (remind)`, documenting `scripts/remind.sh <memId>` / `--global`, the `long-term-memory.md` index format memIds come from, and the 3-step reload story (stdout echo / `/reload-skills` / next session) referencing #31559 and D-05/RECALL-03. Amended the frontmatter `description` (a single clause) to mention `remind.sh` instead of "forthcoming Phase-3 addition."

## Task Commits

1. **Task 1: Create remind.sh (untar + echo body + reload hint)** - `2d09598` (feat)
2. **Task 2: Document remind in SKILL.md** - `214f83e` (docs)

## Files Created/Modified
- `client/mywholelife/scripts/remind.sh` - fetch → sanitize memId → untar into `$SKILLS_DIR/$MEM/` → stdout echo of recalled SKILL.md → stderr reload hint; bash/curl/tar only
- `client/mywholelife/SKILL.md` - `## Recall a long-term memory (remind)` section replacing `## Forthcoming`; frontmatter `description` amended

## Decisions Made
- memId is a positional arg (not `--name`/flag), matching the plan's `<memId>` CLI shape and the RESEARCH sketch.
- Sanitization applied to the memId argument itself (not just trusted server output) since it flows into both a filesystem path and a URL query — T-03-08 mitigation, mirroring `init.sh`'s `NAME` sanitize verbatim (`tr -c 'a-zA-Z0-9_-' '-'`, `${VAR:0:64}`).
- Kept the stdout echo unconditional and ahead of the stderr hint in the script body, matching the plan's explicit "must not be dropped" instruction for the version-independent guarantee.

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required; client script only, no Go/server changes.

## Next Phase Readiness
- RECALL-03 closed: remind.sh + SKILL.md docs complete the client side of the Phase-3 remind story (server-side RECALL-01/02 already closed in 03-02).
- Phase 3 (Compress/Reheat/remind/index) is now fully implemented end-to-end: store round-trip (03-01), server endpoint (03-02), client recall (03-03).
- Full Go suite still green (no Go touched this plan): `go build ./...`, `go test ./... -count=1` all pass.
- `bash -n client/mywholelife/scripts/remind.sh` passes; no `jq` dependency introduced.

---
*Phase: 03-cold-tier-compress-reheat*
*Completed: 2026-07-26*

## Self-Check: PASSED

All created/modified files exist; both task commits (2d09598, 214f83e) present in git log; all plan verification greps pass; Go build/test suite green.
