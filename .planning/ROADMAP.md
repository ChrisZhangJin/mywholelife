# Roadmap: mywholelife

## Overview

mywholelife delivers a persistent, self-managing long-term memory system for Claude Code agents. The journey starts by de-risking the single assumption that can invalidate the whole architecture — that installed skills cost near-zero resident context (Phase 0 spike). It then lays the storage seam and access-time schema every other component depends on (Phase 1), closes the LLM-free hot loop of curated write + boot-time init (Phase 2), adds the cold tier and the one operation that reverses the time arrow — compress + reheat (Phase 3), and finishes with the highest-risk autonomous aging engine: dream consolidation and graduated forgetting (Phase 4). The core loop (write → init → reheat → dream → forget) is irreducible but built one ring at a time, each ring validated against a stable layer beneath it.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 0: Skill-Context Validation Spike** - Measure real skill context cost; go/no-go on the skill-as-memory premise (gates everything) (completed 2026-07-26)
- [ ] **Phase 1: Storage Seam + Schema** - MemoryStore interface + SQLite/FS adapters; access-time as sole lifecycle field; namespacing + pinned global-recent
- [ ] **Phase 2: Hot Path (Write + Init)** - Curated write + boot bundle; skill-as-memory payload contract; mywholelife client skill + SessionEnd hook
- [ ] **Phase 3: Cold Tier (Compress + Reheat)** - .tar.zst archival with round-trip verify; remind promotes long-term back to recent; single structured index
- [ ] **Phase 4: Dream Consolidation + Forgetting** - Server-side aging engine: consolidate, maintain the index, graduated tombstone→soft-delete, atomic/resumable

## Phase Details

### Phase 0: Skill-Context Validation Spike

