-- Tutti control-plane schema, version 1.
-- Conventions: every config row has version (optimistic concurrency), created_at, updated_at. Documents are the same
-- JSON the Redis stores keep; secrets inside them are sealed with the master key (opaque to PostgreSQL).

CREATE TABLE audit_log (
    id          bigserial PRIMARY KEY,
    at          timestamptz NOT NULL DEFAULT now(),
    actor       text        NOT NULL,
    request_id  text        NOT NULL DEFAULT '',
    action      text        NOT NULL,
    object_type text        NOT NULL,
    object_id   text        NOT NULL,
    before      jsonb,
    after       jsonb
);
CREATE INDEX audit_log_object_idx ON audit_log (object_type, object_id, id DESC);
CREATE INDEX audit_log_at_idx ON audit_log (at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log (actor, id DESC);

-- Transactional outbox: ops is a JSON array of idempotent Redis operations that project the committed change into
-- the Redis cache (see publisher.go). Entries of one object_key are applied in id order.
CREATE TABLE outbox (
    id              bigserial PRIMARY KEY,
    object_key      text        NOT NULL,
    ops             jsonb       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    published_at    timestamptz,
    attempts        integer     NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text        NOT NULL DEFAULT ''
);
CREATE INDEX outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;
CREATE INDEX outbox_published_idx ON outbox (published_at) WHERE published_at IS NOT NULL;

-- Runtime-managed storages (Redis HASH tutti:storages).
CREATE TABLE storages (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Tutti user records (Redis HASH tutti:s3users).
CREATE TABLE s3_identities (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Sealed dynamic credentials (Redis HASH chorus:dynamic_creds, fields "<type>:<storage>:<user>").
CREATE TABLE dynamic_credentials (
    storage_type text        NOT NULL,
    storage      text        NOT NULL,
    user_name    text        NOT NULL,
    sealed       text        NOT NULL,
    version      bigint      NOT NULL DEFAULT 1,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (storage_type, storage, user_name)
);
CREATE INDEX dynamic_credentials_user_idx ON dynamic_credentials (storage, user_name);
-- Key derivation salt (field "salt" of the Redis hash). One row: name = 'salt'.
CREATE TABLE dynamic_credentials_meta (
    name  text PRIMARY KEY,
    value text NOT NULL
);

-- Proxy access keys (Redis HASH tutti:proxy-accounts). Secrets inside doc are sealed.
CREATE TABLE proxy_accounts (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    user_name  text GENERATED ALWAYS AS (doc->>'user') STORED,
    status     text GENERATED ALWAYS AS (doc->>'status') STORED,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX proxy_accounts_user_idx ON proxy_accounts (user_name);

-- Access policies (Redis HASH tutti:access-policies).
CREATE TABLE access_policies (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Console users (Redis HASH tutti:console-users + tutti:console-users:last-login).
CREATE TABLE console_users (
    k             text PRIMARY KEY,
    doc           jsonb       NOT NULL,
    role          text GENERATED ALWAYS AS (doc->>'role') STORED,
    last_login_at timestamptz,
    version       bigint      NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Routing (Redis p:route_user, p:route_bucket:<user>). bucket = '' is the user-level route.
CREATE TABLE routes (
    user_name  text        NOT NULL,
    bucket     text        NOT NULL DEFAULT '',
    storage    text        NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_name, bucket)
);
-- Routing blocks (Redis p:route_block_user, p:route_block_bucket:<user>).
CREATE TABLE route_blocks (
    user_name  text        NOT NULL,
    bucket     text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_name, bucket)
);

-- Active replication policies (Redis p:repl_user_from/to, p:repl_bucket_from/to, p:repl_bucket_to_dest_index).
-- from_bucket = to_bucket = '' is a user-level replication.
CREATE TABLE replication_policies (
    user_name    text        NOT NULL,
    from_bucket  text        NOT NULL DEFAULT '',
    to_storage   text        NOT NULL,
    to_bucket    text        NOT NULL DEFAULT '',
    from_storage text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_name, from_bucket, to_storage, to_bucket)
);
CREATE INDEX replication_policies_dest_idx ON replication_policies (user_name, to_storage, to_bucket);

-- Config part of the replication status (Redis p:repl_status_*; failover_pending and queue state stay in Redis).
-- Kept after the policy is removed (archived), like Redis.
CREATE TABLE replication_status (
    user_name    text        NOT NULL,
    from_storage text        NOT NULL,
    to_storage   text        NOT NULL,
    from_bucket  text        NOT NULL DEFAULT '',
    to_bucket    text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL,
    archived     boolean     NOT NULL DEFAULT false,
    archived_at  timestamptz,
    agent_url    text        NOT NULL DEFAULT '',
    event_source text        NOT NULL DEFAULT '',
    PRIMARY KEY (user_name, from_storage, to_storage, from_bucket, to_bucket)
);

-- Replication fencing generations (Redis p:repl_user_gen:<user>, p:repl_bucket_gen:<user>:<bucket>); never deleted.
CREATE TABLE replication_generations (
    user_name   text   NOT NULL,
    from_bucket text   NOT NULL DEFAULT '',
    to_storage  text   NOT NULL,
    to_bucket   text   NOT NULL DEFAULT '',
    generation  bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (user_name, from_bucket, to_storage, to_bucket)
);

-- Failover policies (Redis HASH tutti:failover-policies).
CREATE TABLE failover_policies (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Failover events (Redis STRING tutti:failover:user:<u>, tutti:failover:bucket:<u>:<b>).
CREATE TABLE failover_events (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Protection groups (Redis HASH tutti:protection:groups). The document also carries promotion state; see
-- docs/tutti/control-plane-db.md for why groups are copied but not yet projected.
CREATE TABLE protection_groups (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    user_name  text GENERATED ALWAYS AS (doc->>'user') STORED,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Last storage connection test (Redis HASH tutti:storage-tests).
CREATE TABLE storage_tests (
    k          text PRIMARY KEY,
    doc        jsonb       NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
