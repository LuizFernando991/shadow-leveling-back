-- Extend the exercises table to hold structured attributes imported from
-- exerciseapi.dev (see cmd/fetch_exercises). The original minimal columns
-- (id, name, type, unit, created_at) are kept; these additions unlock the
-- "Trocar Exercício" feature, which recommends substitutes by muscle overlap,
-- force, and mechanic.
--
-- Each attribute is stored twice: the EN value as returned by the upstream
-- API (audit/re-sync key) and the PT translation for display. PT columns are
-- nullable so user-created exercises (POST /exercises, external_id NULL) and
-- any future upstream value not yet translated degrade to NULL instead of
-- forcing a NOT NULL DEFAULT.
--
-- external_id is the exerciseapi.dev id (Capitalized_Snake_Case string); it is
-- nullable so user-created exercises keep coexisting without a source. UNIQUE
-- admits multiple NULLs in Postgres.
--
-- primary_muscles / secondary_muscles are TEXT[] matching the days_of_week
-- precedent in 000008. GIN index on primary_muscles accelerates the
-- intersection operator (&&) used by the substitute query. The PT mirror is
-- not indexed — it is display-only.

ALTER TABLE exercises
    ADD COLUMN external_id             TEXT UNIQUE,
    ADD COLUMN primary_muscles           TEXT[]   NOT NULL DEFAULT '{}',
    ADD COLUMN secondary_muscles         TEXT[]   NOT NULL DEFAULT '{}',
    ADD COLUMN equipment                 VARCHAR(50),
    ADD COLUMN force                     VARCHAR(20),
    ADD COLUMN level                     VARCHAR(20),
    ADD COLUMN mechanic                  VARCHAR(20),
    ADD COLUMN category                  VARCHAR(20),
    -- PT translations (display layer). NULL when untranslated or user-created.
    ADD COLUMN primary_muscles_pt        TEXT[]   NOT NULL DEFAULT '{}',
    ADD COLUMN secondary_muscles_pt      TEXT[]   NOT NULL DEFAULT '{}',
    ADD COLUMN equipment_pt              VARCHAR(50),
    ADD COLUMN level_pt                  VARCHAR(20),
    ADD COLUMN mechanic_pt               VARCHAR(20);

CREATE INDEX idx_exercises_primary_muscles ON exercises USING GIN (primary_muscles);
CREATE INDEX idx_exercises_category        ON exercises (category);