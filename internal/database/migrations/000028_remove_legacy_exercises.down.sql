-- Best-effort reverse: the rebindings to canonical imported twins are
-- irreversible (the legacy rows are gone). A true rollback would require
-- re-inserting the exercise stubs and re-binding workout_exercises /
-- exercise_sets back to them — generally unwanted since the canonical rows
-- are superior (carry muscle/equipment/mechanic data).
--
-- No-op down migration acknowledged.
SELECT 1;