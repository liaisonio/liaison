-- Run only after returning to an older manager and backing up application metadata.
-- Access IDs, legacy installation fields and session history are preserved.
DROP INDEX IF EXISTS idx_agent_accesses_application_id;
ALTER TABLE agent_accesses DROP COLUMN application_id;
DROP TABLE IF EXISTS agent_applications;
