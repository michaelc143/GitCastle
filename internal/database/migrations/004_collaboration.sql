CREATE TABLE IF NOT EXISTS issues (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    number BIGINT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (repository_id, number)
);

CREATE TABLE IF NOT EXISTS pull_requests (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    number BIGINT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'merged', 'closed')),
    source_branch TEXT NOT NULL,
    target_branch TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (repository_id, number)
);

-- Comments attach to either an issue or a pull request by number.
CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('issue', 'pull_request')),
    subject_number BIGINT NOT NULL,
    author TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS comments_subject_idx
    ON comments(repository_id, subject_type, subject_number);

-- Reviews are attached to pull requests by row id.
CREATE TABLE IF NOT EXISTS reviews (
    id BIGSERIAL PRIMARY KEY,
    pull_request_id BIGINT NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    reviewer TEXT NOT NULL,
    verdict TEXT NOT NULL CHECK (verdict IN ('approved', 'changes_requested', 'commented')),
    body TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (pull_request_id, reviewer) -- latest review wins; upsert below
);

CREATE TABLE IF NOT EXISTS branch_protection (
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    branch TEXT NOT NULL,
    required_approvals INT NOT NULL DEFAULT 1 CHECK (required_approvals >= 0),
    allow_force_push BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (repository_id, branch)
);
