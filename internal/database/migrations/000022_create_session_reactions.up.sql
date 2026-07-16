CREATE TABLE session_reactions (
    session_id UUID        NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
    group_id   UUID        NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji      TEXT        NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (session_id, group_id, user_id)
);

CREATE INDEX idx_session_reactions_group_session ON session_reactions(group_id, session_id);
