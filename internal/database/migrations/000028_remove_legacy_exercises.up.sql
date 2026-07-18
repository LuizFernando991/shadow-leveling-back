-- Final cleanup of hand-created exercises that pre-date the 000026 import.
-- These 5 rows have external_id=NULL and primary_muscles='{}' — they were
-- created via POST /exercises before the catalog import and don't carry the
-- structured data the substitute feature needs. After the import, every one
-- has an equivalent canonical row in the catalog with the same intent.
--
-- Mapping is explicit (no prefix heuristics): the failed 000027 retry taught
-- us that naive LIKE matching picks up variants like "Agachamento Tempo" as a
-- substitute for "Agachamento". Each pair below is the canonical substitute
-- mapping by name intent, chosen by hand.

-- 1) Rebind workout_exercises from legacy → canonical imported twin.
UPDATE workout_exercises we
SET exercise_id = imp.id
FROM (VALUES
    -- (legacy_id, canonical_external_id)
    ('5489b650-0df1-4cf8-8015-688dfe6220da', 'Barbell_Squat'),                                -- Agachamento → Agachamento com Barra
    ('a181de8e-4eac-4105-a376-cdb6902a5fa1', 'Barbell_Shoulder_Press'),                       -- Desenvolvimento → Desenvolvimento com Barra
    ('fb9d4257-2a40-4272-a092-d0fb475ce5f7', 'Barbell_Incline_Bench_Press_-_Medium_Grip'),    -- Supino inclinado → Supino Inclinado com Barra - Pegada Média
    ('888074bc-cb55-43a2-a85e-db7ee957d3e9', 'Barbell_Bench_Press_-_Medium_Grip'),            -- Supino reto → Supino Reto com Barra - Pegada Média
    ('7ce74df4-5019-4756-9e51-24dc51c20bb0', 'Cable_Tricep_Rope_Pushdown')                    -- Triceps corda → Tríceps na Polia com Corda
) AS map(legacy_id, canonical_external_id)
JOIN exercises imp ON imp.external_id = map.canonical_external_id
WHERE we.exercise_id = map.legacy_id::uuid;

-- 2) Rebind exercise_sets (historical session records) the same way.
UPDATE exercise_sets es
SET exercise_id = imp.id
FROM (VALUES
    ('5489b650-0df1-4cf8-8015-688dfe6220da', 'Barbell_Squat'),
    ('a181de8e-4eac-4105-a376-cdb6902a5fa1', 'Barbell_Shoulder_Press'),
    ('fb9d4257-2a40-4272-a092-d0fb475ce5f7', 'Barbell_Incline_Bench_Press_-_Medium_Grip'),
    ('888074bc-cb55-43a2-a85e-db7ee957d3e9', 'Barbell_Bench_Press_-_Medium_Grip'),
    ('7ce74df4-5019-4756-9e51-24dc51c20bb0', 'Cable_Tricep_Rope_Pushdown')
) AS map(legacy_id, canonical_external_id)
JOIN exercises imp ON imp.external_id = map.canonical_external_id
WHERE es.exercise_id = map.legacy_id::uuid;

-- 3) Delete the legacy rows. Nothing references them anymore (rebindings done
--    above). workout_exercises and exercise_sets both expose ON DELETE CASCADE
--    for exercise_id, so any *other* legacy reference we missed would also
--    clean up — but we want the rebind path, not the cascade path, so the
--    UPDATEs run first.
DELETE FROM exercises
WHERE external_id IS NULL
  AND cardinality(primary_muscles) = 0;