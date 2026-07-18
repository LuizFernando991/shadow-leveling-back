-- Cleanup: before the 000026 import seed, the user could create exercises by
-- hand via POST /exercises that ended up empty (no catalog attributes — those
-- columns didn't exist yet — and so a NULL primary_muscles). The 000026 seed
-- later imported the same exercise with its full muscles/equipment/force/etc.
-- The 000025 ALTER added those columns so legacy rows now show
-- primary_muscles = '{}' while the imported twin carries the real data.
--
-- This migration re-binds any workout_exercises / exercise_sets pointing to a
-- legacy empty-muscles row to its imported twin (same name, case-insensitive),
-- then deletes the legacy row. Exercises created by hand that have NO imported
-- twin (genuinely custom) are left alone — the substitute query handles them
-- by returning an empty suggestion set so the user falls back to manual search.

-- 1) Re-bind workout_exercises.exercise_id from legacy → imported twin.
UPDATE workout_exercises we
SET exercise_id = imp.id
FROM exercises leg
JOIN exercises imp
  ON lower(trim(imp.name)) = lower(trim(leg.name))
 AND imp.external_id IS NOT NULL
WHERE we.exercise_id = leg.id
  AND leg.external_id IS NULL
  AND cardinality(leg.primary_muscles) = 0;

-- 2) Re-bind exercise_sets.exercise_id from legacy → imported twin.
UPDATE exercise_sets es
SET exercise_id = imp.id
FROM exercises leg
JOIN exercises imp
  ON lower(trim(imp.name)) = lower(trim(leg.name))
 AND imp.external_id IS NOT NULL
WHERE es.exercise_id = leg.id
  AND leg.external_id IS NULL
  AND cardinality(leg.primary_muscles) = 0;

-- 3) Delete the legacy rows now that nothing references them.
DELETE FROM exercises
WHERE external_id IS NULL
  AND cardinality(primary_muscles) = 0
  AND EXISTS (
      SELECT 1 FROM exercises imp
      WHERE imp.external_id IS NOT NULL
        AND lower(trim(imp.name)) = lower(trim(exercises.name))
  );