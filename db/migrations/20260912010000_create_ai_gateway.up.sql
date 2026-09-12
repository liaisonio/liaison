-- SQLite; additive, matches the self-hosted DAO AutoMigrate schema.
CREATE TABLE IF NOT EXISTS ai_applications (
 `application_id` INTEGER PRIMARY KEY, `protocol` TEXT NOT NULL, `base_path` TEXT NOT NULL,
 `tls` NUMERIC NOT NULL DEFAULT 0, `encrypted_key` TEXT NOT NULL DEFAULT '',
 `target_fingerprint` TEXT NOT NULL, `updated_at` DATETIME
);
CREATE TABLE IF NOT EXISTS ai_accesses (
 `proxy_id` INTEGER PRIMARY KEY, `enabled` NUMERIC NOT NULL DEFAULT 0,
 `models` TEXT NOT NULL DEFAULT '{}', `updated_at` DATETIME
);
CREATE TABLE IF NOT EXISTS ai_keys (
 `id` INTEGER PRIMARY KEY AUTOINCREMENT, `proxy_id` INTEGER NOT NULL, `user_id` INTEGER NOT NULL,
 `name` TEXT NOT NULL, `digest` TEXT NOT NULL, `models` TEXT NOT NULL,
 `expires_at` DATETIME NOT NULL, `revoked_at` DATETIME, `created_at` DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_keys_digest ON ai_keys(digest);
CREATE INDEX IF NOT EXISTS idx_ai_keys_owner ON ai_keys(proxy_id,user_id);
CREATE TABLE IF NOT EXISTS ai_requests (
 `id` INTEGER PRIMARY KEY AUTOINCREMENT, `request_id` TEXT NOT NULL, `proxy_id` INTEGER NOT NULL,
 `user_id` INTEGER NOT NULL, `key_id` INTEGER NOT NULL DEFAULT 0, `model` TEXT NOT NULL, `status` INTEGER NOT NULL,
 `duration_ms` INTEGER NOT NULL, `input_tokens` INTEGER, `output_tokens` INTEGER,
 `complete` NUMERIC NOT NULL DEFAULT 0, `created_at` DATETIME
);
CREATE INDEX IF NOT EXISTS idx_ai_requests_owner ON ai_requests(proxy_id,user_id);
