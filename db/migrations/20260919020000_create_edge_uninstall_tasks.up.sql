CREATE TABLE IF NOT EXISTS edge_uninstall_tasks (
 `id` varchar(32) PRIMARY KEY,
 `edge_id` integer NOT NULL,
 `active_edge` integer,
 `created_by` integer NOT NULL,
 `instance_id` varchar(80) NOT NULL,
 `status` varchar(32) NOT NULL,
 `reason` varchar(80) NOT NULL DEFAULT '',
 `token_hash` varchar(64) NOT NULL,
 `expires_at` datetime NOT NULL,
 `created_at` datetime,
 `updated_at` datetime
);
CREATE INDEX IF NOT EXISTS idx_edge_uninstall_tasks_edge_id ON edge_uninstall_tasks(edge_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_edge_uninstall_tasks_active_edge ON edge_uninstall_tasks(active_edge);
