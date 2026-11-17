-- TOTP MFA and per-account backoff.
-- The TOTP secret is encrypted with an external key (data/mfa.key or
-- APP_MFA_KEY); only ciphertext is stored here.
ALTER TABLE app_user ADD COLUMN totp_secret_enc TEXT NOT NULL DEFAULT '';
ALTER TABLE app_user ADD COLUMN totp_confirmed INTEGER NOT NULL DEFAULT 0 CHECK(totp_confirmed IN (0,1));
ALTER TABLE app_user ADD COLUMN last_totp_counter INTEGER NOT NULL DEFAULT 0;
ALTER TABLE app_user ADD COLUMN failed_login_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE app_user ADD COLUMN locked_until TEXT;

-- A password-authenticated session is partial (mfa_passed=0) until the TOTP
-- code is verified; only then does withUser allow normal pages.
ALTER TABLE app_session ADD COLUMN mfa_passed INTEGER NOT NULL DEFAULT 0 CHECK(mfa_passed IN (0,1));
