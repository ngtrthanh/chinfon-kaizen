> **Security notice — ASVS 5.0 L2 pilot:** Old instructions mentioning mobile-derived initial PINs are superseded. Use `issue-invite` and verify all security gates before production. 

# Chinfon Kaizen 360° — v3

Bản mới của ứng dụng sáng kiến & khen thưởng, xây dựng lại từ hai nguồn:

- **v2**: quy trình PN-B0000-02 (2026), 3 luồng, khung 7 mức thưởng, Phụ lục 03 (1.2x), giao diện điều hành.
- **v1**: các điểm mạnh kỹ thuật — đăng nhập thật, CSRF, master data, RBAC server-side, lịch sử bất biến, tệp đính kèm có SHA-256, kiểm thử.

v3 là một ứng dụng Go **single binary**, không Node.js, không build frontend, không CDN. Toàn bộ template/CSS/JS/migration được nhúng vào file thực thi.

> **ASVS 5.0 L2 implementation in progress:** see [security/asvs/README.md](security/asvs/README.md). This branch is NOT an ASVS-compliant release. Phone-derived PIN bootstrap is disabled; use `kaizen issue-invite <username>` from the server console after importing staff. The token expires in 30 minutes. Existing pending accounts require provisioning. Internet/LAN access requires an HTTPS reverse proxy and secure cookies. Old `start-lan.cmd` direct HTTP access is disabled.

## 1. Chạy nhanh trên Windows

```powershell
cd v3
.\run.ps1
```

Mở `http://127.0.0.1:8080`. Lần đầu hệ thống chuyển tới `/setup` để tạo tài khoản `ADMIN` đầu tiên. **Không có mật khẩu mặc định.**

Build file chạy:

```powershell
.\build-win.ps1
.\bin\kaizen.exe
```

Yêu cầu Go 1.27.4+ (bản vá bảo mật công bố 10/11/2026). Driver SQLite là `modernc.org/sqlite` (pure Go, không cần CGO).

### Truy cập LAN / Tailscale

Mặc định `start.cmd` chỉ mở `127.0.0.1`. Để mở cho máy khác trong LAN/Tailscale:

```powershell
cd v3
.\start-lan.cmd
```

Script bind `0.0.0.0:8080` và in các URL, ví dụ:

```text
http://192.168.16.214:8080
http://100.104.181.127:8080   (Tailscale)
```

Máy này có Tailscale IP `100.104.181.127`. Firewall rule đã tạo:

```text
Chinfon Kaizen 360 v3 (Tailscale 8080)
Inbound TCP 8080, RemoteAddress 100.64.0.0/10, Interface Tailscale, Allow
```

Nếu cần tạo lại rule (PowerShell Admin):

```powershell
New-NetFirewallRule -DisplayName 'Chinfon Kaizen 360 v3 (Tailscale 8080)' `
  -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8080 `
  -RemoteAddress 100.64.0.0/10 -InterfaceAlias 'Tailscale' -Profile Any
```

Tailscale ACL cũng phải cho phép TCP 8080 giữa các thiết bị. Traffic qua Tailscale đã được mã hóa; nếu mở ra Internet thay vì Tailscale, phải dùng HTTPS/reverse proxy và đặt `COOKIE_SECURE=true`.

## 2. Cấu hình

| Biến môi trường | Mặc định | Ý nghĩa |
|---|---|---|
| `APP_ADDR` | `127.0.0.1:8080` | Địa chỉ HTTP |
| `APP_DATA_DIR` | `data` | Thư mục dữ liệu |
| `APP_DB` | `data/kaizen.db` | File SQLite |
| `APP_UPLOAD_DIR` | `data/uploads` | Thư mục tệp đính kèm |
| `APP_SEED_DEMO` | `false` | Mặc định tắt. Migration 007 đặt `demo_mode=0`; không nạp dữ liệu minh họa |
| `APP_MFA_KEY` | (tự tạo `data/mfa.key`) | Khóa 32 byte mã hóa TOTP secret; base64 hoặc 64 hex. Nếu không đặt, app tạo file `mfa.key` cạnh DB |
| `COOKIE_SECURE` | `false` | Đặt `true` khi chạy sau HTTPS |

