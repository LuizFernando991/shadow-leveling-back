-- Remove all exercises that came from the exerciseapi.dev import.
-- User-created exercises (external_id IS NULL) are left untouched.
-- The 000025 extension columns are dropped by the 000025 down migration.

DELETE FROM exercises WHERE external_id IS NOT NULL;