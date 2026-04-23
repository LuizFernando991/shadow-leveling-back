CREATE TABLE tasks (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    level               VARCHAR(20)  NOT NULL CHECK (level IN ('hard', 'medium', 'easy', 'no_rank')),
    title               VARCHAR(150) NOT NULL,
    description         TEXT,
    initial_date        DATE         NOT NULL,
    final_date          DATE         NOT NULL,
    recurrence_type     VARCHAR(20)  NOT NULL CHECK (recurrence_type IN ('one_time', 'weekly', 'daily', 'monthly', 'custom')),
    custom_days_of_week TEXT[]       NOT NULL DEFAULT '{}',
    is_optional         BOOLEAN      NOT NULL DEFAULT false,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (final_date >= initial_date),
    CHECK (recurrence_type = 'custom' OR cardinality(custom_days_of_week) = 0),
    CHECK (recurrence_type <> 'custom' OR cardinality(custom_days_of_week) > 0)
);

CREATE TABLE task_completions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    completed_date  DATE NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (task_id, completed_date)
);

CREATE INDEX idx_tasks_user_id ON tasks(user_id);
CREATE INDEX idx_tasks_date_range ON tasks(initial_date, final_date);
CREATE INDEX idx_task_completions_task_date ON task_completions(task_id, completed_date);