Dữ liệu minh họa đã tắt mặc định. Không cần đặt `APP_SEED_DEMO`; biến này chỉ dùng khi phát triển.

## 3. Nhập nhân sự & chế độ vận hành

Vào **Cài đặt & RBAC → Nhân sự & Import** (`/admin?tab=staff`).

### 3.1 Seed một đơn vị (ví dụ P81)

Dùng form **Bước 0 — Seed một đơn vị**. Import CSV cũng tự tạo đơn vị từ cột `dept_code` nếu chưa có.

### 3.2 Template CSV (khuyến nghị thay cho XLSX)

Hướng dẫn đầy đủ theo từng cột: [`docs/STAFF_IMPORT_GUIDE.md`](docs/STAFF_IMPORT_GUIDE.md).

Tải `chinfon_staff_template.csv` (header trống) hoặc `chinfon_staff_P81_sample.csv` (10 dòng mẫu) tại trang nhân sự. Cột:

```text
emp_id,name,job_title,dept_code,dept_name,report_to,e_mail,mobile_num,seed_pin,role,group
```

| Cột | Bắt buộc | Ghi chú |
|---|---|---|
| `emp_id` | Có | Mã nhân viên duy nhất; cũng là username đăng nhập |
| `name` | Có | Họ tên |
| `job_title` | Không | Chức danh / vị trí công việc, tối đa 120 ký tự |
| `dept_code` | Có | Ví dụ `P81`; tạo đơn vị tự động |
| `dept_name` | Không | Tên đơn vị; trống sẽ dùng mã |
| `report_to` | Không | Mã quản lý trực tiếp |
| `e_mail` | Không | Có thể trống; nếu có phải đúng định dạng |
| `mobile_num` | Không* | 5–15 chữ số; Mã ngẫu nhiên cấp qua CLI, không lấy từ số điện thoại. Nếu trống, nhân sự vẫn được import nhưng chưa tạo tài khoản |
| `seed_pin` | Không | Nếu điền phải đúng 5 số cuối mobile; **không lưu vào CSDL** |
| `role` | Không | Mặc định `employee`; nhiều vai trò cách nhau bởi `\|` |
| `group` | Không | Nhóm / tổ / ca làm việc, tối đa 120 ký tự |

Role nhận: `employee`, `supervisor`, `manager`, `director`, `council`, `hr`, `ceo`, `admin` (hoặc mã `WORKER`, `COUNCIL`…).

Phạm vi được suy ra tự động: `council/hr/director/ceo/admin` → `ALL`; các vai trò còn lại → `dept_code`. Có thể chỉnh tay ở tab **Người dùng & vai trò** sau khi import.

- Excel: **File → Save As → CSV UTF-8**. Giữ `emp_id`, `mobile_num`, `seed_pin` ở định dạng Text để không mất số 0 đầu. Hỗ trợ dấu phẩy, chấm phẩy hoặc tab; có BOM.
- **Import theo dòng**: dòng hợp lệ được ghi; dòng lỗi bị bỏ qua và liệt kê. Chỉ lỗi file/header mới chặn toàn bộ.
- Với nhân viên đã tồn tại, ô trống `job_title`, `group`, `e_mail`, `report_to` giữ giá trị cũ; muốn xóa thì sửa trong **Master data → Nhân viên**.
- Import 1.000 dòng không băm 1.000 PIN, nên chạy nhanh. Tài khoản mới có `password_hash='pending'`.

### 3.3 Đăng nhập lần đầu — mã kích hoạt riêng

Tên đăng nhập: `emp_id`. Tài khoản mới ở trạng thái `pending`. Người có quyền quản trị trên máy chủ thực hiện:

```powershell
.\bin\kaizen.exe issue-invite P81-0001
```

Mã được sinh ngẫu nhiên, có thời hạn 30 phút, không lưu dạng rõ trong DB; giao riêng sau khi xác minh danh tính. Nhân viên đặt mật khẩu mới (tối thiểu 10 ký tự) và thiết lập MFA TOTP. Mã dựa trên số điện thoại không còn hợp lệ.

### 3.4 Chế độ vận hành (demo đã tắt)

