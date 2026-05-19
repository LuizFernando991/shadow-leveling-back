CREATE TABLE user_xp (
    user_id            UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_xp           INTEGER NOT NULL DEFAULT 0 CHECK (total_xp >= 0),
    current_streak     INTEGER NOT NULL DEFAULT 0 CHECK (current_streak >= 0),
    last_activity_date DATE,
    updated_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE xp_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount      INTEGER NOT NULL,
    reason      VARCHAR(50) NOT NULL,
    source_type VARCHAR(30),
    source_id   UUID,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_xp_events_user_id ON xp_events(user_id);

-- Idempotência: uma sessão de treino concede XP de conclusão uma única vez.
CREATE UNIQUE INDEX uq_xp_events_session_complete
    ON xp_events(source_type, source_id)
    WHERE reason = 'workout_session_completed';