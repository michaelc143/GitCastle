-- Backfill for databases created before merge_commit existed.
ALTER TABLE pull_requests ADD COLUMN IF NOT EXISTS merge_commit TEXT;