- Migration 007 đặt `demo_mode=0`; **không còn seed dữ liệu demo**.
- Hồ sơ demo cũ (`is_demo=1`) vẫn lưu để đối chiếu nhưng bị ẩn khỏi mọi truy vấn/dashboard/export.
- Không còn role switcher hay dữ liệu dùng chung: mỗi người đăng nhập tài khoản riêng. `WORKER` chỉ thấy hồ sơ mình nộp/đồng tác giả; `MANAGER`/`SUPERVISOR` theo đơn vị; `COUNCIL`/`HRD`/`CEO_AUDIT`/`ADMIN` theo quyền và phạm vi.

### 3.5 Import trực tiếp từ thư mục `data/uploads`

Đặt file CSV vào:

```text
C:\Users\User\App\01-service\initiative-app\v3\data\uploads\staff.csv
```

Sau đó chạy:

```powershell
cd C:\Users\User\App\01-service\initiative-app\v3
.\import-staff.cmd
# hoặc chỉ định file / reset PIN:
.\import-staff.cmd "C:\...\data\uploads\P81.csv"
.\import-staff.cmd --reset-pin
```

Hoặc dùng trực tiếp:

```powershell
.\bin\kaizen.exe import-staff
# hoặc chỉ định file:
.\bin\kaizen.exe import-staff "C:\...\data\uploads\P81.csv"
# đặt lại trạng thái chờ kích hoạt cho tài khoản đã tồn tại:
.\bin\kaizen.exe import-staff --reset-pin
```

- Không cần đăng nhập ADMIN.
- Tự tìm `staff.csv`, `personnel.csv`, `employees.csv`, `nhan_su.csv` hoặc file `.csv` duy nhất trong thư mục uploads (không quét thư mục con).
- In ra số dòng, đơn vị, tài khoản tạo/cập nhật, số tài khoản bị bỏ qua vì thiếu mobile và danh sách lỗi.
- Nên dừng server trước khi chạy CLI; nếu đang chạy, SQLite WAL vẫn cho phép nhưng không khuyến nghị.

### 3.6 Hội đồng ĐGKT

- Thêm một dòng `role=council` trong CSV, hoặc dùng form **Tạo tài khoản Hội đồng** (chọn nhân viên).
- Tài khoản mới nhận mã ngẫu nhiên do quản trị cấp qua CLI, phạm vi `ALL`, không có quyền `track:confidential:read`.
- Tài khoản đã tồn tại chỉ được thêm vai trò `COUNCIL` và mở phạm vi `ALL`; mật khẩu hiện tại được giữ nguyên.

### 3.7 Xuất danh sách

`Chinfon_Staff_YYYYMMDD.csv` gồm emp_id, name, job_title, dept, report_to, email, mobile, role, group, username, scope, account_status, mfa. **Không xuất PIN/hash**.

## 4. Vai trò & phạm vi dữ liệu

Quyền được cộng dồn từ nhiều vai trò. Phạm vi (`dept_scope`) là `ALL` hoặc mã đơn vị.

| Vai trò | Quyền mặc định | Phạm vi gợi ý |
|---|---|---|
| `CEO_AUDIT` | tạo, xác nhận, thẩm định, phê duyệt cuối, giải mật Luồng 3, chi trả | `ALL` |
| `COUNCIL` | tạo, xác nhận, thẩm định, phê duyệt cuối | `ALL` |
| `DIRECTOR` | như trên | `ALL` |
| `MANAGER` | tạo, xác nhận, thẩm định | đơn vị |
| `SUPERVISOR` | tạo, xác nhận hiện trường | đơn vị |
| `HRD` | tạo, chi trả & xuất bảng lương | `ALL` |
| `ADMIN` | toàn bộ 7 quyền | `ALL` |
| `WORKER` | tạo hồ sơ | đơn vị / chỉ hồ sơ của mình |

Các quyền: `idea:create`, `idea:screen:dept`, `idea:review:roi`, `idea:approve:final`, `track:confidential:read`, `hr:payroll:export`, `rbac:admin`.

**Cô lập dữ liệu** được áp dụng trong SQL, không chỉ ẩn nút:

- `WORKER`: chỉ thấy hồ sơ mình nộp hoặc mình là đồng tác giả.
- Vai trò có phạm vi đơn vị: chỉ thấy hồ sơ thuộc đơn vị đó (trừ hồ sơ của chính mình).
- `CONFIDENTIAL`: chỉ `CEO_AUDIT`/`ADMIN` thấy toàn bộ; người nộp vẫn theo dõi được hồ sơ của mình; cấp trung gian nhận **404** để không suy đoán được sự tồn tại.
- Mọi lần mở/in hồ sơ Luồng 3 đều ghi audit.

## 5. Luồng nghiệp vụ

```text
DRAFT --submit--> SCREEN --xác nhận--> REVIEW --thẩm định--> APPROVE
      --phê duyệt--> IMPLEMENT --xác nhận triển khai & chi trả--> APPLIED
```

- **Luồng 1 — FAST48**: SLA 48 giờ; sự cố, đại tu, Kaizen < 20 Tr/năm.
- **Luồng 2 — STANDARD**: SLA 5 ngày làm việc ở bước xác nhận; nghiệm thu 30 ngày; chốt sổ ngày 25.
- **Luồng 3 — CONFIDENTIAL**: bỏ qua `SCREEN`, nộp thẳng `REVIEW`, tự động ẩn danh, chỉ TGĐ & Kiểm toán Nội bộ.

Mỗi bước có thể `APPROVE`, `REJECT` (lưu hồ sơ, không xóa) hoặc `NEED_INFO` (giữ nguyên bước). Người nộp không được tự thẩm định/phê duyệt hồ sơ của mình, trừ `ADMIN`/`CEO_AUDIT`.

**Hai chiều trạng thái tách biệt**:

- quy trình: `stage_code` + `record_status` (OPEN/REJECTED/CLOSED);
- thực tế: `implementation_status_code` (IDEA/PILOT/IMPLEMENTED).

## 6. Khung thưởng (Mục 5.1)

| Mức | Net Benefit/năm | Thưởng cơ sở |
|---|---|---|
| T1 | ≤ 0 | 300.000 |
| T2 | < 20 Tr | 1.000.000 |
| T3 | 20 – < 50 Tr | 2.500.000 |
| T4 | 50 – < 150 Tr | 5.000.000 |
| T5 | 150 – < 300 Tr | 10.000.000 |
| T6 | 300 – < 500 Tr | 20.000.000 |
| T7 | ≥ 500 Tr | 6% Net, tối đa 100.000.000 |

- `Net = max(0, A − B)`; tất cả tiền là **số nguyên VND**.
- Thuộc Phụ lục 03: nhân **1.2x**, vẫn trần 100.000.000.
- Mục 5.2 (phi tài chính): 4 mức 1 / 3 / 8 / 25 triệu, không nhân 1.2x.
- `T1` là thưởng khuyến khích ý tưởng, tặng đứt, không trừ vào thưởng nghiệm thu.
- Thanh toán chia theo `initiative_participant`; người nộp nhận phần còn lại. Tổng tỷ lệ đồng tác giả ≤ 100%.
- HRD xuất CSV có BOM UTF-8 và chống CSV injection.

## 7. Bảo mật đã có

