---
phase: 00-skill-context-validation-spike
plan: 02
subsystem: testing
tags: [claude-code, skills, context-window, progressive-disclosure, measurement, bug-14882]

# Dependency graph
requires:
  - phase: 00-01
    provides: dummy-skill generator (naive/proxy/merged) + chars/4 token-estimator curve (curve.csv)
provides:
  - Ground-truth /context resident-token anchors on CC 2.1.211 (spike/readings.csv)
  - Baseline (0 dummy) reading isolating per-skill resident delta
  - Empirical signal that skill progressive disclosure HOLDS (catalog-only load) — bug #14882 does NOT reproduce for this shape/version
affects: [00-03, phase-1, phase-2]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Automate-then-human-verify: executor installs exact skill config + hands human a runbook; human reads interactive /context before first user turn (D-02)"

key-files:
  created: []
  modified:
    - .planning/phases/00-skill-context-validation-spike/spike/readings.csv

key-decisions:
  - "Measured ~335-338 tok/skill for naive skills = catalog-only load (~1/3 of full-body est 1005/skill, close to catalog-only est 255/skill) → progressive disclosure holds, #14882 does NOT reproduce on CC 2.1.211"
  - "Proxy anchor intentionally NOT measured — naive already clears the go/no-go bar, so the shape-2 fallback is empirically unnecessary; VAL-02 documented from estimator data instead"

patterns-established:
  - "Per-skill resident cost isolated as (config reading − baseline reading) / N, using a zero-dummy baseline that still contains the 136 pre-existing real skills"

requirements-completed: [VAL-01]

# Metrics
duration: 6min
completed: 2026-07-26
---

# Phase 0 Plan 02: /context Anchor Readings Summary

**Ground-truth `/context` readings on CC 2.1.211 show naive skills cost ~335-338 resident tokens each (catalog-only, ~1/3 of the full-body estimate) — progressive disclosure holds and bug #14882 does NOT reproduce for this shape/version.**

## Performance

- **Duration:** ~6 min (continuation after human-verify checkpoint)
- **Completed:** 2026-07-26
- **Tasks:** 2 (Task 1 in prior session; Task 2 human-verify checkpoint resolved here)
- **Files modified:** 1 (spike/readings.csv finalized with captured anchors)

## Accomplishments

- Captured the three required ground-truth anchors from live fresh Claude Code 2.1.211 sessions, read as the first action before any user turn (D-02):
  - **baseline** (0 dummy, 136 real skills): **8000** resident tokens / 1,000,000 window
  - **naive N=20**: **14700** → +6.7k vs baseline = **335 tok/skill**
  - **naive N=50**: **24900** → +16.9k vs baseline = **338 tok/skill**
- Isolated a stable per-skill resident delta (~335-338 tok) that is consistent across N=20 and N=50 — a flat, linear per-skill cost with no super-linear body-loading term.
- Confirmed the environment was left clean (0 `spike-dummy-*` skills remain installed).

## Interpretation: catalog-only vs. full-body (the #14882 signal)

Plan 01's `chars/4` estimator (curve.csv) bracketed each naive skill between a **catalog-only** cost (name + description always resident) of **255 tok/skill** and a **full-body-load** cost (catalog + ~3 KB body) of **1005 tok/skill**. The measured **~335-338 tok/skill** sits close to the catalog-only estimate and is only ~1/3 of the full-body estimate.

This is the decisive signal for bug anthropics/claude-code#14882: **only skill descriptions load into resident context at startup, not their bodies.** Progressive disclosure is working on CC 2.1.211 — the skill-as-memory premise (N recent skills ≈ near-free resident context) holds for this shape and version, and #14882 does NOT reproduce here.

## Why proxy was not measured

The proxy shape (shape 2) is the fallback workaround for the case where naive skills load their full body. Because naive **already clears the 10% go/no-go bar** (measured catalog-only cost is far below the full-body worst case), the proxy fallback is empirically unnecessary — measuring it adds no diagnostic value. VAL-02's fallback design is therefore documented from the estimator data (curve.csv) rather than a live reading. The `proxy,20` / `proxy,50` rows in readings.csv are intentionally left with empty token cells and a note recording this decision.

## Feeds Plan 00-03

These anchors are the calibration input for Plan 00-03, which owns the final go/no-go decision, the safe-ceiling N math (the value the T1 forgetting knob is tuned to), and the merged-fallback design writeup in 00-SPIKE-FINDINGS.md. **This plan deliberately does not duplicate that math** — its scope is only to capture and attest the anchor readings.

## Task Commits

1. **Task 1: runbook + seed readings.csv** — `c5f4a40` (feat, prior session)
2. **Task 2: human captures /context anchors** — checkpoint resolved; readings.csv finalized in this plan's metadata commit

**Plan metadata:** see final `docs(00-02)` commit below.

## Files Created/Modified

- `spike/readings.csv` — filled with the three captured anchors (baseline, naive N=20, naive N=50), each tagged cc_version 2.1.211, plus the intentionally-unmeasured proxy rows with rationale.

## Decisions Made

- **Naive load is catalog-only (~335-338 tok/skill), not full-body** — settles #14882 as non-reproducing for this shape/version.
- **Proxy intentionally not measured** — naive passes, so the fallback is empirically unnecessary; VAL-02 documented from estimator data.

## Deviations from Plan

None - plan executed exactly as written. The proxy anchor being left unmeasured is explicitly permitted by the plan (naive is the primary go/no-go anchor; proxy N=50 was already optional, and the plan's context notes the proxy exists only as fallback data).

## Issues Encountered

None. The human-verify checkpoint (D-02: `/context` read before the first user turn) was the expected, planned pause — resolved with an "approved" resume signal once the anchors were filled.

## Next Phase Readiness

- Anchors are ready for Plan 00-03 calibration (go/no-go + safe-ceiling N + adopted shape).
- Environment clean; no dummy skills linger.

## Self-Check: PASSED

- FOUND: .planning/phases/00-skill-context-validation-spike/00-02-SUMMARY.md
- FOUND commit: c5f4a40 (Task 1)
- Anchors verified in readings.csv (cc_version 2.1.211): baseline,0 resident=8000; naive,20 resident=14700; naive,50 resident=24900 (all window=1000000)

---
*Phase: 00-skill-context-validation-spike*
*Completed: 2026-07-26*
