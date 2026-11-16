-- Optional periodic rescan interval of a replication in seconds (NULL = off): the worker re-lists the source bucket
-- and copies objects newer than the last scan, for sources written to directly.
ALTER TABLE replication_status ADD COLUMN rescan_seconds integer;
