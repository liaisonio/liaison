-- Destructive rollback: export configuration and request metadata before use.
DROP TABLE IF EXISTS ai_requests;
DROP TABLE IF EXISTS ai_keys;
DROP TABLE IF EXISTS ai_accesses;
DROP TABLE IF EXISTS ai_applications;
