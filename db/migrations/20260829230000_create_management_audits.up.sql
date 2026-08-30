CREATE TABLE IF NOT EXISTS management_audits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    user_id INTEGER NOT NULL DEFAULT 0,
    user_email VARCHAR(255) NOT NULL DEFAULT '',
    module VARCHAR(64) NOT NULL DEFAULT '',
    action VARCHAR(64) NOT NULL DEFAULT '',
    resource VARCHAR(255) NOT NULL DEFAULT '',
    method VARCHAR(16) NOT NULL DEFAULT '',
    client_ip VARCHAR(64) NOT NULL DEFAULT '',
    success BOOLEAN NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    elapsed_ms BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_management_audits_created_at ON management_audits(created_at);
CREATE INDEX IF NOT EXISTS idx_management_audits_user_id ON management_audits(user_id);
CREATE INDEX IF NOT EXISTS idx_management_audits_module ON management_audits(module);
CREATE INDEX IF NOT EXISTS idx_management_audits_action ON management_audits(action);
