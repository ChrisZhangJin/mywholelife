# Phase 0 — Skill-Context Validation Spike: Findings

**Status:** GO
**Decided:** 2026-07-26
**Requirements settled:** VAL-01 (measured curve + go/no-go + safe ceiling), VAL-02 (data-backed fallback design)

This is the durable output the rest of the roadmap is gated on. Phases 1–4 read two
values from here: the **safe ceiling N** (the T1 forgetting knob) and the **adopted
payload shape** (the SKILL-01 payload contract). Everything else is supporting evidence.

---

## Environment

| Fact | Value | Source |
|------|-------|--------|
| Claude Code version | **2.1.211** (D-08) | `claude --version`, 2026-07-26 |
| Total context window observed | **1,000,000 tokens** | `/context`, fresh Opus session |
| Baseline resident (Skills segment) | **8,000 tokens** | 136 pre-existing real skills, 0 dummy, read before first user turn (D-02) |
| Measurement method | chars/4 estimate (Plan 01) calibrated to live `/context` anchors (Plan 02) | D-01 |

The baseline 8,000 tokens across 136 real skills is **~59 tok/skill** — real
project-memory descriptions are cheap. The dummy naive skills below deliberately used
near-1024-char descriptions (the worst-case description cap), so their per-skill cost is
an upper bound, not the typical case.

---

## Cost Curve

Calibrated from `spike/curve-calibrated.csv`. `estimated_tokens` is the chars/4
always-on catalog estimate (Plan 01); `calibrated_tokens` applies the per-skill
resident factor derived from the live anchors; `pct_of_window` is against the
1,000,000-token window (percent — compare directly to the D-05 10% line).

**naive** (full skill: ~1024-char description + ~3 KB body):

| N | estimated (catalog) | calibrated | % of window |
|----|--------------------|-----------|-------------|
| 1  | 255   | 335    | 0.0335% |
| 5  | 1,275 | 1,675  | 0.1675% |
| 10 | 2,550 | 3,350  | 0.3350% |
| 20 | 5,100 | **6,700**  | **0.6700%** |
| 30 | 7,650 | 10,050 | 1.0050% |
| 50 | 12,750| 16,750 | 1.6750% |

**proxy** (near-empty body that `Read @<path>`s the real memory; lean description) —
no live `/context` anchor (naive already cleared the bar, so shape 2 was not measured
per Plan 02); calibrated with the naive catalog factor since proxy is also a
catalog-loaded skill:

| N | estimated (catalog) | calibrated | % of window |
|----|--------------------|-----------|-------------|
| 1  | 43    | 56    | 0.0056% |
| 20 | 860   | 1,130 | 0.1130% |
| 50 | 2,150 | 2,825 | 0.2825% |

**merged** (single `recent-brief.md` — one flat markdown file, bypasses the skill
mechanism entirely, so no catalog re-injection and no calibration factor):

| N | estimated | calibrated | % of window |
|----|-----------|-----------|-------------|
| 1  | 37    | 37    | 0.0037% |
| 20 | 740   | 740   | 0.0740% |
| 50 | 1,850 | 1,850 | 0.1850% |

### Calibration factor (how estimate → calibrated)

Per-skill resident cost isolated from the anchors:

- `(naive N=20 resident 14,700 − baseline 8,000) / 20` = **335 tok/skill**
- cross-check `(naive N=50 resident 24,900 − baseline 8,000) / 50` = **338 tok/skill** (consistent, linear — no super-linear body-loading term)

Against the chars/4 catalog estimate of 255 tok/skill, the correction factor is
**335 / 255 ≈ 1.31**. It is applied to every naive (and, for lack of its own anchor,
proxy) catalog row; merged is left un-calibrated.

### #14882 verdict on 2.1.211: DOES NOT REPRODUCE

