CREATE TABLE IF NOT EXISTS agents (
  id         TEXT PRIMARY KEY,
  name       TEXT UNIQUE NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS memories (
  mem_id      TEXT PRIMARY KEY,
  agent_id    TEXT NOT NULL REFERENCES agents(id),
  scope       TEXT NOT NULL CHECK(scope IN ('global','project')),
  state       TEXT NOT NULL CHECK(state IN ('recent','long_term','tombstone')),
  access_time INTEGER NOT NULL,
  pinned      INTEGER NOT NULL DEFAULT 0,
  brief       TEXT,
  rel_path    TEXT,
  created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memories_aging
  ON memories(agent_id, scope, state, access_time);
