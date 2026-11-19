# Kaizen P0 — Windows real-data controlled rollout runbook

**Reference:** `ngtrthanh/chinfon-kaizen` PR #1 -> PR #2.
**Scope:** pilot with actual, HR-authorized staff and authentic initiative only. No demo cases. **Status: NOT DEPLOYED / GATES OPEN.**

## 0. Approvals before processing live personal data

HR must authorize a privacy-minimized CSV with `emp_id,name,job_title,dept_code,dept_name,report_to,e_mail` and a signed effective-date roster. No private phone numbers, DOB, national ID, salary or mobile PIN. Financial rules and reviewer roles signed by HR/Finance; security acceptance and server ownership by MIS. The earlier `main` branch still contains a predictable phone-derived PIN; **do not use**.

## 1. Source freeze and checks (Windows PowerShell)

```powershell
git fetch origin
git switch feature/p0-invite-without-phone-20261119
go test ./... -count=1
go vet ./...
python tools/p0_hr_preflight.py --input "D:\\SecureHR\\staff_approved.csv" --output "D:\\SecureHR\\staff_kaizen_ready.csv" --report "D:\\SecureHR\\preflight.json"
Get-FileHash "D:\\SecureHR\\staff_approved.csv" -Algorithm SHA256
.\build-win.ps1
```

The P0 preflight rejects unresolved `report_to`, cycles, duplicate IDs, privileged roles, private mobile and predictable PIN fields. Output roles are *all WORKER*, not supervisor/manager; a signed HR/authority matrix must be used to assign subsequent review permissions manually. The Kaizen import *replaces* roles when reimporting; it should not be rerun against production accounts without reviewing its privilege changes and diff. Export/import checks apply to one approved full reporting population with all referenced managers, not only a unit without its reporting roots. This CSV is a temporary handoff; it does **not** certify ORG-03/PERSON-01 as live OrgAxis modules.

## 2. Isolated non-demo Kaizen environment

Use a **fresh**, access-controlled path; do not overwrite `data/kaizen.db` from any pilot. Initialize with APP_SEED_DEMO=false. Native HTTP listener remains loopback. Do not directly expose `:8080` to LAN or Internet.

```powershell
$env:APP_DATA_DIR = "D:\\Apps\\Kaizen\\prod"
$env:APP_DB = "D:\\Apps\\Kaizen\\prod\\kaizen.db"
$env:APP_UPLOAD_DIR = "D:\\Apps\\Kaizen\\prod\\uploads"
$env:APP_ADDR = "127.0.0.1:8080"
$env:APP_SEED_DEMO = "false"
$env:COOKIE_SECURE = "true"
# Place a verified HTTPS reverse proxy in front of 127.0.0.1:8080,
# with firewall/ACL allowing only the intended workforce. No public port 8080.
.\bin\kaizen.exe
```

At the first verified HTTPS launch open `/setup` and create the first named ADMIN, enroll TOTP. Complete a second controlled ADMIN and test recovery. Do not run the `COOKIE_SECURE=true` flow using plaintext browser HTTP; secure cookies require HTTPS. Keep application directory and MFA encryption key protected by OS ACLs.

## 3. Import approved HR source (after first ADMIN exists)

Stop the service for a controlled import and backup or execute the command in a controlled maintenance window against the selected DB path.

```powershell
.\bin\kaizen.exe import-staff "D:\\SecureHR\\staff_kaizen_ready.csv"
.\bin\kaizen.exe issue-invite C003
```

The `C003` example is an **illustrative placeholder**. Invite is printed once and valid 30 minutes; deliver individually after verified identification. Never put invitation secrets in logs/tickets/email threads. First login requires password change and TOTP. Staff without a smartphone/authenticator require an independently accepted assisted-access method; P0 does not create a safe no-second-factor bypass.

Reconcile HR row count, active staff count, department codes and manager graph. Record import results and skipped/rejected rows. **Zero missing references and zero privileged import roles** before manager/council/HR access is assigned by authorized admins.

## 4. Reviewers and single reward ledger

HR signs matrix: worker, shift leader/foreman, manager, council, HRD and independent executive; admin separately. Assign via existing Kaizen admin interface, not from `job_title` or unreviewed `report_to`. Check unauthorized cross-dept reads, Luồng 3 confidentiality and no self-approval. Source of truth for initiatives and reward amounts is only the Kaizen database.

In OrgAxis configure `ORGAXIS_KAIZEN_URL=https://<your-approved-domain>` only once the validated TLS endpoint exists. This creates a **link**, not SSO or a live data sync. Once configured OrgAxis legacy BONUS writes are denied; do not enter real rewards into the demo ledger.

## 5. Finance and payment control

Before award: compare PN-B0000-02 version, financial 7 tiers, 1.2x cap, nonfinancial 4 tiers and participant splits with a signed independent calculation; preserve evidence source and approver. Validate HR CSV export against pay list.

In P0 Kaizen, final IMPLEMENT->APPLIED requires a 6–120 character externally checked payment document number saved under `initiative.payment_reference`; a button click alone is insufficient. **The app cannot verify settlement or bank transfers**; HR/Finance must check external evidence. Historical paid records with blank reference must be reconciled and flagged.

## 6. Restore, rollback and no-go

Back up as one consistent snapshot: `kaizen.db` (including WAL as needed), `uploads/`, `mfa.key` and version/config; encrypt at rest, restrict restores. Perform an isolated restore and log evidence. P0 migration 010 must be tested on a copy of any existing database. If integrity, login, MFA, approval/receipt, authorization or recovery fails, disable new submissions/role grants and revert to previous approved release and backup through documented MIS change control.

**No production announcement, payment authority or whistleblower/public-channel deployment without written HR, MIS, Finance and independent security acceptance.**
