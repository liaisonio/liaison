CREATE TABLE IF NOT EXISTS agent_accesses (
  id TEXT PRIMARY KEY,
  owner_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  edge_id INTEGER NOT NULL,
  installation_id TEXT NOT NULL,
  project TEXT NOT NULL,
  created_at DATETIME,
  updated_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_agent_accesses_owner_id ON agent_accesses(owner_id);
CREATE INDEX IF NOT EXISTS idx_agent_accesses_edge_id ON agent_accesses(edge_id);