**Goal**: Empirically settle whether Claude Code's skill progressive disclosure makes "N recent skills ≈ free context" true on the target version, before any endpoint or sizing logic is built on the assumption.
**Mode:** mvp
**Depends on**: Nothing (first phase — gates everything downstream)
**Requirements**: VAL-01, VAL-02
**Success Criteria** (what must be TRUE):

  1. A measured `/context` cost curve for 20 and 50 installed dummy recent skills on the target Claude Code version is recorded, showing real resident tokens per skill (per bug anthropics/claude-code#14882).
  2. A documented go/no-go decision on the skill-as-memory premise exists, with the measured safe ceiling for the recent-skill set (the value the T1 forgetting knob will be tuned to).
  3. A fallback payload design (a merged single "recent brief" markdown instead of N installed skills) is documented before any dependent phase proceeds, whether or not the premise holds.

**Plans**: 3 plans
Plans:
**Wave 1**

- [x] 00-01-PLAN.md — Build measurement tooling: dummy-skill generator (naive/proxy/merged) + token-estimator producing the estimated cost curve

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 00-02-PLAN.md — Capture ground-truth `/context` anchor readings (baseline, naive N=20/50, proxy N=20) on CC 2.1.211

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 00-03-PLAN.md — Calibrate the curve, decide go/no-go + safe ceiling N + adopted shape, write 00-SPIKE-FINDINGS.md (incl. merged fallback design)

### Phase 1: Storage Seam + Schema

**Goal**: A unit-testable `MemoryStore` is the only door to storage, with a correct access-time state model, agent namespacing, and a pinned global-recent class — the foundation every later phase reads and writes through.
**Mode:** mvp
**Depends on**: Phase 0 (premise validated or fallback chosen)
**Requirements**: STORE-01, STORE-02, STORE-03, STORE-04, STORE-05, STORE-06
**Success Criteria** (what must be TRUE):

  1. An agent can be registered and looked up by name + unique ID; the ID is the entire scope model, and every path/row/index is namespaced by agent.
  2. Every memory row persists agent, memId, access-time, state ∈ {recent, long-term, tombstone}, and the path to its folder or `.tar.zst`; all disk/index/compression access goes through `MemoryStore` and nothing outside the store layer imports an adapter.
  3. `access-time` is the sole timestamp any lifecycle logic reads and it is updated in the same transaction on every access path (write, init, remind).
  4. `global recent` is pinned and exempt from aging/forgetting, and project memory is namespaced `YYYYMMDD-projectName` with a server-guaranteed no-collision key.

**Plans**: TBD

### Phase 2: Hot Path (Write + Init)

**Goal**: The recent-only loop works end to end — an agent curates and pushes memory, a fresh session boots its identity back into native Claude Code locations — proving the skill-as-memory payload contract with no LLM involved.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: SKILL-01, WRITE-01, WRITE-02, INIT-01, INIT-02, CLIENT-01
**Success Criteria** (what must be TRUE):

  1. A project memory is represented as a Claude Code skill folder (`SKILL.md` + assets) where the compact `description` is the always-loaded brief and the body is load-on-demand — the one payload format write, init, and remind all move.
  2. An agent can push agent-curated memory via `POST /agent/{id}/memory` and it lands as `recent`.
  3. `GET /agent/{id}/init` returns a single downloadable bundle of `global recent` + each recent project skill + `long-term-memory.md`, and the agent can unpack it into its memory dir and `.claude/skills/`.
  4. The `mywholelife` skill documents the service URL + init/remind usage and installs a `SessionEnd` hook — dumb transport only, using dependency-free `bash`/`curl`/`tar` — that triggers the curated upload.

**Plans**: TBD

### Phase 3: Cold Tier (Compress + Reheat)

**Goal**: Memory can go cold and come back — the store gains lossless compression and the single operation that reverses the time arrow (remind), with the structured index the reheat path matches against.
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: COMP-01, COMP-02, RECALL-01, RECALL-02, RECALL-03, INDEX-01
**Success Criteria** (what must be TRUE):

  1. Long-term memory compresses the whole skill folder to `.tar.zst` with the brief/metadata left uncompressed as the reminder, and a source folder is deleted only after its archive is round-trip verified.
  2. `GET /agent/{id}/remind?mem={memId}` returns the targeted long-term memory's decompressed skill folder and reinstalls it, and a successful remind promotes the memory to `recent` by setting access-time = now.
  3. The mid-session reload path (how a reinstalled skill becomes active in the running session) is explicitly defined and verified on the target Claude Code version.
  4. A single `long-term-memory.md` holds one structured entry (`name + hook + memId`) per long-term memory, sized to stay within a bounded ~1–5k token budget.

**Plans**: TBD

### Phase 4: Dream Consolidation + Forgetting

**Goal**: The autonomous aging engine closes the loop — aged recents consolidate into long-term, the index stays a valid structured list, and graduated forgetting bounds the set — all engineered to fail loud and reversible rather than silently amnesiac.
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: INDEX-02, DREAM-01, DREAM-02, DREAM-03, DREAM-04
**Success Criteria** (what must be TRUE):

  1. A server-side "dream" job (cron-invoked CLI) ages memories past T1 (14d, unreminded) from recent to long-term and compresses them.
  2. The dream job maintains `long-term-memory.md` by emitting only new/changed per-item hooks — never a full-file rewrite, never reassigning memIds — and a validator asserts every stored memId maps to exactly one index entry before any output is committed.
  3. Forgetting is graduated: past T2 (90d, never reminded) a memory becomes a remind-able brief tombstone; past T3 (180d) it is soft-deleted with a grace window and a per-run destruction rate limit.
  4. The dream job is atomic and resumable per-item; an interrupted run leaves durable state (filesystem, index, DB) consistent and recoverable via a consistency scan.

**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 0 → 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 0. Skill-Context Validation Spike | 3/3 | Complete   | 2026-07-26 |
| 1. Storage Seam + Schema | 0/TBD | Not started | - |
| 2. Hot Path (Write + Init) | 0/TBD | Not started | - |
| 3. Cold Tier (Compress + Reheat) | 0/TBD | Not started | - |
| 4. Dream Consolidation + Forgetting | 0/TBD | Not started | - |
