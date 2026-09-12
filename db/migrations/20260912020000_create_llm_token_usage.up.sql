-- Append-only usage ledger; independent of request-log retention. SQLite.
CREATE TABLE IF NOT EXISTS llm_token_usage (
 id INTEGER PRIMARY KEY AUTOINCREMENT, request_id TEXT NOT NULL,
 proxy_id INTEGER NOT NULL, user_id INTEGER NOT NULL, key_id INTEGER NOT NULL DEFAULT 0,
 model TEXT NOT NULL, input_tokens INTEGER, output_tokens INTEGER,
 complete NUMERIC NOT NULL DEFAULT 0, created_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_usage_request ON llm_token_usage(request_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_owner ON llm_token_usage(proxy_id,user_id,created_at);
CREATE INDEX IF NOT EXISTS idx_llm_usage_key ON llm_token_usage(key_id);
