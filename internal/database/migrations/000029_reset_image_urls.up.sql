-- Image object keys moved to an owner-scoped layout (users/<id>/…,
-- groups/<id>/cover). The stored URLs are absolute Firebase download URLs, so
-- they still point at objects under the old keys. Only test data existed, so
-- the bucket is wiped by hand and every URL is cleared instead of migrated.
UPDATE users SET avatar_url = NULL WHERE avatar_url IS NOT NULL;
UPDATE groups SET cover_url = NULL WHERE cover_url IS NOT NULL;
UPDATE workout_sessions SET photo_url = NULL WHERE photo_url IS NOT NULL;
