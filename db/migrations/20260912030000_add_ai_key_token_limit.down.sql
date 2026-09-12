-- Export quota configuration before rollback; usage is retained separately.
ALTER TABLE ai_keys DROP COLUMN token_limit;
