DROP INDEX IF EXISTS idx_exercises_category;
DROP INDEX IF EXISTS idx_exercises_primary_muscles;

ALTER TABLE exercises
    DROP COLUMN IF EXISTS mechanic_pt,
    DROP COLUMN IF EXISTS level_pt,
    DROP COLUMN IF EXISTS equipment_pt,
    DROP COLUMN IF EXISTS secondary_muscles_pt,
    DROP COLUMN IF EXISTS primary_muscles_pt,
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS mechanic,
    DROP COLUMN IF EXISTS level,
    DROP COLUMN IF EXISTS force,
    DROP COLUMN IF EXISTS equipment,
    DROP COLUMN IF EXISTS secondary_muscles,
    DROP COLUMN IF EXISTS primary_muscles,
    DROP COLUMN IF EXISTS external_id;