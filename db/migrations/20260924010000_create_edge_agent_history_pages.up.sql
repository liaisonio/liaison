CREATE TABLE IF NOT EXISTS edge_agent_history_pages (
 owner_id INTEGER NOT NULL,
 access_id VARCHAR(32) NOT NULL,
 edge_id BIGINT NOT NULL,
 session_id VARCHAR(32) NOT NULL,
 window BIGINT NOT NULL,
 revision BIGINT NOT NULL,
 payload BLOB,
 PRIMARY KEY (owner_id, access_id, edge_id, session_id, window)
);
