---
phase: 00-skill-context-validation-spike
plan: 01
subsystem: testing
tags: [claude-code-skills, spike, bash, token-estimation, context-window, bug-14882]

# Dependency graph
requires: []
provides:
  - "spike/gen-skills.sh — safe dummy-skill generator for naive/proxy/merged shapes + --clean teardown"
  - "spike/estimate.sh — chars/4 token-estimator splitting always-on catalog cost from full-body-load cost"
  - "spike/curve.csv — estimated per-shape, per-N (1/5/10/20/30/50) cost curve, catalog and fullload columns"
  - "Per-unit token figures: naive catalog=255/fullload=1005, proxy catalog=43/fullload=57, merged=37"
affects: [00-02 (/context calibration of the estimate), 00-03 (go/no-go + shape adoption), phase-1 (T1 recent-set ceiling), phase-2 (init payload shape)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Prefix-gated destructive ops: every install/delete confined to the spike-dummy- namespace, marker-guarded (.spike-generated) so a real skill sharing the name aborts the run"
    - "chars/4 token heuristic as the pre-calibration estimate; /context anchors correct it in Plan 02"
    - "Catalog (name+description) vs full-body-load split to model bug #14882 both ways"

key-files:
  created:
    - .planning/phases/00-skill-context-validation-spike/spike/gen-skills.sh
    - .planning/phases/00-skill-context-validation-spike/spike/estimate.sh
    - .planning/phases/00-skill-context-validation-spike/spike/curve.csv
  modified: []

key-decisions:
  - "fullload = name+description+body (excludes extra proxy frontmatter keys) so the proxy shape reads as body-free vs naive body-dominated"
  - "recent-brief.md written to the spike dir, never to ~/.claude/skills/ — the merged shape bypasses the skill mechanism entirely"
  - "Marker file .spike-generated gates every deletion; a spike-dummy-* dir without it aborts teardown to protect real skills"

patterns-established:
  - "Namespace + marker double-gate on destructive filesystem ops writing into a live shared directory"
  - "Idempotent generators: re-running a shape cleans its prior instances first"

requirements-completed: [VAL-01, VAL-02]

# Metrics
duration: 12min
completed: 2026-07-26
---

# Phase 0 Plan 01: Skill-Context Measurement Tooling Summary

**Throwaway bash tooling that installs N dummy recent skills of three payload shapes into the live ~/.claude/skills/ and emits an estimated catalog-vs-full-body token cost curve (chars/4), with real skills prefix- and marker-gated against any teardown damage.**

## Performance

- **Duration:** ~12 min
- **Completed:** 2026-07-26
- **Tasks:** 2
- **Files created:** 3

## Accomplishments
- `gen-skills.sh` installs/removes naive, proxy, and merged payload shapes at any N, with all writes/deletes confined to the `spike-dummy-` prefix and a `.spike-generated` marker guarding every deletion.
- `estimate.sh` produces the estimated cost curve, separating always-on catalog cost (name+description, re-injected each turn per D-02) from full-body-load cost (name+description+body, the cost if bug #14882 reproduces).
- `curve.csv` holds 18 data rows (3 shapes × N=1/5/10/20/30/50) with `catalog_tokens` and `fullload_tokens` columns.
- The estimate cleanly distinguishes the shapes: naive fullload (1005) is ~4× its catalog (255, body dominates); proxy fullload (57) ≈ catalog (43, near-empty body); merged is a single flat per-brief figure (37).

## Per-Unit Token Estimates (chars/4, pre-calibration — Plan 02 anchors these)

| Shape  | catalog_tokens | fullload_tokens |
|--------|----------------|-----------------|
| naive  | 255            | 1005            |
| proxy  | 43             | 57              |
| merged | 37             | 37              |

These are the figures Plan 02 (`/context` calibration) and Plan 03 (go/no-go, shape adoption) consume.

## Task Commits

Each task was committed atomically:

1. **Task 1: Dummy-skill generator with safe install/teardown** - `b75ce75` (feat)
2. **Task 2: Token-estimator and estimated cost curve** - `30ad9eb` (feat)

## Files Created/Modified
- `spike/gen-skills.sh` - Generator for naive/proxy/merged shapes + `--clean` teardown; prefix+marker gated
- `spike/estimate.sh` - chars/4 token-estimator; splits catalog vs full-body-load cost, writes the curve
- `spike/curve.csv` - Estimated per-shape, per-N cost curve (18 data rows)

## Decisions Made
- **fullload = name+description+body** (not whole-file chars): excludes the proxy's extra frontmatter keys so the proxy reads as body-free (fullload≈catalog) while naive is body-dominated — matching the plan's shape-distinction acceptance criterion.
- **Merged shape writes only `spike/recent-brief.md`**, never a skill dir — it definitionally bypasses progressive disclosure, so catalog and fullload are equal.
- **Marker-file deletion gate** (`.spike-generated`): teardown refuses any `spike-dummy-*` dir lacking the marker, so a hypothetical real skill colliding with the namespace aborts the run rather than being deleted.

## Deviations from Plan

None - plan executed exactly as written. (The commit path used `git add -f` because the project gitignores `.planning/`; this is a staging mechanic, not a change to the planned work.)

## Issues Encountered
- Repository had unborn HEAD (`master`, no commits) and flagged dubious ownership; resolved by adding the safe.directory exception. First task commit became the repo's initial commit. No worktrees (config `use_worktrees: false`), so worktree safety assertions did not apply.
- `.planning/` is gitignored, so the deliverable spike scripts were staged with `git add -f` on the specific named files (never `git add -A`).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 02 can now run `/context` at N=20 and N=50 to anchor the chars/4 estimate against real resident tokens; `curve.csv` + the per-unit table above are the values it calibrates.
- Plan 03's go/no-go has data-backed figures for all three shapes, including the merged fallback (VAL-02), not just descriptions.
- Real skills (code-review, gsd-*, etc.) verified untouched after every teardown; at least one non-`spike-dummy-*` skill dir survives `--clean`.

## Self-Check: PASSED

- Files verified present: spike/gen-skills.sh, spike/estimate.sh, spike/curve.csv, 00-01-SUMMARY.md
- Commits verified in log: b75ce75, 30ad9eb, cf98126
- Safety verified: 136 non-spike-dummy skill dirs intact, 0 spike-dummy dirs remaining after teardown

---
*Phase: 00-skill-context-validation-spike*
*Completed: 2026-07-26*
