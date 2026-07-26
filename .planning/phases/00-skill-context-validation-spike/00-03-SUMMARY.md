---
phase: 00-skill-context-validation-spike
plan: 03
subsystem: testing
tags: [claude-code-skills, spike, calibration, go-no-go, bug-14882, decision-doc]

# Dependency graph
requires:
  - phase: 00-01
    provides: estimated chars/4 cost curve (spike/curve.csv)
  - phase: 00-02
    provides: ground-truth /context anchors on CC 2.1.211 (spike/readings.csv)
provides:
  - "spike/calibrate.sh — reconciles curve.csv to readings.csv anchors, derives per-skill resident factor, computes go/no-go"
  - "spike/curve-calibrated.csv — estimated vs calibrated resident tokens + pct-of-window for naive/proxy/merged across N=1..50"
  - "00-SPIKE-FINDINGS.md — the durable gated decision: GO, safe ceiling N=298 (T1 knob target N~20), adopted shape naive, pre-measured VAL-02 fallbacks"
affects: [phase-1 (T1 forgetting knob + global-recent budget), phase-2 (SKILL-01 payload contract), phase-3, phase-4]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Calibrate a chars/4 estimate to live /context anchors via a per-skill resident factor = (anchor - baseline)/N"
    - "catalog-only vs catalog+body distance test as the empirical #14882 verdict"

key-files:
  created:
    - .planning/phases/00-skill-context-validation-spike/spike/calibrate.sh
    - .planning/phases/00-skill-context-validation-spike/spike/curve-calibrated.csv
    - .planning/phases/00-skill-context-validation-spike/00-SPIKE-FINDINGS.md
  modified: []

key-decisions:
  - "GO on the skill-as-memory premise: calibrated naive N=20 = 6700 tok = 0.67% of the 1M window, ~150x under the D-05 10% line"
  - "Correction factor 335/255 = 1.31 (measured naive per-skill resident 335 tok vs chars/4 catalog estimate 255); cross-checked 338 tok at N=50"
  - "#14882 does NOT reproduce on CC 2.1.211 — measured 335 tok/skill tracks catalog-only (|d|=80) not catalog+body (|d|=670); progressive disclosure holds"
  - "Safe ceiling N=298 @1M window / 59 @200k window; T1 forgetting knob operating target is D-06 N~20 with ~15x (1M) / ~3x (200k) headroom"
  - "Adopted shape = naive (full skill folder) per D-07 ladder — first shape passing the 10% line, so no fallback triggered"
  - "Proxy calibrated with the naive catalog factor (no live anchor of its own); merged left un-calibrated (bypasses the skill mechanism)"

patterns-established:
  - "Two-file awk reconciliation: pass 1 extracts anchors + window from readings.csv, pass 2 fills/emits the calibrated curve and prints the decision arithmetic"

requirements-completed: [VAL-01, VAL-02]

# Metrics
duration: 3min
completed: 2026-07-26
---

# Phase 0 Plan 03: Calibrate + Go/No-Go Decision Summary

**Calibrated the estimated skill-cost curve against the live `/context` anchors into a real 335 tok/skill resident factor, and recorded the phase's gating decision: GO on the skill-as-memory premise (naive N=20 = 0.67% vs the 10% line), #14882 does not reproduce on CC 2.1.211, safe ceiling N=298 (T1 knob target N~20), adopted shape naive — with the proxy and merged `recent-brief.md` fallbacks pre-measured for VAL-02.**

## Performance

- **Duration:** ~3 min
- **Completed:** 2026-07-26
- **Tasks:** 2
- **Files created:** 3

## Accomplishments

- `spike/calibrate.sh` reads `curve.csv` + `readings.csv`, isolates the per-skill resident cost `(14700 − 8000)/20 = 335 tok` (cross-checked `(24900 − 8000)/50 = 338 tok`), derives the correction factor `335/255 = 1.31`, and writes `curve-calibrated.csv` with `shape,N,estimated_tokens,calibrated_tokens,pct_of_window`.
- The #14882 diagnostic in stdout resolves the bug: measured 335 tok/skill is close to the catalog-only estimate (255) and far from the catalog+body estimate (1005) — **catalog-only load, progressive disclosure holds, bug does not reproduce on 2.1.211.**
- `00-SPIKE-FINDINGS.md` records all six required sections (Environment, Cost curve, Go/No-Go, Safe ceiling N, Adopted shape, Fallback design) plus a Phase 1/2 consumption note, phrased so downstream phases read one number and one choice.
- The VAL-02 fallback is data-backed: proxy ~56 tok/skill (1,130 @N=20) and merged `recent-brief.md` ~37 tok/brief (740 @N=20) are quoted from the calibrated curve, not described hand-wavily.

