CREATE TABLE IF NOT EXISTS edge_agent_histories (
  owner_id INTEGER NOT NULL,
  access_id TEXT NOT NULL,
  edge_id INTEGER NOT NULL,
  session_id TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 0,
  payload BLOB,
  title_override BLOB,
  closed NUMERIC NOT NULL DEFAULT 0,
  deleted NUMERIC NOT NULL DEFAULT 0,
  created_at DATETIME,
  updated_at DATETIME,
  synced_at DATETIME,
  PRIMARY KEY (owner_id, access_id, edge_id, session_id)
);
CREATE INDEX IF NOT EXISTS idx_edge_agent_history_scope ON edge_agent_histories(owner_id, access_id, edge_id, deleted, updated_at);
CREATE INDEX IF NOT EXISTS idx_edge_agent_history_sync ON edge_agent_histories(deleted, closed, synced_at);
