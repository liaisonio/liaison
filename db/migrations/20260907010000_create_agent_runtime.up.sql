CREATE TABLE IF NOT EXISTS agent_sessions (
    id VARCHAR(64) PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    organization_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT 0,
    active_turn_id VARCHAR(64) NOT NULL DEFAULT '',
    active_attachment_id VARCHAR(64) NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_deleted_at ON agent_sessions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_organization_id ON agent_sessions(organization_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_created_by ON agent_sessions(created_by);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_status ON agent_sessions(status);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_active_turn_id ON agent_sessions(active_turn_id);

CREATE TABLE IF NOT EXISTS agent_attachments (
    id VARCHAR(64) PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    agent_session_id VARCHAR(64) NOT NULL REFERENCES agent_sessions(id),
    access_id INTEGER NOT NULL,
    application_id INTEGER NOT NULL,
    protocol VARCHAR(32) NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    generation INTEGER NOT NULL DEFAULT 1,
    state TINYINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_agent_attachments_deleted_at ON agent_attachments(deleted_at);
CREATE INDEX IF NOT EXISTS idx_agent_attachments_session ON agent_attachments(agent_session_id);
CREATE INDEX IF NOT EXISTS idx_agent_attachments_access ON agent_attachments(access_id);
CREATE INDEX IF NOT EXISTS idx_agent_attachments_application ON agent_attachments(application_id);
CREATE INDEX IF NOT EXISTS idx_agent_attachments_protocol_state ON agent_attachments(protocol, state);

CREATE TABLE IF NOT EXISTS agent_turns (
    id VARCHAR(64) PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    agent_session_id VARCHAR(64) NOT NULL REFERENCES agent_sessions(id),
    status TINYINT NOT NULL DEFAULT 0,
    active_step_id VARCHAR(64) NOT NULL DEFAULT '',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message VARCHAR(1024) NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    started_at DATETIME NULL,
    completed_at DATETIME NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_turns_deleted_at ON agent_turns(deleted_at);
CREATE INDEX IF NOT EXISTS idx_agent_turns_session ON agent_turns(agent_session_id);
CREATE INDEX IF NOT EXISTS idx_agent_turns_status ON agent_turns(status);
CREATE INDEX IF NOT EXISTS idx_agent_turns_completed_at ON agent_turns(completed_at);

CREATE TABLE IF NOT EXISTS agent_messages (
    id VARCHAR(64) PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    agent_session_id VARCHAR(64) NOT NULL REFERENCES agent_sessions(id),
    turn_id VARCHAR(64) NOT NULL REFERENCES agent_turns(id),
    sequence INTEGER NOT NULL,
    role VARCHAR(16) NOT NULL DEFAULT '',
    content_json TEXT NOT NULL DEFAULT '{}',
    model_provider VARCHAR(64) NOT NULL DEFAULT '',
    model_name VARCHAR(128) NOT NULL DEFAULT '',
    usage_json TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_agent_messages_deleted_at ON agent_messages(deleted_at);
CREATE INDEX IF NOT EXISTS idx_agent_messages_session ON agent_messages(agent_session_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_message_order ON agent_messages(turn_id, sequence);
CREATE INDEX IF NOT EXISTS idx_agent_messages_turn ON agent_messages(turn_id);

CREATE TABLE IF NOT EXISTS agent_steps (
    id VARCHAR(64) PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    turn_id VARCHAR(64) NOT NULL REFERENCES agent_turns(id),
    sequence INTEGER NOT NULL,
    kind TINYINT NOT NULL,
    status TINYINT NOT NULL DEFAULT 0,
    tool_id VARCHAR(192) NOT NULL DEFAULT '',
    tool_snapshot_id VARCHAR(64) NOT NULL DEFAULT '',
    input_json TEXT NOT NULL DEFAULT '{}',
    output_json TEXT NOT NULL DEFAULT '{}',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message VARCHAR(1024) NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    started_at DATETIME NULL,
    completed_at DATETIME NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_steps_deleted_at ON agent_steps(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_step_sequence ON agent_steps(turn_id, sequence);
CREATE INDEX IF NOT EXISTS idx_agent_steps_status ON agent_steps(status);
CREATE INDEX IF NOT EXISTS idx_agent_steps_snapshot ON agent_steps(tool_snapshot_id);

CREATE TABLE IF NOT EXISTS agent_approvals (
    id VARCHAR(64) PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    agent_session_id VARCHAR(64) NOT NULL REFERENCES agent_sessions(id),
    turn_id VARCHAR(64) NOT NULL REFERENCES agent_turns(id),
    step_id VARCHAR(64) NOT NULL REFERENCES agent_steps(id),
    requested_by INTEGER NOT NULL,
    decided_by INTEGER NULL,
    status TINYINT NOT NULL DEFAULT 0,
    risk TINYINT NOT NULL DEFAULT 0,
    input_sha256 CHAR(64) NOT NULL DEFAULT '',
    attachment_generation INTEGER NOT NULL DEFAULT 0,
    reason VARCHAR(1024) NOT NULL DEFAULT '',
    decision_note VARCHAR(1024) NOT NULL DEFAULT '',
    expires_at DATETIME NOT NULL,
    decided_at DATETIME NULL,
    invocation_json TEXT NOT NULL DEFAULT '{}',
    snapshot_json TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_agent_approvals_deleted_at ON agent_approvals(deleted_at);
CREATE INDEX IF NOT EXISTS idx_agent_approvals_session ON agent_approvals(agent_session_id);
CREATE INDEX IF NOT EXISTS idx_agent_approvals_turn ON agent_approvals(turn_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_approvals_step ON agent_approvals(step_id);
CREATE INDEX IF NOT EXISTS idx_agent_approvals_status_expiry ON agent_approvals(status, expires_at);
CREATE INDEX IF NOT EXISTS idx_agent_approvals_requested_by ON agent_approvals(requested_by);
CREATE INDEX IF NOT EXISTS idx_agent_approvals_decided_by ON agent_approvals(decided_by);

CREATE TABLE IF NOT EXISTS agent_toolset_snapshots (
    id VARCHAR(64) PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    turn_id VARCHAR(64) NOT NULL REFERENCES agent_turns(id),
    model_step_id VARCHAR(64) NOT NULL REFERENCES agent_steps(id),
    catalog_generation INTEGER NOT NULL,
    policy_revision VARCHAR(128) NOT NULL DEFAULT '',
    attachments_json TEXT NOT NULL DEFAULT '{}',
    tools_json TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_agent_toolset_snapshots_deleted_at ON agent_toolset_snapshots(deleted_at);
CREATE INDEX IF NOT EXISTS idx_agent_toolset_snapshots_turn ON agent_toolset_snapshots(turn_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_toolset_snapshots_model_step ON agent_toolset_snapshots(model_step_id);
