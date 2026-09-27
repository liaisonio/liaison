-- Apply once using the migration ledger, before starting the new manager.
CREATE TABLE IF NOT EXISTS agent_applications (
  id TEXT PRIMARY KEY,
  owner_id INTEGER NOT NULL,
  edge_id INTEGER NOT NULL,
  kind TEXT NOT NULL,
  installation_id TEXT NOT NULL,
  name TEXT NOT NULL,
  created_at DATETIME,
  updated_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_installation ON agent_applications(owner_id, edge_id, kind, installation_id);
ALTER TABLE agent_accesses ADD COLUMN `application_id` TEXT DEFAULT "";
CREATE INDEX IF NOT EXISTS idx_agent_accesses_application_id ON agent_accesses(application_id);
INSERT INTO agent_applications (id, owner_id, edge_id, kind, installation_id, name, created_at, updated_at)
SELECT lower(hex(randomblob(16))), owner_id, edge_id, kind, installation_id, 'Codex', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM agent_accesses
GROUP BY owner_id, edge_id, kind, installation_id
ON CONFLICT(owner_id, edge_id, kind, installation_id) DO NOTHING;
UPDATE agent_accesses SET application_id = (
  SELECT id FROM agent_applications a WHERE a.owner_id = agent_accesses.owner_id
    AND a.edge_id = agent_accesses.edge_id AND a.kind = agent_accesses.kind
    AND a.installation_id = agent_accesses.installation_id
) WHERE application_id IS NULL OR application_id = '';