- Mật khẩu PBKDF2-SHA256, 210.000 vòng, salt ngẫu nhiên; tối thiểu 10 ký tự.
- **MFA TOTP bắt buộc cho mọi tài khoản**: QR + mã 6 số, chống dùng lại mã, secret mã hóa AES-256-GCM bằng khóa ngoài DB.
- Phiên đăng nhập chỉ được coi là đầy đủ sau khi xác thực MFA; phiên trước MFA chỉ sống 10 phút và chỉ vào được trang MFA.
- Backoff theo tài khoản: sau 5 lần sai, khóa 1, 2, 4, 8 rồi tối đa 15 phút; kèm giới hạn theo IP + username.
- Reset MFA: ADMIN khác trong trang Người dùng, hoặc CLI `reset-mfa` khi mất thiết bị/không còn ADMIN đăng nhập được. Chi tiết: [`docs/MFA.md`](docs/MFA.md).
- Mã ngẫu nhiên được lưu dưới dạng hash và có thời hạn; tài khoản mới ở trạng thái `pending` và bắt buộc đổi mật khẩu.
- Demo mode được lưu trong `app_setting`; hồ sơ demo có `is_demo` và bị ẩn ngay trong SQL khi tắt demo.
- Session cookie `HttpOnly`, `SameSite=Strict`, hết hạn 12 giờ; token lưu dạng SHA-256.
- CSRF cho mọi POST; token ổn định theo người dùng (không mất hiệu lực khi mở tab mới/đăng nhập lại), login/setup dùng double-submit cookie dùng lại giữa các tab.
- Trang HTML gửi `Cache-Control: no-store` để tránh form/token cũ bị cache; cookie phiên có tiền tố theo đường dẫn CSDL để hai instance khác DB không ghi đè cookie của nhau.
- CSP `default-src 'self'`, không inline script/style; `X-Frame-Options: DENY`, `nosniff`.
- Upload giới hạn 10 MB, whitelist phần mở rộng, lưu ngoài DB, tải xuống qua handler có kiểm tra quyền và `Content-Disposition`.
- SHA-256 của tệp lưu trong CSDL để đối chiếu.
- Audit log + lịch sử trạng thái **append-only** bằng trigger SQLite; mã hồ sơ bất biến.
- Không xóa cứng hồ sơ đã nộp.

## 8. Kiểm thử

```powershell
go test ./... -count=1
```

Bao phủ: khung 7 mức & biên, 1.2x, phi tài chính, Net clamp, RBAC/scope, Luồng 3, SQL visibility, trigger bất biến, auth gate, static embed, và lỗi deadlock một-connection SQLite.

## 9. Cấu trúc

```text
v3/
  cmd/kaizen/main.go
  internal/security/           PBKDF2
  internal/mfa/               TOTP, AES-GCM secret, QR
  internal/app/
    app.go                     embed, routes, CSP, template funcs
    auth.go                    session, CSRF, setup/login/password
    handlers_mfa.go            TOTP setup/verify, backoff, admin/CLI reset
    rbac.go                    quyền, scope, visibility SQL
    reward.go                  khung 7 mức, 1.2x, phi tài chính
    workflow.go                transition transaction + history
    store.go                   query, master data, ledger
    handlers_*.go
    staff.go                  CSV import/export, council provisioning
    seed.go
    migrations/001_init.sql
    migrations/002_hardening.sql
    migrations/003_staff.sql
    migrations/004_csrf.sql
    migrations/005_mfa.sql
    web/templates/*.html
    web/static/app.css|app.js
```

## 10. Điểm mạnh lấy từ v1 và v2

Xem [`docs/MIGRATION_V1_V2.md`](docs/MIGRATION_V1_V2.md). Tóm tắt:

- Giữ từ v1: xác thực thật, CSRF, master data có mã, workflow history, tệp + SHA-256, RBAC trong handler, test.
- Giữ từ v2: PN-B0000-02, 3 track, 5 bước, khung 7 mức, 1.2x, Mục 5.2, dashboard/KPI/kanban, audit và ma trận RBAC.
- Không bê nguyên: role switcher bằng header `X-Cem-Role` và dữ liệu trong bộ nhớ của v2; hard-delete hồ sơ; CDN Tailwind/Lucide (không phù hợp môi trường offline/nhà máy).

## 11. Sao lưu & triển khai

Sao lưu `data/kaizen.db` + `data/uploads/` + `data/mfa.key` khi ứng dụng dừng, hoặc dùng `sqlite3 .backup` khi chạy. Nếu mất `mfa.key`, không thể giải mã TOTP secret; phải reset MFA từng tài khoản. Trước khi mở ra Internet, đọc [`docs/PUBLIC_DEPLOYMENT_GATE.md`](docs/PUBLIC_DEPLOYMENT_GATE.md).

Ứng dụng chưa có email/SSO/AD, chưa có rate-limit đăng nhập tập trung, chưa có PostgreSQL. Chưa có QR/offline queue và chưa có cổng mobile riêng: biểu mẫu hiện tại responsive và dùng được trên điện thoại. Đây là các bước mở rộng có chủ đích, không phải thiếu sót ngẫu nhiên.
