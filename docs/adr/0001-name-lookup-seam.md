# ADR-0001: Single name-to-id lookup seam, name-keyed writes

- **Status:** Accepted
- **Date:** 2026-07-27
- **Scope:** server-side request handling; supersedes local "register-then-stash-UUID-on-the-client" pattern

## Context

The `mywholelife` API exposes two identities per agent:

- **`name`** — a human-meaningful string passed on every write via the `X-Agent-Name` header on `POST /agent/register`. Used at registration only.
- **`id`** — a server-issued UUID (e.g. `2605fcd4-e471-433e-bdcd-d1ca95495b7f`). Used on every subsequent request: `POST /agent/:id/memory`, `GET /agent/:id/init`, `GET /agent/:id/remind`.

The first iteration shipped `POST /agent/register` returning **200 + bare UUID** on success and **409** when `name` violated the `UNIQUE` constraint on `agents(name)`. The 409 contract was server-correct but **client-broken**: `init.sh` invoked register with `curl -sf`, treated any non-2xx as fatal, and silently dropped the existing UUID. A fresh `outbox` then arrived at a server with no agent row, producing `404 unknown agent` on every push.

We also shipped **no reverse directory** (name → UUID). Once a 409 fires, no API path brings the operator back to the existing record short of grepping the SQLite file.

This combination — UUID-only routes + 409-on-collision + no reverse lookup — is the structural cause of the **dead-on-arrival second session**: after the first init writes the UUID to `~/.mywholelife/agent.json`, the file can drift (machine swap, container rebuild, manual cleanup) and the next `init.sh` is stuck between "register new agent" (loses continuity) and "trust the stale UUID from agent.json" (may 404). Neither path is right.

## Decision

We adopt one **single lookup seam** plus one **name-first write rule**:

1. **Single lookup seam.** Every server-side entry point that, today or in the future, takes a `name`, goes through **one** method on `MemoryStore`:
   ```go
   GetAgentByName(ctx, name) (Agent, error)
   ```
   No handler shall reimplement name-to-id resolution by parsing the `agents` table, by holding an in-process cache, or by chaining other methods. The seam is the test surface.

2. **Reverse endpoint.** We expose `GET /agent/lookup?name=<name>` returning the existing UUID (200) or 404. This endpoint is the **only** path clients use to recover an id after a 409; clients shall not infer ids from logs, agent.json, or HTTP error bodies.

3. **Name-first write rule.** `init.sh` treats the name as the durable identity of the operator. The flow is, in order:
   1. Call `POST /agent/register`.
   2. On **200** → write UUID to `agent.json`.
   3. On **409** → call `GET /agent/lookup?name=<name>` → write the returned UUID to `agent.json`.
   4. On anything else → fail loud (`HTTP <code>`), never invent.

4. **Client contract.** `agent.json` is a **cache** of the server-side name→id mapping, not the source of truth. A future client version may drop the file entirely if it can talk to the server.

5. **No second paths to identity.** Any future endpoint that needs an agent (per-agent ACLs, per-agent quotas, audit log filtering) **must** accept `name` and internally resolve to id via `GetAgentByName`. Adding a sibling `GetAgentByNameHash`, `GetAgentByEmail`, `GetAgentByToken`, etc. requires opening this ADR.

## Consequences

**Positive**

- A 409 is no longer fatal: the client always has a deterministic path back to the existing record. The "second session" failure mode is closed.
- `GetAgentByName` is a **single seam**; tests for name uniqueness, conflicting-case, and aliasing live in one place. The deletion test passes: removing the method removes the capability, instead of merely relocating it across handlers.
- The seam maps 1:1 to `store.MemoryStore`, so the future `postgresStore` adapter inherits name resolution at the same single point — no per-handler dialect divergence.
- Future per-agent features (quotas, audit) inherit the rule for free and stay grep-able.

**Negative / cost**

- One extra round-trip on the 409 path. Acceptable: 409 is the rare path; the happy path stays a single register.
- A new public endpoint to document and to keep stable. We commit to the response shape (`text/plain`, body is the bare UUID; status 200/400/404) and treat changes as breaking.
- `init.sh` had to grow a branching path; `agent.json` is now contractually a cache and *can* drift, which is healthy but pushes some lifecycle complexity into the operator (a stale `agent.json` is silently corrected on next `init.sh`, which is the behaviour we want — but it is no longer "I lost my id and I lost my data", just "the next init re-adopts the existing record").

## Alternatives Considered

- **Always-200 with overwrite.** Register returns 200 and always creates a new agent, dropping the old one's memories. Rejected: silently orphaning the existing record violates the operator's mental model. The operator chose this name.
- **Register returns the existing UUID on conflict** (200 + body = existing ID). Rejected: collapses two semantically distinct outcomes. The 409 is also load-bearing for human operators, who want to see the collision reported. Adding a separate lookup endpoint keeps the signal distinct.
- **Client trusts its cached UUID forever.** Rejected: when the agent.json is lost (machine move, container rebuild), the operator has no recovery path. Demoting `agent.json` to a cache makes the cache correctable.
- **Server-side name-only routes** (drop UUIDs, route on `name` everywhere). Rejected: name is mutable-by-policy (rename), UUID is not. Treat name as the **operator-facing** identity and UUID as the **storage-facing** identity. Conflating them costs rename flexibility with no compensating benefit.
- **In-process name→id LRU cache.** Rejected on the deletion test: cache + 2 source-of-truths (DB + cache) concentrates complexity at every CRUD edge instead of collapsing it. The seam is the lookup, not a memo of it. Single-tenant, in-process, single-binary scale makes the cache's win negligible.

## References

- Discussion that triggered this ADR: 2026-07-27 session, after `POST /agent/register` returned 409 for a duplicated `X-Agent-Name` and `init.sh` exited with no diagnostic.
- Implemented in: `server/lookup.go`, `server/router.go`, `store/memorystore.go`, `store/sqlite.go`, `client/mywholelife/scripts/init.sh`.
- Feedback loop that validated the fix: `/tmp/mwl-test/feedback.sh` (5/5 PASS).
