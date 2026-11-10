-- Publisher: pending entries are selected in SQL (next_attempt_at <= now(), no earlier failed entry of the same
-- object), which needs a per-object lookup of the unpublished entries.
CREATE INDEX outbox_pending_key_idx ON outbox (object_key, id) WHERE published_at IS NULL;
