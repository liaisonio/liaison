-- Public model identifiers only. Empty values follow the current system default.
-- Runtime schema bootstrap adds the same columns on embedded SQLite installs.
ALTER TABLE agent_sessions ADD COLUMN model_provider VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE agent_sessions ADD COLUMN model_name VARCHAR(200) NOT NULL DEFAULT '';
