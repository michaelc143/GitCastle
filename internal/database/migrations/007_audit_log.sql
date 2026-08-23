-- Phase 5: production hardening.

-- Append-only audit trail. No UPDATE/DELETE grants in production; enforced
-- by convention here and by a trigger blocking mutations.
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    target TEXT NOT NULL DEFAULT '',
    details JSONB,
    remote_ip TEXT
);

CREATE INDEX IF NOT EXISTS audit_log_actor_idx ON audit_log(actor, occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_action_idx ON audit_log(action, occurred_at DESC);

-- Enforce append-only at the database level.
CREATE OR REPLACE FUNCTION audit_log_block_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_log_no_update ON audit_log;
CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_block_mutation();
