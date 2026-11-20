> **P0 update — 2026-11-19:** Workers without email or personal phone are now provisioned as PENDING local accounts on import. A verified server administrator must issue `kaizen issue-invite <emp_id>` (random, 30-minute, one use), delivered individually and privately. The old text below about skipping no-mobile accounts is superseded. This does not substitute for a formally verified employee roster and sign-off. No PIN derived from phone or DOB is accepted.

> **Security update (17 Nov 2026):** Sections describing the five-digit mobile PIN reflect previous releases and are superseded. Only random, expiring invitations are accepted. 

# Staff import guide — P81 / 1,000 employees

This guide is the exact contract for **Cài đặt & RBAC → Nhân sự & Import**.

## 1. TL;DR

1. Open `/admin?tab=staff`.
2. Optional: use **Bước 0 — Seed một đơn vị** to create `P81` and its real name.
3. Download **CSV mẫu 10 dòng** or **Template trống**.
4. Fill one row per employee, keep the header exactly as-is.
5. Import. Valid rows are committed; invalid rows are skipped and listed. File/header errors block the whole import.
6. After success, no demo data is seeded. Rows with `mobile_num` get an account; rows without it are imported as personnel only and can be provisioned later.

## 2. Columns — only these 11 are needed

```text
emp_id,name,job_title,dept_code,dept_name,report_to,e_mail,mobile_num,seed_pin,role,group
```

| Column | Required | What to enter | Example |
|---|---|---|---|
| `emp_id` | **Yes** | Unique employee ID; becomes the login username. Max 50. | `P81-0001` |
| `name` | **Yes** | Full name. Max 120. | `Nguyễn Văn A` |
| `job_title` | No | Job title / position. Max 120. | `Kỹ sư Cơ khí` |
| `dept_code` | **Yes** | Department code. Creates the department if missing. Max 20. | `P81` |
| `dept_name` | No | Department name. If new and blank, the code is used. | `Phòng Kỹ thuật 81` |
| `report_to` | No | Manager's `emp_id`. Stored for reference only, no permission effect. | `P81-0000` |
| `e_mail` | No | Email if available. **Leave blank if none** — do not write `N/A`. Must be a valid email if filled. | `a.nguyen@chinfon.com` |
| `mobile_num` | No* | 5–15 digits. An administrator must issue a random 30-minute invite; the phone is not a credential. If blank, the employee is imported but **no account** is created. Keep it as Text in Excel. | `0912345678` |
| `seed_pin` | No | Leave blank normally. If filled, it must be exactly the last 5 digits of `mobile_num`. It is **not stored**. | `45678` |
| `role` | No | Blank = `employee`. Multiple roles separated by `\|`. Case-insensitive. | `manager\|employee` |
| `group` | No | Team / shift / group within the department. Max 120. | `Tổ Cơ khí` |

There is no other required field. Extra columns are ignored.

For an existing employee, a blank `job_title`, `group`, `e_mail` or `report_to`
keeps the current value. To clear one, edit it in **Master data → Nhân viên**.
`mobile_num` is optional for the employee record: a blank mobile means the account
is skipped, and a later import with the mobile will provision it.

## 3. Role: `admin` or `ADMIN`?

**Both work.** Role matching is case-insensitive; the stored code is uppercase.

Accepted values:

| CSV value | Stored role | Scope | Main ability |
|---|---|---|---|
| `employee` / `EMPLOYEE` / `worker` | `WORKER` | own department | Submit ideas; see own/participating records |
| `supervisor` / `SUPERVISOR` | `SUPERVISOR` | own department | Confirm field facts |
| `manager` / `MANAGER` | `MANAGER` | own department | Confirm + ROI review |
| `director` / `DIRECTOR` | `DIRECTOR` | all | Review + final approval |
| `council` / `COUNCIL` | `COUNCIL` | all | Council review + final approval; **cannot see Luồng 3** |
| `hr` / `HR` / `hrd` / `HRD` | `HRD` | all | Payroll export; **cannot see Luồng 3** |
| `ceo` / `CEO` / `ceo_audit` | `CEO_AUDIT` | all | Final approval + unlock Luồng 3 |
| `admin` / `ADMIN` | `ADMIN` | all | All permissions + RBAC |

Multiple roles are additive: `manager|employee` gives both. If any role has `ALL` scope (`council`, `hr`, `director`, `ceo`, `admin`), the account gets `ALL`; otherwise the account gets `dept_code`.

Use `admin`/`ADMIN` only for real system administrators. The import refuses to remove the last active ADMIN.

## 4. Minimum sample — one P81 department

```csv
emp_id,name,job_title,dept_code,dept_name,report_to,e_mail,mobile_num,seed_pin,role,group
P81-0001,Nguyễn Văn A,Kỹ sư Cơ khí,P81,Phòng Kỹ thuật 81,P81-0000,a.nguyen@chinfon.com,0912345678,,employee,Tổ Cơ khí
P81-0002,Trần Thị B,Giám sát vận hành,P81,Phòng Kỹ thuật 81,P81-0001,,0909876543,76543,supervisor,Ca A
P81-0003,Lê Văn C,Trưởng bộ phận,P81,Phòng Kỹ thuật 81,P81-0001,,0901112222,,manager|employee,Văn phòng
P81-0004,Phạm Thị D,Kỹ thuật viên,P81,Phòng Kỹ thuật 81,P81-0003,,0902223333,,employee,Tổ Điện
P81-0005,Hoàng Văn E,Thành viên Hội đồng,P81,Phòng Kỹ thuật 81,P81-0003,,0903334444,,council,Hội đồng ĐGKT
P81-0006,Vũ Thị F,Chuyên viên Nhân sự,P81,Phòng Kỹ thuật 81,P81-0000,,0904445555,,hr,Phòng HRD
P81-0007,Đỗ Văn G,Quản trị hệ thống,P81,Phòng Kỹ thuật 81,P81-0000,,0905556666,,admin,MIS
P81-0008,Ngô Thị H,Công nhân vận hành,P81,Phòng Kỹ thuật 81,P81-0001,,0906667777,,employee,Ca B
P81-0009,Bùi Văn I,Kỹ sư Điện,P81,Phòng Kỹ thuật 81,P81-0001,,0907778888,,employee,Tổ Điện
P81-0010,Dương Thị K,KCS,P81,Phòng Kỹ thuật 81,P81-0003,,0908889999,,employee,Tổ KCS
```

