-- Optional display name of a replication (label only, not part of its identity).
ALTER TABLE replication_status ADD COLUMN name text NOT NULL DEFAULT '';
