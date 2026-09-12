-- Stop Shell Agent traffic and roll back the application first. Shell attachment
-- records are not usable by old binaries; preserve this column for normal rollback.
ALTER TABLE agent_attachments DROP COLUMN access_handle_id;
