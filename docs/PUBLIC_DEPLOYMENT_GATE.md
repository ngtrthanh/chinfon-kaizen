# Public / production deployment gate

v3 is safe for a local/internal pilot on a trusted LAN. Before exposing it to the
public Internet or using it as the only reward ledger, complete the items below.

## Identity & access

- [ ] Replace local password login with company SSO/AD (or enforce strong
      password rotation and MFA).
- [ ] Add login rate limiting / lockout and alerting for repeated failures.
      v3.1 has an in-memory 10-failure/10-minute limiter; make it durable and
      shared before multi-instance deployment.
- [ ] Confirm identity proofing and distribution for 30-minute random invites; adopt SSO for mass onboarding. The 5-digit mobile PIN is disabled.
- [ ] Confirm MFA is mandatory: the first admin, every imported worker, manager,
      council member and HRD must enrol a TOTP app before accessing data.
- [ ] Back up `data/mfa.key` (or `APP_MFA_KEY`) separately from the DB. Without
      it, all TOTP secrets must be reset.
- [ ] Test ADMIN reset MFA and the `reset-mfa` CLI before go-live; keep a
      break-glass procedure for the only ADMIN.
- [ ] Confirm the 12-hour full session and 10-minute partial session are
      acceptable; consider reducing full-session lifetime for privileged roles.
- [ ] Passkeys/WebAuthn are not included; decide whether TOTP-only MFA is
      sufficient for the risk profile.
- [ ] Confirm that all imported `mobile_num` values are accurate; the last five
      digits are a login factor.
- [ ] Review the RBAC matrix with HRD; least privilege for `CEO_AUDIT` and
      `ADMIN`; at least two break-glass admins.
- [ ] Confirm who may be `dept_scope=ALL`; every such account can see all
      non-confidential records.
- [ ] Confirm the Luồng 3 visibility rule (submitter can see own report).

## Staff import

- [ ] Dry-run the CSV on a copy of the database; check row errors and department
      names.
- [ ] Verify file/header errors block the import while row errors skip only the
      affected rows; at least one ADMIN remains.
- [ ] Review the “Dòng bỏ qua” list after the first import and fix/import those
      rows again.
- [ ] Decide how to handle rows without `mobile_num`: they import as personnel
      but have no account until a later re-import supplies the mobile.
- [ ] Confirm no demo data is required; migration 007 archives/hides demo records
      and production does not seed new ones.
- [ ] Export the staff list and reconcile headcount with HR before go-live.
- [ ] Test random invite + forced password change for a worker, a manager, a
      council member and an admin.
- [ ] Remove/rotate the temporary `pending` accounts that never logged in.

## Data protection

- [ ] Run on HTTPS; set `COOKIE_SECURE=true`; terminate TLS at a reverse proxy.
- [ ] Encrypt disk/backups; restrict OS access to `data/`. SQLite + local files
      are not an access-control boundary.
- [ ] Define retention for `audit_log`, attachments and personal data.
- [ ] Test restore from a backup, not just backup creation.
- [ ] Consider PostgreSQL + object storage before concurrent multi-site use;
      the SQL layer is centralized but not a zero-change driver swap.

## Integrity & audit

- [ ] Run an independent review of the 7-tier reward formula and the 1.2x cap
      with HRD/Finance (see docs/MIGRATION_V1_V2.md, open decisions).
- [ ] Decide whether final approval may override the computed amount; record
      the reason.
- [ ] Verify the HRD CSV against payroll before the first real payment.
- [ ] Keep database triggers enabled; do not grant application accounts DDL
      rights in a future PostgreSQL deployment.
- [ ] Add notification/escalation for overdue SLA (currently dashboard only).

## Network exposure

- [ ] Confirm `start-lan.cmd` is only used on trusted LAN/Tailscale, not exposed
      directly to the public Internet.
- [ ] Keep the firewall rule scoped to the Tailscale interface/CGNAT range
      (`100.64.0.0/10`), or restrict `RemoteAddress` to known peer IPs.
- [ ] If public access is required, put v3 behind HTTPS/reverse proxy, set
      `COOKIE_SECURE=true`, and do not publish port 8080 directly.
- [ ] Check Tailscale ACLs allow TCP 8080 only for the intended users/devices.

## Operations

- [ ] Health check `/healthz` monitored.
- [ ] Log aggregation for `kaizen:` log lines.
- [ ] Scheduled backup with restore test.
- [ ] Capacity check for attachments (10 MB/file) and SQLite WAL growth.
- [ ] Privacy notice for employees; whistleblower handling procedure.

Until these are complete, use v3 as an internal pilot, not as the sole
authoritative record for payroll or a public whistleblowing channel.
