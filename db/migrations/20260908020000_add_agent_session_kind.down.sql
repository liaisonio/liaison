-- Archive management sessions before rolling back application code. Removing
-- this column must not be used to reinterpret management sessions as access.
ALTER TABLE agent_sessions DROP COLUMN kind;
