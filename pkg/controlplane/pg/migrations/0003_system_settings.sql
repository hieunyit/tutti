-- Dynamic system and worker settings.
CREATE TABLE system_settings (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
