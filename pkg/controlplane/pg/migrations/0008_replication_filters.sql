-- Advanced replication filtering: glob include/exclude patterns
ALTER TABLE replication_status ADD COLUMN IF NOT EXISTS include_patterns text[] NOT NULL DEFAULT '{}';
ALTER TABLE replication_status ADD COLUMN IF NOT EXISTS exclude_patterns text[] NOT NULL DEFAULT '{}';
