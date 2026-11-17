-- Legacy rows have no reliable generation and cannot safely be replayed.
ALTER TABLE delete_events ADD COLUMN IF NOT EXISTS generation bigint NOT NULL DEFAULT -1;
