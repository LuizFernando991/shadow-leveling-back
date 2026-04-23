CREATE TABLE exercises (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    type       VARCHAR(20)  NOT NULL CHECK (type IN ('repetition', 'time')),
    unit       VARCHAR(20)  NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
