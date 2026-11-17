-- Stable per-user CSRF token. This survives multiple tabs and re-login, so an
-- old form is not invalidated just because a new session was created.
ALTER TABLE app_user ADD COLUMN csrf_token TEXT NOT NULL DEFAULT '';

UPDATE app_user
SET csrf_token = lower(hex(randomblob(24)))
WHERE csrf_token = '';
