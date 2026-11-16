-- Optional cutoff of a replication: the initial copy skips source objects last modified before it (NULL = copy everything).
ALTER TABLE replication_status ADD COLUMN since timestamptz;
