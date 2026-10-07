CREATE TABLE IF NOT EXISTS webide_applications (
  id TEXT PRIMARY KEY,
  owner_id INTEGER NOT NULL,
  edge_id INTEGER NOT NULL,
  installation_id TEXT NOT NULL,
  name TEXT NOT NULL,
  mode TEXT NOT NULL,
  port INTEGER NOT NULL,
  created_at DATETIME,
  updated_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_webide_installation ON webide_applications(owner_id, edge_id, installation_id);
CREATE INDEX IF NOT EXISTS idx_webide_applications_owner_id ON webide_applications(owner_id);

CREATE TABLE IF NOT EXISTS webide_accesses (
  id TEXT PRIMARY KEY,
  owner_id INTEGER NOT NULL,
  application_id TEXT NOT NULL,
  name TEXT NOT NULL,
  enabled NUMERIC NOT NULL,
  project TEXT NOT NULL,
  created_at DATETIME,
  updated_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_webide_accesses_owner_id ON webide_accesses(owner_id);
CREATE INDEX IF NOT EXISTS idx_webide_accesses_application_id ON webide_accesses(application_id);
