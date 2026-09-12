-- SQLite additive migration. NULL preserves unlimited behavior for existing keys.
ALTER TABLE ai_keys ADD COLUMN token_limit INTEGER;
