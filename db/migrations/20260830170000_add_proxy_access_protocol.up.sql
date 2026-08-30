ALTER TABLE proxies ADD COLUMN access_protocol varchar(32) NULL DEFAULT '';

UPDATE proxies
SET access_protocol = CASE
    WHEN application_id IN (SELECT id FROM applications WHERE application_type = 'http') THEN 'http'
    WHEN port = 0 AND application_id IN (SELECT id FROM applications WHERE application_type = 'ssh') THEN 'webssh'
    WHEN port = 0 THEN 'web'
    ELSE 'tcp'
END
WHERE access_protocol IS NULL OR access_protocol = '';

CREATE INDEX IF NOT EXISTS idx_proxies_access_protocol ON proxies(access_protocol);
