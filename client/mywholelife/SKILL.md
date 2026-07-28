---
name: mywholelife
description: Long-term memory for Claude Code, persisted as project skill folders on a mywholelife server. On first use on a machine, ask the operator which server URL to use, then run scripts/init.sh --url <URL>. Later sessions just run scripts/init.sh. During a session, stage memory into ~/.mywholelife/outbox/<project>/; SessionEnd tars and POSTs it. scripts/remind.sh <memId> recalls a long-term memory back into working context.
---

# mywholelife

Long-term memory for Claude Code, stored as skill folders on a single-tenant
`mywholelife` server. One project memory is one Claude Code skill folder: the
`SKILL.md` `description` is the always-loaded brief; the body is the load-on-demand
full memory.

## Service URL and identity

The server URL is resolved from `--url` on `init.sh`, else from
`MWL_SERVICE_URL` env, else from `~/.mywholelife/agent.json` (cached on first
register), else from a built-in fallback. Your local identity lives at
`~/.mywholelife/agent.json`:

```json
{ "id": "<uuid>", "name": "<agent-name>", "service_url": "<url>" }
```

### First-use URL check (you do this, before calling init.sh)

`init.sh` will not prompt. If you launch this skill on a machine for the first
time, **check `~/.mywholelife/agent.json`** before running `init.sh`:

- **No `agent.json`** — first time on this machine. Ask the human operator
  where the mywholelife server lives. Use the AskUserQuestion tool with a
  short list of common deployment forms (or free-form input). Once they
  answer, pass it via `--url` so the call is explicit:
  ```bash
  scripts/init.sh --url "<server-url>" [--name "<agent-name>"] [--global]
  ```
  `init.sh` registers, persists `agent.json`, and pulls memories from the
  chosen server.
- **`agent.json` exists** — already wired. Just run `scripts/init.sh`
  (no flags), it reads `service_url` from the cached file.

Do not guess the URL. A wrong cached id yields `404 unknown agent` on every
downstream call; the recovery path is `GET /agent/lookup?name=<name>` with the
right server (ADR-0001) but that's wasted round trips — ask first, then run.

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

## Recall a long-term memory (remind)

Memories that have aged out of "recent" are compressed server-side into
long-term storage and listed in `~/.mywholelife/memory/long-term-memory.md`
(one `- <name> | <hook> | <memId>` line per memory, `memId` last). To bring one
back into working context:

```bash
scripts/remind.sh <memId>              # unpack into project-local .claude/skills/<memId>/
scripts/remind.sh --global <memId>     # unpack into ~/.claude/skills/<memId>/
```

`remind.sh` downloads `GET /agent/<id>/remind?mem=<memId>`, which promotes the
memory back to `recent` (access-time = now) on the server, and unpacks the
returned `.tar` into the skill folder above.

Mid-session reload story (D-05, RECALL-03): on the target Claude Code version a
newly-created skill subdirectory is not reliably auto-activated mid-session
(anthropics/claude-code#31559), so `remind.sh` also prints the reinstalled
`SKILL.md` body to stdout — read it directly for immediate use this session.
Run `/reload-skills` (CC 2.1.152+) to make the recalled skill model-invocable
without leaving the session, or restart Claude Code; either way it loads
normally next session from the skills dir, same restart caveat as `init.sh`.
