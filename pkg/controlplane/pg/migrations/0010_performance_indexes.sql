-- Performance indexes for delete events lookup and audit logs
CREATE INDEX IF NOT EXISTS idx_delete_events_pending_lookup ON delete_events (replication_id, bucket, object_key, id DESC) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_delete_events_repl_status_id ON delete_events (replication_id, status, id DESC);
CREATE INDEX IF NOT EXISTS idx_lifecycle_audit_rule_id_desc ON lifecycle_audit_log (rule_id, id DESC);
DROP INDEX IF EXISTS idx_lifecycle_audit_rule;
CREATE INDEX IF NOT EXISTS idx_delete_audit_timestamp ON delete_audit_log (timestamp DESC);
