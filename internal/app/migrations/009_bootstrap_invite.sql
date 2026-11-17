-- ASVS 5.0 V6.4.1: no bootstrap secret derived from personal data.
ALTER TABLE app_user ADD COLUMN bootstrap_token_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE app_user ADD COLUMN bootstrap_expires_at TEXT NOT NULL DEFAULT '';
