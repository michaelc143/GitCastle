CREATE TABLE IF NOT EXISTS repository_permissions (
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    username TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('read', 'write', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (repository_id, username)
);
