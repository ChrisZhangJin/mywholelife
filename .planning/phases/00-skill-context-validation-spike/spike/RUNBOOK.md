# /context Anchor Measurement Runbook (Phase 0, Plan 02)

Capture the **ground-truth `/context` resident-token readings** that calibrate Plan 01's
estimated `chars/4` curve and settle bug anthropics/claude-code#14882 for this environment.

Only a human can do this: `/context` is an interactive Claude Code slash command, and per
**D-02 the reading must be taken _before the first user turn_** (the always-on cost the
skill catalog adds at session start). An agent cannot observe that number. All setup
(installing the exact skill config) is automated by `gen-skills.sh`; you only run the
sessions and record the numbers.

**Pin (D-08):** every reading must be on Claude Code **2.1.211**. Confirm with
`claude --version` before each session.

---

## What number to record

When you run `/context` at session start, Claude Code prints a breakdown of the context
window, e.g. System prompt, System tools, MCP tools, Memory files (CLAUDE.md), Custom
agents, **Skills**, Messages, and Free space, plus a total window size.

- **`resident_tokens`** = the total context **already consumed before you type anything** —
  i.e. everything except the conversation/`Messages` portion (which is ~0 at a fresh start).
  This is the "always-on" figure: system prompt + tools + memory + **skills** + MCP.
  If `/context` prints a single "used / total" summary at the top, the **used** number at a
  fresh session (before any turn) is the resident figure. Skills show up under the
  **Skills** (or skills/tools) segment of the breakdown — that segment is what grows as N
  rises, and its delta vs the baseline reading is the per-skill resident cost.
- **`total_window_tokens`** = the total context-window size reported (the denominator for
  the D-05 10% go/no-go line; also confirms which window this session runs at).

Record **both** every time. The baseline row (0 dummy skills) isolates the per-skill delta:
`(naive20.resident − baseline.resident) / 20` ≈ real per-skill resident cost.

**#14882 signal:** if `naive,20` resident ≈ baseline + ~20×(catalog ≈ 255 est) the bodies did
**not** load (progressive disclosure holds → likely GO). If it is closer to baseline +
~20×(fullload ≈ 1005 est), the ~3 KB bodies loaded at startup (premise fails → the bug
reproduces at N=20). Note which it looks like in the `notes` cell.

---

## Per-config procedure

Run from the phase directory:
`.planning/phases/00-skill-context-validation-spike/`

For **each** config below:

1. **Clean** (remove any leftover dummy skills):
   ```bash
   bash spike/gen-skills.sh --clean
   ```
2. **Install** the config's skills (skip for `baseline` — it has zero dummy skills):
   ```bash
   bash spike/gen-skills.sh <shape> <N>
   ```
3. **Confirm version:**
   ```bash
   claude --version   # must print 2.1.211
   ```
4. **Start a FRESH Claude Code session** (new session, not this one).
5. **As the very first action, before any other prompt, run:**
   ```
   /context
   ```
6. **Record** `resident_tokens` and `total_window_tokens` from the readout into
   `spike/readings.csv` (fill the matching pre-seeded row; set `cc_version` = `2.1.211`).
7. **Clean before the next config:**
   ```bash
   bash spike/gen-skills.sh --clean
   ```

Each reading needs its **own fresh session** — `/context` must be the first thing run so
the skill catalog cost is measured before any turn inflates the `Messages` segment.

---

## Configs to read (fill readings.csv)

| # | Config      | Install command                       | Row in readings.csv |
|---|-------------|---------------------------------------|---------------------|
| 1 | baseline    | *(none — run `--clean`, install nothing)* | `baseline,0`    |
| 2 | naive N=20  | `bash spike/gen-skills.sh naive 20`   | `naive,20`          |
| 3 | naive N=50  | `bash spike/gen-skills.sh naive 50`   | `naive,50`          |
| 4 | proxy N=20  | `bash spike/gen-skills.sh proxy 20`   | `proxy,20`          |
| 5 | proxy N=50 (optional) | `bash spike/gen-skills.sh proxy 50` | `proxy,50` |

**Why proxy needs only N=20 (D-01(b)):** the second calibration anchor (N=50) exists to
confirm the curve stays linear where the per-unit **body** cost is large. The proxy body is
near-empty (fullload ≈ catalog), so its curve is near-flat — a second point adds no
diagnostic value. Naive keeps both anchors because its ~3 KB body is the whole go/no-go
question. Record proxy N=50 only if convenient.

---

## Copy-paste command blocks

Baseline (config 1):
```bash
cd .planning/phases/00-skill-context-validation-spike
bash spike/gen-skills.sh --clean
claude --version   # expect 2.1.211
# → fresh Claude Code session → run /context first → record baseline,0
```

Naive N=20 (config 2):
```bash
cd .planning/phases/00-skill-context-validation-spike
bash spike/gen-skills.sh --clean
bash spike/gen-skills.sh naive 20
claude --version   # expect 2.1.211
# → fresh session → /context first → record naive,20
```

Naive N=50 (config 3):
```bash
cd .planning/phases/00-skill-context-validation-spike
bash spike/gen-skills.sh --clean
bash spike/gen-skills.sh naive 50
claude --version   # expect 2.1.211
# → fresh session → /context first → record naive,50
```

Proxy N=20 (config 4):
```bash
cd .planning/phases/00-skill-context-validation-spike
bash spike/gen-skills.sh --clean
bash spike/gen-skills.sh proxy 20
claude --version   # expect 2.1.211
# → fresh session → /context first → record proxy,20
```

Proxy N=50 (config 5, optional):
```bash
cd .planning/phases/00-skill-context-validation-spike
bash spike/gen-skills.sh --clean
bash spike/gen-skills.sh proxy 50
claude --version   # expect 2.1.211
# → fresh session → /context first → record proxy,50
```

---

## Finish — leave the environment clean

After the last reading, remove all dummy skills so none linger:
```bash
cd .planning/phases/00-skill-context-validation-spike
bash spike/gen-skills.sh --clean
ls -d ~/.claude/skills/spike-dummy-* 2>/dev/null | wc -l   # must print 0
```

Then reply **"approved"** to the checkpoint (or describe any `/context` ambiguity you hit).
The continuation agent reads the filled `readings.csv`, calibrates the curve, and writes the
plan SUMMARY.

**Safety note:** every install/delete is confined to the `spike-dummy-` namespace and gated
by a `.spike-generated` marker; `--clean` cannot touch your 136 real skills.
