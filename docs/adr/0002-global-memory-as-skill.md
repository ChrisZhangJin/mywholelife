# ADR-0002: Promote global memory into a Claude-Code-readable skill

- **Status:** Accepted
- **Date:** 2026-07-28
- **Scope:** client-side init.sh, hub skill description, server behavior unchanged.
- **Supersedes:** none. Updates the implicit "global memory is a passive mirror"
  behavior that shipped in v0.1.x.

## Context

`mywholelife` has two memory scopes on the server:

- **project** (`scope=project`): a skill folder uploaded by the user, keyed by
  `YYYYMMDD-<project>`, recoverable via `/remind` and shipped through `/init`
  under the zip path `skills/<memId>/<basename>/...`. Today, `init.sh` copies
  these into `~/.claude/skills/<memId>/<basename>/SKILL.md`, where Claude
  Code's skill catalog picks them up automatically on every session that
  loads after the unzip.

- **global** (`scope=global`): a single pinned per-agent SKILL.md, stored at
  `agents/<agent-id>/global.tar`, surfaced by the server zip at
  `memory/global/<basename>/SKILL.md`. Today, `init.sh` copies it to
  `~/.mywholelife/memory/global/<basename>/SKILL.md` — an "audit mirror",
  visible to the operator but NOT discoverable by Claude Code as a skill.

We name this scope **global** because its semantics are "this is always true for
this agent". The contract we want is: when an agent returns in a fresh session,
the global memory is part of what they know without an explicit `/remind` call.
That is exactly what landing the memory inside Claude Code's skill catalog
provides.

Failing that, "global memory" is a sentence that promises more than it does:
agents must explicitly `/remind` a global entry to use it (and currently they
can't even do that, since `/remind` is scoped to project memories). The
operator flagged this on 2026-07-28 with "memory downloaded but where does it
land, isn't it supposed to be under .claude/?". That is the symptom we are
fixing.

## Decision

`init.sh` will, in addition to copying the zip's `memory/global/<basename>/*`
into the human-readable mirror at `~/.mywholelife/memory/global/<basename>/*`,
also copy it under `~/.claude/skills/_global/<basename>/*` so that Claude
Code's skill catalog recognizes the global memory as a real skill and
progressive-discloses its `description` into every new session that opens
after the unzip.

### Concrete shape

```
zip → /tmp/x/memory/global/<basename>/SKILL.md
   copy 1:  ~/.mywholelife/memory/global/<basename>/SKILL.md    (preserve)
   copy 2:  ~/.claude/skills/_global/<basename>/SKILL.md        (new — Claude Code reads)
```

The `_global` prefix on the skill folder name is intentional:

- Underscores cannot conflict with `YYYYMMDD-<project>` (`digits`, `-`, letters).
- `_global` signals "this skill is an always-on baseline" to humans listing
  `~/.claude/skills/`.
- The full folder name is the natural `name` field in the SKILL.md frontmatter;
  renames are an open follow-up, not a blocker.

### Operator-visible contract changes

- **Existing init behavior preserved**: the human-readable mirror at
  `~/.mywholelife/memory/` continues to exist. Operators who grep that path
  during debugging see no change.
- **New auto-active path**: after `init.sh` returns, the operator can run
  `ls ~/.claude/skills/_global/` to see the global memory as a Claude-Code
  skill. Restarting Claude Code (or `/reload-skills` on CC ≥ 2.1.152) makes
  it participate in the catalog.
- **Multi-agent on one machine**: each agent id maps to a distinct global
  tar blob on the server. The client-side path uses only the directory name
  `_global`, **not** the agent id, which means if the operator runs two
  agents with two different globals, the second `init.sh` overwrites the
  first. We accept this for v0.2.0 (single-agent-per-machine was already a
  soft constraint via `agent.json`) and note it as an open follow-up — see
  "Alternatives considered" for the per-agent hash scheme and why we
  deferred it.

### What we are NOT doing in this ADR

- **Server-side change**: the server still ships the same zip. ADR-0001
  (name-to-id lookup) and the `ReserveProjectMemID` flow are unaffected.
  The composition flows all run server-side today (Compress/Reheat on
  long-term project memories), and the operator can still ping global
  via `/remind` after re-anchoring the memID once we wire `/remind` to
  global scope.
- **Naming changes to the SKILL.md frontmatter**: the SKILL.md inside the
  global tar keeps its existing `name:` field. The folder name on disk
  (`_global/<basename>`) is independent.

## Consequences

**Positive**

- "global memory" lives up to its name. A new session that opens after
  `init.sh` returns sees the global skill in the catalog and gets its
  always-true context for free — same mechanism as project memories.
- No server-side change: a single-client-side addition (one `cp -R` in
  `init.sh`) unblocks the global guarantee. Existing server instances
  keep working.
- Operators who like the mirror behavior keep it.

**Negative**

- The single-folder layout (`_global`) cannot carry two agents' globals
  simultaneously on a single machine. Operators running multi-agent
  setups see whichever agent ran `init.sh` last. This was already an
  implicit constraint via `agent.json`; we make it explicit.
- The SKILL.md `description` field for the global baseline now costs
  the always-loaded token budget on every session. Operators should keep
  the global SKILL.md `description` small (we already do: 421 chars on
  v0.1.3).
- One more directory to keep in sync on disk; if the global tar is
  updated server-side, the client gets the new copy on the next
  `init.sh` run (push.sh already does this for project). Operators can
  also manually re-pull with `init.sh`.

## Alternatives Considered

- **Per-agent folder hash**: write `~/.claude/skills/_global_<sha1(agent-id)>/`
  to disambiguate. Defer. The single-agent-per-machine invariant is
  already implicit; making the folder a per-agent hash surfaces a
  problem the operator didn't ask us to solve today.
- **Keep global as a mirror only** (`~/.mywholelife/memory/global/`):
  rejected because the operator's "isn't it supposed to be under
  .claude/?" was explicit feedback that the mirror-only behaviour is
  the wrong default.
- **Server-side dual-write**: have the server zip already include
  `skills/_global/...` instead of `memory/global/...`. Rejected because
  it changes the server's contract for every existing client, and the
  client-only change is strictly smaller.
- **Tell Claude Code to load global via a separate command**: e.g. an
  `M.py` that reads `~/.mywholelife/memory/global/...` on demand.
  Possible but out-of-band from the existing `init.sh` flow. The
  mirror-to-skill copy is the same idea without a new mechanism.

## References

- Trigger: operator feedback 2026-07-28 captured in
  `~/.claude/projects/-root-workspace-mywholelife/memory/open-questions-agent-json-and-global-memory.md`.
- Implementation in: `client/mywholelife/scripts/init.sh`. Server
  unchanged.
- Regression test: new test in `client/scripts` (or an integration test
  scrim) that asserts after `init.sh`, both
  `~/.mywholelife/memory/global/<basename>/SKILL.md` and
  `~/.claude/skills/_global/<basename>/SKILL.md` exist with identical
  contents.
- Rollout: hub skill bumps to v0.2.0; install.sh needed only if the
  operator wants to refresh the local copy of `init.sh`.
