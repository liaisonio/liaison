-- Prefer application rollback with these additive columns retained.
-- Only run after stopping the new application; this discards session selections.
ALTER TABLE agent_sessions DROP COLUMN model_name;
ALTER TABLE agent_sessions DROP COLUMN model_provider;
