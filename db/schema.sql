-- db/schema.sql
-- PostgreSQL schema for Mockify SaaS
-- --------------------------------------------------------------
-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- --------------------------------------------------------------
-- Enumerated types
CREATE TYPE user_role AS ENUM ('admin', 'developer', 'viewer');
CREATE TYPE contract_status AS ENUM ('draft', 'published', 'archived');

-- --------------------------------------------------------------
-- Users table
CREATE TABLE users (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email            TEXT NOT NULL UNIQUE,
    password_hash    TEXT NOT NULL,
    full_name        TEXT,
    role             user_role NOT NULL DEFAULT 'developer',
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at   TIMESTAMPTZ
);

-- Trigger to update updated_at on row modification
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_timestamp_users
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

-- --------------------------------------------------------------
-- API Contracts table
CREATE TABLE contracts (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    description      TEXT,
    status           contract_status NOT NULL DEFAULT 'draft',
    is_public        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,                     -- soft delete
    UNIQUE (owner_id, name)                         -- each user/tenant has unique contract names
);

CREATE TRIGGER set_timestamp_contracts
BEFORE UPDATE ON contracts
FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

-- Indexes for fast lookup
CREATE INDEX idx_contracts_owner_id   ON contracts(owner_id);
CREATE INDEX idx_contracts_status     ON contracts(status);
CREATE INDEX idx_contracts_public    ON contracts(is_public);

-- --------------------------------------------------------------
-- Contract Versions table (immutable history)
CREATE TABLE contract_versions (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id      UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    version_number  INTEGER NOT NULL,
    spec             JSONB NOT NULL,                     -- OpenAPI spec
    changelog        TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (contract_id, version_number)
);

-- Auto‑increment version_number per contract using a trigger
CREATE OR REPLACE FUNCTION set_contract_version_number()
RETURNS TRIGGER AS $$
DECLARE
    max_version INTEGER;
BEGIN
    SELECT COALESCE(MAX(version_number), 0) INTO max_version
    FROM contract_versions
    WHERE contract_id = NEW.contract_id;

    NEW.version_number = max_version + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_set_version_number
BEFORE INSERT ON contract_versions
FOR EACH ROW EXECUTE FUNCTION set_contract_version_number();

-- Index for latest version lookup
CREATE INDEX idx_contract_versions_contract_id ON contract_versions(contract_id);
CREATE INDEX idx_contract_versions_created_at ON contract_versions(created_at DESC);

-- --------------------------------------------------------------
-- Audit Log table
CREATE TYPE audit_action AS ENUM ('create', 'read', 'update', 'delete', 'login', 'logout', 'generate_mock', 'run_test');

CREATE TABLE audit_logs (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID REFERENCES users(id) ON DELETE SET NULL,
    action           audit_action NOT NULL,
    target_table     TEXT NOT NULL,
    target_id        UUID,
    details          JSONB,                               -- arbitrary context (e.g., request payload diff)
    ip_address       INET,
    user_agent       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_user_id      ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action       ON audit_logs(action);
CREATE INDEX idx_audit_logs_target_table ON audit_logs(target_table);
CREATE INDEX idx_audit_logs_created_at   ON audit_logs(created_at DESC);

-- --------------------------------------------------------------
-- Rate limiting table for public mock URLs (optional)
CREATE TABLE mock_rate_limits (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id      UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    api_key          TEXT NOT NULL,
    requests_made   INTEGER NOT NULL DEFAULT 0,
    window_start     TIMESTAMPTZ NOT NULL DEFAULT now(),
    window_seconds   INTEGER NOT NULL DEFAULT 60,
    UNIQUE (contract_id, api_key)
);

CREATE INDEX idx_mock_rate_limits_contract_id ON mock_rate_limits(contract_id);
CREATE INDEX idx_mock_rate_limits_window_start ON mock_rate_limits(window_start);

-- --------------------------------------------------------------
-- Views for convenience (optional)

-- Latest version per contract
CREATE OR REPLACE VIEW contract_latest_version AS
SELECT
    cv.contract_id,
    cv.id AS version_id,
    cv.version_number,
    cv.spec,
    cv.changelog,
    cv.created_at
FROM contract_versions cv
JOIN (
    SELECT contract_id, MAX(version_number) AS max_version
    FROM contract_versions
    GROUP BY contract_id
) latest ON latest.contract_id = cv.contract_id
       AND latest.max_version = cv.version_number;

-- --------------------------------------------------------------
-- Permissions (example for a role‑based setup)
-- Note: In production you would manage these via a dedicated RBAC system.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO PUBLIC;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO PUBLIC;

-- End of schema.sql