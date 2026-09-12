-- Additive: legacy attachments continue resolving their id as the live handle.
ALTER TABLE agent_attachments ADD COLUMN access_handle_id VARCHAR(64) NOT NULL DEFAULT '';
