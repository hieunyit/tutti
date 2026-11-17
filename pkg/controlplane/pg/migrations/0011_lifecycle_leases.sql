ALTER TABLE lifecycle_rules ADD COLUMN lease_token text;
ALTER TABLE lifecycle_rules ADD COLUMN lease_until timestamptz;
