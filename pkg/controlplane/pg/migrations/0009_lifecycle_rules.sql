-- Lifecycle rules for TTL-based object expiration and audit logging
CREATE TABLE IF NOT EXISTS lifecycle_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    storage_name VARCHAR(255) NOT NULL,
    bucket VARCHAR(255) NOT NULL,
    prefix TEXT NOT NULL DEFAULT '',
    expiration_days INT NOT NULL,
    schedule VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_run_stats JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_lifecycle_rule UNIQUE(storage_name, bucket, prefix)
);
CREATE INDEX IF NOT EXISTS idx_lifecycle_rules_next_run ON lifecycle_rules (next_run_at) WHERE enabled = true;
CREATE INDEX IF NOT EXISTS idx_lifecycle_rules_storage_bucket ON lifecycle_rules (storage_name, bucket);

CREATE TABLE IF NOT EXISTS lifecycle_audit_log (
    id BIGSERIAL PRIMARY KEY,
    rule_id BIGINT NOT NULL REFERENCES lifecycle_rules(id) ON DELETE CASCADE,
    bucket VARCHAR(255) NOT NULL,
    object_key TEXT NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    last_modified TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_lifecycle_audit_rule ON lifecycle_audit_log (rule_id);
CREATE INDEX IF NOT EXISTS idx_lifecycle_audit_deleted ON lifecycle_audit_log (deleted_at);
