# MFA (TOTP) — Chinfon Kaizen 360 v3

MFA is mandatory for every account. It follows the `cem-auth` model: password
(or first-login mobile PIN) + a 6-digit TOTP code, encrypted secret at rest,
one-use codes, account backoff, and admin/CLI recovery.

Passkeys/WebAuthn are **not** included in this version; TOTP is the second
factor.

## 1. Sign-in flow

```text
username + password/PIN
        |
        v
partial session (10 minutes, mfa_passed=0)
        |
        +-- must change password? --> /account --> set a strong password
        |
        +-- MFA already enrolled? --> /mfa/verify --> 6-digit TOTP
        |
        +-- no MFA yet? ------------> /mfa/setup  --> QR + confirm code
        |
        v
full session (12 hours, mfa_passed=1)
```

A partial session can only open `/mfa/setup`, `/mfa/verify`, `/mfa/qr.png`,
and logout. It cannot reach the dashboard, admin pages or any initiative data.

A CSV row with no `mobile_num` is imported as personnel only; it has no account
until the mobile is added and the row is re-imported.

## 2. Enrolment

1. Sign in with password/PIN.
2. The page shows a QR code and a manual base32 secret.
3. Scan with Google Authenticator, Microsoft Authenticator, Authy or any TOTP app.
4. Enter the current 6-digit code.
5. The secret is confirmed and the full session starts.

- Algorithm: SHA1, 6 digits, 30-second period (standard TOTP).
- Accepted window: ±1 step for clock drift.
- A code can be used only once. The last accepted counter is stored in
  `app_user.last_totp_counter`; replay is rejected.
- The QR is generated locally (`skip2/go-qrcode`); no external QR service.

## 3. Secret storage

- The TOTP secret is encrypted with **AES-256-GCM** before it is written to
  `app_user.totp_secret_enc`.
- The key is 32 bytes, from:
  - `APP_MFA_KEY` (base64 or 64 hex characters), or
  - `data/mfa.key` (auto-created with mode 0600 next to the SQLite DB).
- The key is **not** stored in SQLite. Back it up with the DB.
- If the key is lost, existing TOTP secrets cannot be decrypted; use
  `reset-mfa`/admin reset for each account.
- On Windows, file mode 0600 is best-effort; restrict the folder with NTFS ACLs.

## 4. Backoff and lockout

- Existing per-IP+username limiter: 10 failures / 10 minutes.
- Per-account counter:
  - after 5 failures: lock 1 minute;
  - then 2, 4, 8, up to 15 minutes.
- Applies to password failures and TOTP failures/replay.
- A successful MFA verification resets the counter.
- Generic login errors do not reveal whether the username, password or code was
  wrong. A locked account shows only “try again later”.

## 5. Recovery

### ADMIN in the app

**Cài đặt & RBAC → Người dùng & vai trò → Reset MFA**

- Another ADMIN can reset a user’s MFA.
- Nobody can reset their own MFA from the UI.
- Reset clears the TOTP secret, replay counter, lock and all sessions.
- The user signs in with password, then enrols a new authenticator.

### CLI break-glass

If the only ADMIN has lost the authenticator:

```powershell
cd C:\Users\User\App\01-service\initiative-app\v3
.\bin\kaizen.exe reset-mfa <username>
# or, without building:
go run ./cmd/kaizen reset-mfa <username>
```

If the only ADMIN also forgot the password:

```powershell
.\bin\kaizen.exe reset-password <username>
```

It prints a one-time temporary password, sets `must_change_password=1`, and
revokes sessions. MFA is unchanged; run `reset-mfa` too if the authenticator is
also lost.

Stop the server first if possible. The commands use the same `APP_DATA_DIR` /
`APP_DB` and `APP_MFA_KEY`/`data/mfa.key`. They do not change roles.

## 6. Sessions and headers

- Partial session: 10 minutes.
- Full session: 12 hours.
- Session cookie: `HttpOnly`, `SameSite=Strict`, `Secure` when `COOKIE_SECURE=true`.
- Password reset, MFA reset, user disable or password change revokes sessions.
- HTML pages are `Cache-Control: no-store`.
- CSP: `default-src 'self'`, no inline script/style.
- HSTS is sent when `COOKIE_SECURE=true`; run behind HTTPS.

## 7. Admin/API checks

- MFA setup/verify require the partial session and CSRF token.
- Admin reset cannot target the current admin.
- The last active ADMIN is protected.
- Audit events: `MFA_ENROLLED`, `MFA_VERIFIED`, `MFA_ENROLL_FAILED`,
  `MFA_VERIFY_FAILED`, `MFA_LOCKED`, `MFA_RESET`.
- `audit_log`/`initiative_history` are append-only via SQLite triggers.

## 8. Tests

```powershell
cd v3
go test ./... -count=1
```

Covers:
- RFC 6238 SHA1 vectors and TOTP window;
- AES-GCM secret round-trip and wrong-key rejection;
- first-login PIN → password change → MFA enrolment;
- MFA challenge, replay rejection and full-session gating;
- QR route (`image/png`), CLI reset;
- existing RBAC/scope, staff import and reward tests.

## 9. Not included (next hardening steps)

- WebAuthn/passkeys.
- Recovery codes (use ADMIN reset/CLI).
- Email/SMS OTP.
- Central rate limiting across multiple instances.
- Recent-auth requirement (15 minutes) for sensitive admin actions.
- Per-IP-only throttle; current throttle is account + IP+username.
