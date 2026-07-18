-- Reverse is best-effort: We can't know which legacy row each rebind came
-- from (names matched but the legacy row is gone, and multiple workouts might
-- have shared it). The intent of the down migration is to drop the table
-- effects of the up; if a rollback is ever needed, re-import the legacy row
-- manually with POST /exercises (it will land with primary_muscles='{}' as
-- before) and re-bind the affected workout_exercises.
--
-- As a partial undo: any exercises that were imported as part of 000026 stay
-- untouched; only the rebinding is unreversible. Acknowledged empty down:
SELECT 1; -- no-op