The measured **335 tok/skill** sits close to the **catalog-only** estimate (255,
|Δ|=80) and is only ~1/3 of the **catalog+body / full-body** estimate (1,005, |Δ|=670).
The naive shape therefore tracks **catalog-only** load: **only skill descriptions load
into resident context at startup, not their bodies.** Progressive disclosure is working
on CC 2.1.211 — bug [anthropics/claude-code#14882](https://github.com/anthropics/claude-code/issues/14882)
**does not reproduce** for this shape/version, and the "N recent skills ≈ near-free
resident context" premise holds.

---

## Go/No-Go

**Decision: GO.**

The D-05 pass line is: recent-skill set's always-resident cost ≤ **10%** of the context
window at the target set size N≈20 (D-06).

- Calibrated naive cost at N=20 = **6,700 tokens = 0.67%** of the 1,000,000-token window.
- The 10% line at 1M is **100,000 tokens**; at N=20 we are **~150× under** it.
- Even on a conservative **200,000-token** working window, N=20 = **3.35%** — still well inside the 10% line (20,000 tokens).

All three shapes pass at N=20 (naive 0.67%, proxy 0.11%, merged 0.07%), so the premise
is validated on the strongest (most expensive) shape with room to spare.

---

## Safe Ceiling N

**Safe ceiling N = 298 skills** (largest recent-set that stays ≤ 10% of the observed
1,000,000-token window at the adopted-shape cost of 335 tok/skill: 100,000 / 335 = 298).

> **This is the value the T1 = 14d forgetting knob (Phase 1) is tuned against.** The
> D-06 operating target is **N ≈ 20**, which sits ~15× under the 1M-window ceiling and
> ~3× under a conservative 200k-window ceiling — generous headroom. Phase 1 should set
> the T1 recent-set bound (and the `global recent` pinning budget) at the N≈20 target,
> not at the 298 hard ceiling; the ceiling is the do-not-exceed safety limit, N≈20 is
> the intended steady-state size.

| Window basis | 10% line (tokens) | Safe ceiling N @ 335 tok/skill |
|--------------|-------------------|--------------------------------|
| 1,000,000 (observed) | 100,000 | **298** |
| 200,000 (conservative session) | 20,000 | **59** |

**Caveats that qualify the ceiling (do not raise the operating target on their account):**

1. The 335 tok/skill figure is a **worst case** — dummies used near-1024-char
   descriptions. The 136 real skills average ~59 tok/skill, so real recent memories
   will cost less and the effective ceiling is higher.
2. Independent of the 10% line, Claude Code budgets **all** skill descriptions to
   ~1% of the window and silently evicts the oldest past that budget (PITFALLS.md
   Pitfall 2). At N≈20 with lean real descriptions this is not a concern, but it is a
   second reason to keep the recent set small rather than pushing toward 298.

---

## Adopted Payload Shape

**Adopted shape: naive** (a full Claude Code skill folder — compact `description` as the
always-loaded brief, `SKILL.md` body load-on-demand).

Per the D-07 escalation ladder (naive → proxy → merged), naive is the first shape that
passes the 10% line, so no fallback is needed. **This is the shape Phase 2's SKILL-01
payload contract implements**: one project memory = one skill folder, description = brief,
body = full memory, and this single format is what write / init / remind all move.

The proxy skill (shape 2) and merged brief (shape 3) remain the documented VAL-02
fallbacks below but are **not adopted** — naive cleared the bar directly.

---

## Fallback Design (VAL-02)

Documented so a dependent phase can adopt it whether or not the premise had held, with
measured costs from the calibrated curve — data-backed, not hand-wavy.

### Tier 1 fallback — proxy skill

If a future CC version regresses (#14882 starts reproducing, bodies load at startup),
switch each recent skill to a **proxy**: a skill whose body is near-empty and only does
`Read @<path-to-real-memory-file>`, with `disable-model-invocation: true` /
`user-invocable: true` frontmatter to keep the description lean (STACK.md
progressive-disclosure caveat). The real memory content lives outside the skill body, so
even under full-body loading the resident cost stays near catalog-only.

- Measured (calibrated) cost: **~56 tok/skill**, **1,130 tokens at N=20 (0.11%)**.
- Safe ceiling under this shape: ~1,785 skills @1M / ~357 @200k.

### Tier 2 fallback — merged `recent-brief.md`

If the skill mechanism itself becomes untenable, replace the N installed recent skills
with **one concatenated markdown file** — `recent-brief.md`, one brief per recent memory
in a single file loaded directly (not via the skill catalog). This bypasses progressive
disclosure entirely, so its resident cost equals its own token count with no per-skill
re-injection overhead.

- Measured cost: **~37 tokens per brief**, **740 tokens at N=20 (0.07%)**, **1,850 at N=50 (0.19%)**.
- This is the cheapest shape and the ultimate floor: init would bundle a single
  `recent-brief.md` instead of N skill folders, and Phase 2's payload contract would
  emit briefs into that one file rather than installing skills.

Recovery mapping (PITFALLS.md P2): *premise invalid → fall back to merged single
"recent brief" markdown instead of N installed skills; re-plan init.* The merged shape's
cost is already measured, so that re-plan starts from data.

---

## Consumption Note — What Phases 1–4 Read From This Doc

- **Phase 1 (Storage Seam + Schema)** reads the **safe ceiling N (298; operating target
  N≈20)** to set the **T1 = 14d forgetting bound** and the pinned **`global recent`**
  budget — the recent set must never exceed the ceiling, and T1 is tuned to hold it at
  ~20.
- **Phase 2 (Hot Path)** reads the **adopted shape (naive skill folder)** as the
  **SKILL-01** payload contract: description = always-loaded brief, body = load-on-demand,
  one format for write / init / remind. If it ever needs a fallback, the proxy and
  merged designs above are pre-measured.
- **Phases 3–4** inherit the same shape and ceiling; the merged `recent-brief.md` design
  is on file as the pre-costed Plan B.