Files:

- In-app download: `/admin/staff/sample.csv` (with BOM, ready for Excel).
- Repo copy: [`docs/sample_staff_P81.csv`](sample_staff_P81.csv).
- Empty header template: `/admin/staff/template.csv`.

## 5. Excel instructions

1. Open the sample or template.
2. Keep `emp_id`, `mobile_num`, `seed_pin` as **Text** so leading zeros are not lost.
3. Save as **CSV UTF-8 (Comma delimited)**.
4. Do not add merged cells, formulas, or a title row above the header.
5. Do not rename the header columns.

## 5.1 Import directly from `data/uploads`

Place the file at:

```text
C:\Users\User\App\01-service\initiative-app\v3\data\uploads\staff.csv
```

Then run:

```powershell
cd C:\Users\User\App\01-service\initiative-app\v3
.\import-staff.cmd
```

- If no path is given, the CLI looks for `staff.csv`, `personnel.csv`,
  `employees.csv`, `nhan_su.csv`, or the only `.csv` in the uploads root.
- Pass a path to use a different file: `.\import-staff.cmd "C:\...\P81.csv"`.
- The legacy `--reset-pin` option resets an account to pending; a new invite is then required.
- The CLI prints row counts and all validation errors. It does not need an ADMIN login.
- Stop the server first if possible.

## 6. What happens on import

- The file/header is validated first. A file-level or missing-required-column
  error blocks the whole import.
- A row-level error (missing `emp_id`, `name`, `dept_code`, duplicate `emp_id`,
  invalid role/email/PIN) skips **only that row**; other valid rows are imported.
- Valid rows are committed in one transaction, so the partial import is atomic
  for the rows that passed.
- Missing `mobile_num` is **not** a row error: the employee is imported, but the
  account is skipped and counted in “Chưa tạo TK”. Add the mobile later and
  re-import the same `emp_id` to create the account.
- Department `P81` is created/updated automatically from `dept_code` / `dept_name`.
- Employee master data is inserted/updated.
- Each row **with a mobile** gets an account:
  - username = `emp_id`;
  - `password_hash='pending'`;
  - initial sign-in secret is issued separately by authorized server operator;
  - forced password change on first login, then MFA enrolment.
- Roles/scope of every `emp_id` present in the file are **replaced** by the CSV values
  (when an account exists or is created).
- The last active ADMIN is protected.
- Demo mode is disabled by default; no demo data is seeded.
- The import is limited to 5,000 rows / 8 MB.

## 7. Secure initial login (ASVS V6.4.1)

Initial passwords derived from phone numbers are disabled. After staff import, each account remains pending until an operator with server-console authority runs:

```powershell
.\bin\kaizen.exe issue-invite P81-0001
```

The command issues a random secret valid for 30 minutes. Transfer the secret privately after checking employee identity. The user signs in, changes password, then enrolls MFA. If it expires, issue a new secret. For company-wide provisioning, plan corporate SSO or an approved enrollment service.

## 8. Export

`/admin/staff/export.csv` exports:

```text
emp_id,name,job_title,dept_code,dept_name,report_to,e_mail,mobile_num,role,group,username,scope,account_status,mfa
```

- No PIN or password hash is exported.
- `account_status`: `no_account`, `must_change_password`, or `active`.
- `mfa`: `no_account`, `not_enrolled`, or `enrolled`.
- The export can be edited and re-imported; `role` codes such as `MANAGER|WORKER` are accepted.

## 9. Common errors

| Error | Fix |
|---|---|
| `Thiếu emp_id` | Fill column A. |
| `emp_id bị trùng trong file` | Remove the duplicate row. |
| `Thiếu name` | Fill the full name. |
| `Thiếu dept_code` | Fill `P81` or the correct department. |
| `mobile_num phải có 5–15 chữ số` | Mobile is present but malformed; fix it or leave blank to import without an account. |
| `seed_pin phải bằng 5 số cuối của mobile_num` | Leave `seed_pin` blank or make it match. |
| `e_mail không hợp lệ` | Leave blank if no email. |
| `vai trò không hợp lệ` | Use one of the accepted role values; separate multiple roles with `\|`. |
| `import sẽ không còn tài khoản ADMIN` | Include at least one active ADMIN row. |
| `Không thể thu hồi quyền ADMIN cuối cùng` | Keep another ADMIN account. |

## 10. Before importing 1,000 rows

- [ ] Confirm the exact `dept_code`/`dept_name` (P81).
- [ ] Confirm mobile numbers; a random invitation must be issued.
- [ ] Mark at least one `council` and one `admin` row.
- [ ] Remove sample rows or replace them with real employees.
- [ ] Decide whether current records should be archived as demo.
- [ ] Test short-lived invitation + forced password change + MFA enrolment for a worker,
      a manager, a council member and an admin.
- [ ] Test ADMIN reset MFA and the `reset-mfa` CLI.
- [ ] Export the staff list after import and reconcile headcount with HR.
- [ ] Remove/rotate the temporary `pending` accounts that never logged in.
