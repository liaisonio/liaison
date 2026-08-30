DROP INDEX IF EXISTS idx_proxies_access_protocol;

ALTER TABLE proxies DROP COLUMN access_protocol;
