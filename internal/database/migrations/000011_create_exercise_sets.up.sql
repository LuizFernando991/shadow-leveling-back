CREATE TABLE exercise_sets (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID         NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
    exercise_id UUID         NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    set_number  INT          NOT NULL,
    reps        INT,
    weight      NUMERIC(6,2),
    duration    INT,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exercise_sets_session_id ON exercise_sets(session_id);
