CREATE TABLE workout_sessions (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workout_id UUID        NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    date       DATE        NOT NULL,
    status     VARCHAR(20) NOT NULL CHECK (status IN ('complete', 'incomplete', 'skipped')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workout_sessions_workout_id ON workout_sessions(workout_id);
CREATE INDEX idx_workout_sessions_date       ON workout_sessions(date);
