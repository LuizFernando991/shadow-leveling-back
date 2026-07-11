CREATE TABLE groups (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    cover_url   TEXT,
    invite_code VARCHAR(12) NOT NULL UNIQUE,
    owner_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_groups_owner_id ON groups(owner_id);
