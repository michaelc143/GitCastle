-- Phase 4: automation.

CREATE TABLE IF NOT EXISTS webhooks (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret TEXT NOT NULL DEFAULT '',
    events TEXT[] NOT NULL DEFAULT '{push,pull_request}',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id BIGSERIAL PRIMARY KEY,
    webhook_id BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status INT NOT NULL DEFAULT 0,          -- HTTP status of last attempt
    attempts INT NOT NULL DEFAULT 0,
    delivered_at TIMESTAMPTZ,               -- null while pending
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS build_jobs (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    commit_hash TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT '',
    trigger_kind TEXT NOT NULL,             -- push | pull_request | manual
    trigger_ref TEXT NOT NULL DEFAULT '',   -- PR number for pull_request kind
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','success','failed')),
    output TEXT NOT NULL DEFAULT '',
    exit_code INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS build_jobs_repo_idx ON build_jobs(repository_id, created_at DESC);

-- Deployment secrets are encrypted at rest with the server key.
CREATE TABLE IF NOT EXISTS deploy_secrets (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    ciphertext BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (repository_id, name)
);
