-- Delete propagation policy for replications and tracking tables.
ALTER TABLE replication_status ADD COLUMN IF NOT EXISTS delete_propagation text NOT NULL DEFAULT 'ignore';
ALTER TABLE replication_status ADD COLUMN IF NOT EXISTS delete_include_prefixes text[] NOT NULL DEFAULT '{}';
ALTER TABLE replication_status ADD COLUMN IF NOT EXISTS delete_exclude_prefixes text[] NOT NULL DEFAULT '{}';
ALTER TABLE replication_status ADD COLUMN IF NOT EXISTS delete_delay_seconds bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS delete_events (
    id BIGSERIAL PRIMARY KEY,
    replication_id VARCHAR(255) NOT NULL,
    bucket VARCHAR(255) NOT NULL,
    object_key TEXT NOT NULL,
    version_id VARCHAR(255),
    deletion_type VARCHAR(20) NOT NULL DEFAULT 'hard',
    source_timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_delete_events_status ON delete_events (status);
CREATE INDEX IF NOT EXISTS idx_delete_events_replication ON delete_events (replication_id);

CREATE TABLE IF NOT EXISTS delete_audit_log (
    id BIGSERIAL PRIMARY KEY,
    replication_id VARCHAR(255) NOT NULL,
    bucket VARCHAR(255) NOT NULL,
    object_key TEXT NOT NULL,
    action VARCHAR(20) NOT NULL,
    destination_storage VARCHAR(255) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB
);
CREATE INDEX IF NOT EXISTS idx_delete_audit_replication ON delete_audit_log (replication_id);
