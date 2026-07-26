---
name: mywholelife
description: Persistent long-term memory for Claude Code. Run scripts/init.sh at session start to register this agent (first run) and reload recent project + global memories into ~/.mywholelife/memory/ and .claude/skills/. During a session, curate what is worth keeping by writing a Claude Code skill folder into ~/.mywholelife/outbox/<project>/; the SessionEnd hook tars each staged folder, POSTs it to the server, then clears the outbox. The service URL comes from MWL_SERVICE_URL or ~/.mywholelife/agent.json. Run scripts/install.sh once to copy this skill and register the SessionEnd hook. Dependency-free (bash + curl + tar/unzip). A remind command is a forthcoming Phase-3 addition.
---

# mywholelife

Long-term memory for Claude Code, stored as skill folders on a single-tenant
`mywholelife` server. One project memory is one Claude Code skill folder: the
`SKILL.md` `description` is the always-loaded brief; the body is the load-on-demand
full memory.

## Service URL and identity

The server URL is resolved from `MWL_SERVICE_URL`, else from
`~/.mywholelife/agent.json`. Your local identity lives at
`~/.mywholelife/agent.json`:

```json
{ "id": "<uuid>", "name": "<agent-name>", "service_url": "<url>" }
```

The first run of `scripts/init.sh` (no local id) registers this agent with the
server (`POST /agent/register`, `X-Agent-Name: <name>`), receives a UUID, and
writes `agent.json`. Later runs read the id back from that file.

## Reload memory at session start

```bash
scripts/init.sh              # unpack into project-local .claude/skills/
scripts/init.sh --global     # unpack into ~/.claude/skills/
```

`init.sh` downloads `GET /agent/<id>/init` and unpacks it 1:1 into native
locations: `memory/*` -> `~/.mywholelife/memory/`, `skills/*` -> the skills dir.

Restart caveat: Claude Code only picks up skills live from a `.claude/skills/`
directory that already existed at session start. If `init.sh` creates a top-level
`.claude/skills/` for the first time this session, restart Claude Code (or run
`init.sh` before starting the session) for the reloaded skills to be visible.

## Curate what to remember (your job)

Curation is your responsibility, not the hook's. While working, write the skill
folder you want to persist into the staging outbox:

```
~/.mywholelife/outbox/<project>/SKILL.md
~/.mywholelife/outbox/<project>/<optional assets>
```

Give the folder a compact `description` (the brief) and a useful body. Only what
you stage in the outbox is ever pushed.

## Push on session end (dumb transport)

`scripts/install.sh` registers a `SessionEnd` hook
(`scripts/session_end.sh`). When the session ends the hook tars each outbox
project folder, POSTs it to `POST /agent/<id>/memory?scope=project&project=<name>`
as `Content-Type: application/x-tar`, and clears the outbox on success. It carries
no "what to keep" logic — that lives here, with you.

To push manually:

```bash
scripts/push.sh <project> [scope]   # scope defaults to project
```

## Forthcoming

`remind` (server-side recall + compression) arrives in Phase 3 and is not part of
this skill yet.
