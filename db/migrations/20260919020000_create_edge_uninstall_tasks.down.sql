-- Rollback only after all uninstall tasks have reached confirmed terminal states.
DROP TABLE IF EXISTS edge_uninstall_tasks;