## Decision Surfaced (phase gate)

| Value | Result |
|-------|--------|
| **Go/No-Go** | **GO** — naive N=20 = 6,700 tok = 0.67% of 1M window (10% line = 100k) |
| **#14882 on 2.1.211** | Does not reproduce (catalog-only load, 335 tok/skill) |
| **Safe ceiling N** | 298 @1M window / 59 @200k window |
| **T1 knob operating target** | N ≈ 20 (D-06) — generous headroom under the ceiling |
| **Adopted shape** | naive (full skill folder) per the D-07 ladder |

## Calibrated Curve (from curve-calibrated.csv, % of 1M window)

| Shape | N=20 calibrated | N=20 % | N=50 calibrated | N=50 % |
|-------|-----------------|--------|-----------------|--------|
| naive | 6,700 | 0.6700% | 16,750 | 1.6750% |
| proxy | 1,130 | 0.1130% | 2,825 | 0.2825% |
| merged | 740 | 0.0740% | 1,850 | 0.1850% |

## Task Commits

1. **Task 1: Calibrate the curve and compute the go/no-go** — `b7ea38e` (feat)
2. **Task 2: Write 00-SPIKE-FINDINGS.md** — `5878af9` (docs)

## Files Created/Modified

- `spike/calibrate.sh` — two-file awk reconciliation of estimate vs anchors; derives the per-skill factor, writes the calibrated curve, prints the #14882 verdict + pass/fail + safe ceiling.
- `spike/curve-calibrated.csv` — 18 data rows (3 shapes × N=1/5/10/20/30/50), `shape,N,estimated_tokens,calibrated_tokens,pct_of_window`.
- `00-SPIKE-FINDINGS.md` — the durable, gating decision document Phases 1–4 consume.

## Decisions Made

- **Data contract honored:** `estimated_tokens` sourced solely from `curve.csv` `catalog_tokens`; the same column drives both the factor derivation and every row-fill. `fullload_tokens` used only for the stdout #14882 diagnostic, never written to the calibrated CSV.
- **Proxy calibrated, not anchored:** proxy has no live `/context` reading (Plan 02 skipped it because naive already passed), so it takes the naive catalog factor — defensible because proxy is also a catalog-loaded skill. Merged is left un-calibrated since it bypasses the skill catalog entirely.
- **Ceiling vs operating target:** 298 is stated as the do-not-exceed measured ceiling; N≈20 is the recommended steady-state the T1 knob is tuned to, with the ~1% description-budget-eviction caveat recorded so Phase 1 keeps the set small.

## Deviations from Plan

None — plan executed exactly as written. (Commits used `git add -f` on specific named files because `.planning/` is gitignored; a staging mechanic, not a change to planned work.)

## Issues Encountered

None. The proxy row carrying no live anchor was anticipated by Plan 02 and handled by applying the naive catalog factor, with the limitation noted in both the script and the findings.

## Next Phase Readiness

- Phase 1 can read the safe ceiling N and the D-06 target N≈20 directly from `00-SPIKE-FINDINGS.md` to set the T1 forgetting bound and the pinned `global recent` budget.
- Phase 2 can read the adopted shape (naive skill folder) as the SKILL-01 payload contract, with proxy/merged pre-costed fallbacks on file.
- VAL-01 and VAL-02 are both satisfied and were already marked complete in REQUIREMENTS.md; the phase gate is now legibly closed by the findings doc.

## Self-Check: PASSED

- Files verified present: spike/calibrate.sh, spike/curve-calibrated.csv, 00-SPIKE-FINDINGS.md, 00-03-SUMMARY.md
- Commits verified in log: b7ea38e (Task 1), 5878af9 (Task 2)
- Calibrated curve verified: header + naive/proxy/merged rows at N=20 present; naive N=20 = 6700 tok / 0.67%

---
*Phase: 00-skill-context-validation-spike*
*Completed: 2026-07-26*
