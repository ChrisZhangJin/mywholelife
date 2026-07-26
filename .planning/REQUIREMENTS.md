# Requirements: mywholelife

**Defined:** 2026-07-26
**Core Value:** An agent can leave, come back, and correctly recall who it is and what it has done — with the right project context reassembled into its working context at the right time, without drowning it in irrelevant history.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases. The core loop (write → init → reheat/remind → dream consolidation → forget) is irreducible — all rings must close to validate the central claims — but is built incrementally.

### Validation (skill-as-memory premise)

- [x] **VAL-01**: Measure real resident context cost of 20–50 installed skills on the target Claude Code version (`/context`), producing a go/no-go on the skill-as-memory premise (per bug anthropics/claude-code#14882)
- [x] **VAL-02**: If the premise fails, a fallback payload design (merged single "recent brief" markdown instead of N installed skills) is documented before dependent work proceeds

### Storage & Identity

- [x] **STORE-01**: An agent can be registered and looked up by name + unique ID; the ID is the entire scope model for single-tenant MVP
- [x] **STORE-02**: All storage access (disk, index, compression) goes through a single `MemoryStore` interface — nothing outside the store layer touches adapters
- [x] **STORE-03**: The index persists, per memory, its agent, memId, access-time, state ∈ {recent, long-term, tombstone}, and the path to its folder or `.tar.zst`
- [x] **STORE-04**: All lifecycle logic reads `access-time` (last touched) as the sole timestamp; it is updated in the same transaction on every access path (write, init, remind)
- [x] **STORE-05**: `global recent` memory (durable knowledge/methods) is pinned and exempt from aging/forgetting
- [x] **STORE-06**: Project memory is namespaced as `YYYYMMDD-projectName`, guaranteeing no name collisions

### Skill-as-Memory Contract

- [ ] **SKILL-01**: A project memory is represented as a Claude Code skill folder (`SKILL.md` + assets); the compact `description` is the always-loaded brief, the body is load-on-demand full memory. This one payload format is what write, init, and remind all move

### Write / Capture

- [x] **WRITE-01**: An agent can push agent-curated memory via `POST /agent/{id}/memory`; a write lands as `recent`
- [ ] **WRITE-02**: The `mywholelife` skill installs a `SessionEnd` hook that triggers the curated upload; the hook is dumb transport only (curation happens in the agent)

### Init / Boot

- [ ] **INIT-01**: `GET /agent/{id}/init` returns a single downloadable bundle of `global recent` + each recent project skill + the `long-term-memory.md` index
- [ ] **INIT-02**: An agent can unpack the init bundle into its memory directory and `.claude/skills/`, reconstituting working memory into the native locations Claude Code reads

### Recall / Reheat

- [ ] **RECALL-01**: `GET /agent/{id}/remind?mem={memId}` returns the targeted long-term memory's skill folder (decompressed)
- [ ] **RECALL-02**: A successful remind promotes the memory to `recent` by setting its access-time to now (the only operation that reverses the time arrow)
- [ ] **RECALL-03**: remind reinstalls the skill folder and the mid-session reload path (how the reinstalled skill becomes active) is explicitly defined and verified

### Long-Term Index

- [ ] **INDEX-01**: A single `long-term-memory.md` holds one structured entry (`name + hook + memId`) per long-term memory, sized to stay within a bounded token budget (~1–5k)
- [ ] **INDEX-02**: The index remains a structured list (never narrative prose); a validator asserts every stored memId maps to exactly one index entry before any dream output is committed

### Compression

- [ ] **COMP-01**: Long-term memory compresses the whole skill folder to `.tar.zst`, leaving the brief/metadata uncompressed as the reminder
- [ ] **COMP-02**: A source folder is deleted only after its compressed artifact is round-trip verified

### Dream Consolidation & Forgetting

- [ ] **DREAM-01**: A server-side "dream" job ages memories past T1 (14d, unreminded) from recent to long-term and compresses them
- [ ] **DREAM-02**: The dream job maintains the `long-term-memory.md` index by emitting only new/changed per-item hooks (never a full-file rewrite, never reassigning memIds)
- [ ] **DREAM-03**: Forgetting is graduated: past T2 (90d, never reminded) a memory becomes a brief tombstone; past T3 (180d) it is soft-deleted with a grace window and a per-run destruction rate limit
- [ ] **DREAM-04**: The dream job is atomic and resumable per-item; an interrupted run leaves durable state (filesystem, index, DB) consistent and recoverable

### Client Distribution

- [ ] **CLIENT-01**: The `mywholelife` skill documents the service URL and init/remind usage and installs the completion hook, using only dependency-free tooling (`bash`/`curl`/`tar`) so it runs in any Claude Code environment

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Scaling & Tuning

- **NEXT-01**: `remind?q=` semantic-search escape hatch — trigger: index exceeds ~150–300 hooks / 5k tokens and in-context matching misses
- **NEXT-02**: Dream trigger tuning (cron vs lazy-on-init/push, model selection) — trigger: observed consolidation latency or cost problems
- **NEXT-03**: `global recent` long-term policy — decide whether global knowledge ever folds into the index or stays pinned forever
- **NEXT-04**: Storage adapter swaps (SQLite→Postgres, FS→object-store) — trigger: approaching multi-tenant or scale limits

## Out of Scope

Explicitly excluded. Documented to prevent scope creep. (Anti-features from research.)

| Feature | Reason |
|---------|--------|
| Multi-tenant accounts / auth / quotas | Single-tenant first; ID is the scope. Isolation/abuse governance would drown core-loop validation ("step 2") |
| Vector DB / server-side semantic search | LLM matches the single `long-term-memory.md` in-context; only the `remind?q=` endpoint *shape* is reserved |
| Cross-agent memory sharing | Privacy + cross-contamination risk; a platform-era concern |
| Non-Claude-Code agent support (e.g. OpenClaw) | Skill mechanism is Claude-Code-specific; a generic loader is a whole second delivery path, deferred to step 2 |
| Human-browsable timeline / memoir UI | Not needed to validate core assumptions; data model already supports building it later |
| "Soul" / persona layer | No practical value distinct from `global recent` (durable knowledge/methods); dropped outright |
| Auto-capture-everything (firehose) | Produces noise and bloats the store; contradicts the bounded single-index design. Agent-curated writes instead |

## Traceability

Which phases cover which requirements. (Finalized by roadmapper — matches ROADMAP.md phase structure.)

| Requirement | Phase | Status |
|-------------|-------|--------|
| VAL-01 | Phase 0 | Complete |
| VAL-02 | Phase 0 | Complete |
| STORE-01 | Phase 1 | Complete |
| STORE-02 | Phase 1 | Complete |
| STORE-03 | Phase 1 | Complete |
| STORE-04 | Phase 1 | Complete |
| STORE-05 | Phase 1 | Complete |
| STORE-06 | Phase 1 | Complete |
| SKILL-01 | Phase 2 | Pending |
| WRITE-01 | Phase 2 | Complete |
| WRITE-02 | Phase 2 | Pending |
| INIT-01 | Phase 2 | Pending |
| INIT-02 | Phase 2 | Pending |
| CLIENT-01 | Phase 2 | Pending |
| COMP-01 | Phase 3 | Pending |
| COMP-02 | Phase 3 | Pending |
| RECALL-01 | Phase 3 | Pending |
| RECALL-02 | Phase 3 | Pending |
| RECALL-03 | Phase 3 | Pending |
| INDEX-01 | Phase 3 | Pending |
| INDEX-02 | Phase 4 | Pending |
| DREAM-01 | Phase 4 | Pending |
| DREAM-02 | Phase 4 | Pending |
| DREAM-03 | Phase 4 | Pending |
| DREAM-04 | Phase 4 | Pending |

**Coverage:**
- v1 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0 ✓

---
*Requirements defined: 2026-07-26*
*Last updated: 2026-07-26 after roadmap creation (traceability finalized)*
