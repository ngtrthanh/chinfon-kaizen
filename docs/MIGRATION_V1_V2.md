# Migration notes — v1 & v2 → v3

## 1. Bảng đối chiếu

| Năng lực | v1 | v2 | v3 |
|---|---|---|---|
| Kiến trúc | Go + SQLite + server-rendered | Go + SQLite + SPA Tailwind CDN | Go + SQLite + server-rendered, asset nhúng |
| Xác thực | PBKDF2, session, CSRF | Header `X-Cem-Role` (demo) | PBKDF2, session, CSRF, cookie double-submit cho login/setup |
| Phân quyền | Role cộng dồn, handler check | 8 role + ma trận trên UI | 8 role + permission DB + scope guard trong SQL |
| Cô lập dữ liệu | Employee/Manager/HRD | `dept_scope` phía JS | SQL clause + kiểm tra `CanView/CanAct` |
| Luồng 3 bảo mật | Không có | Có, nhưng ẩn phía client | Có, ẩn trong SQL; người nộp vẫn theo dõi được hồ sơ mình |
| Workflow | 10 status tuyến tính | 5 stage dùng chung | 5 stage + `record_status`; transaction + optimistic lock |
| Hai chiều trạng thái | Có (`implementation_status` tách workflow) | Chỉ stage | Có (`stage_code` + `implementation_status_code`) |
| Khung thưởng | 8 mức cũ, có vùng chồng lấn | 7 mức PN-B0000-02 + 1.2x | 7 mức + 1.2x + Mục 5.2, tính bằng integer VND, có test |
| Mã hồ sơ | `SI-YYYY-NNNNN`, bất biến | Mã theo track, sinh demo | `SI-YYYY-NNNNN`, bất biến, trigger DB |
| Lịch sử | `initiative_status_history` | Không lưu thật | `initiative_history` append-only, trigger DB |
| Audit | Không có | Mảng JS + audit demo | `audit_log` append-only, ghi cả 403/giải mật/xuất lương |
| Tệp đính kèm | Metadata + SHA-256 | Chỉ URL ảnh demo | Metadata + SHA-256 + whitelist + tải qua handler |
| Master data | Có | Một phần, hard-code | Có; phòng ban/nhân viên/RBAC sửa được |
| Báo cáo | Không | Demo toast | Sổ cái thưởng + CSV HRD thật |
| Test | Có template/password test | Không | Reward, RBAC/scope, SQL visibility, trigger, auth gate, deadlock |
| Giao diện | Bảng tối giản | Dashboard/kanban đẹp nhưng demo | Dashboard/kanban/OPL theo ngôn ngữ v2, không CDN |
| Nhập nhân sự | Master data tay | Không có | CSV template, import/export 1.000 dòng, PIN khởi tạo, role cộng dồn |
| Demo mode | Không có | Toàn bộ là dữ liệu JS | `app_setting.demo_mode=0` mặc định; không seed demo, hồ sơ demo cũ bị ẩn |
| Hội đồng | Role COUNCIL | Role COUNCIL | Import `role=council` hoặc form cấp tài khoản Hội đồng |

## 2. Điểm mạnh được giữ nguyên

### Từ v1

1. **Xác thực thật**: không có tài khoản demo, mật khẩu băm, session hash, CSRF.
2. **Master data có mã**: phòng ban, nhân viên, trạng thái, tài liệu, mức thưởng.
3. **RBAC trong handler**: không chỉ ẩn UI; mọi thao tác kiểm tra quyền + phạm vi.
4. **Workflow history**: mọi chuyển bước ghi vết; không xóa cứng.
5. **Tệp đính kèm**: metadata + SHA-256 + storage key.
6. **Migration có phiên bản** và test.

### Từ v2

1. **PN-B0000-02**: 3 luồng tốc độ, quyền hạn theo chức danh, Luồng 3.
2. **Khung 7 mức + Phụ lục 03 1.2x + Mục 5.2**.
3. **Dashboard điều hành**: KPI, 7 lĩnh vực, bảng vàng, cảnh báo SLA.
4. **Kanban 5 bước**, OPL, sổ cái HRD, ma trận RBAC, audit.
5. **Ngôn ngữ hình ảnh**: sidebar công nghiệp, cam an toàn, thẻ KPI.

## 3. Điểm cố ý không bê nguyên

| v2 | Lý do | v3 thay thế |
|---|---|---|
| `X-Cem-Role` header, mặc định CEO | Bất kỳ ai cũng đổi được role | Session + DB role; role chỉ ADMIN sửa |
| Dữ liệu `ideasData` trong JS | Mất khi refresh, không audit | SQLite + transaction |
| Xóa hồ sơ bằng `filter` | Mất dấu vết | `record_status=REJECTED`, giữ history |
| Tailwind/Lucide CDN | Không chạy offline trong nhà máy | CSS/icon nhúng, CSP `self` |
| Nút "đổi quà" 850 điểm | Không có quy định nguồn trong PN-B0000-02 | Bỏ; chỉ giữ sổ cái thưởng thật |
| Xuất DOCX bằng Blob HTML | Nội dung tĩnh, không theo hồ sơ | In Phụ lục 02 từ dữ liệu thật + CSV HRD |
| Simulator mobile + offline localStorage | Chỉ là demo, không đồng bộ/không audit | Form responsive thật; offline/QR là hạng mục mở rộng |
| SQLite một connection + query lồng nhau | Deadlock nếu mở rows rồi query tiếp | Đóng rows trước query kế tiếp; có test hồi quy `masterUsers` |

## 4. Quyết định nghiệp vụ cần HRD xác nhận

1. **Người nộp Luồng 3** được xem hồ sơ của chính mình (không xem hồ sơ mật khác). Nếu HRD muốn ẩn tuyệt đối, đổi `CanView` và `VisibilityClause`.
2. **Phạm vi MANAGER**: v3 mặc định theo đơn vị (kế thừa v1). v2 để `ALL`; ADMIN có thể đổi `dept_scope` thành `ALL` nếu cần.
3. **Phê duyệt cuối** được phép điều chỉnh số tiền trong trần 100 triệu; lý do điều chỉnh nên ghi trong comment.
4. **T1 300k**: v3 nhân 1.2x nếu thuộc Phụ lục 03 (theo code v2). Nếu coi đây là khoản tặng đứt cố định, cần sửa `ComputeFinancialReward`.
5. **Trần 100 triệu** áp cho cả override; mức 5.2 tối đa 25 triệu trong cấu hình hiện tại.

## 5. Quyết định onboarding nhân sự (v3.1)

1. **CSV thay XLSX**: chọn CSV UTF-8 vì Excel mở/save trực tiếp, không cần thư viện XLSX bên thứ ba, không có rủi ro đọc file zip/XML lỗi. Template có BOM để Excel hiển thị đúng tiếng Việt.
2. **PIN = 5 số cuối mobile, không lưu**: giảm rủi ro lộ PIN trong CSDL và giảm thời gian import 1.000 người (không băm 1.000 PIN). Bắt buộc đổi mật khẩu lần đầu + rate limit 10 lần/10 phút.
3. **Import theo dòng**: dòng hợp lệ vẫn được ghi; dòng lỗi bị bỏ qua và báo cáo. Chỉ lỗi file/header mới rollback toàn bộ. Thiếu mobile được chấp nhận: nhân sự vẫn import, tài khoản bỏ qua đến khi bổ sung mobile.
4. **Import là authoritative cho role/scope của các dòng có mặt**: role cũ bị thay thế; tài khoản ADMIN cuối cùng được bảo vệ để không tự khóa hệ thống.
5. **Demo mode tắt mặc định**: migration 007 đặt `demo_mode=0`; không seed dữ liệu mẫu. Hồ sơ demo cũ vẫn lưu nhưng bị ẩn.
6. **Nếu HR muốn XLSX native**: cần thêm thư viện như excelize và một lớp kiểm thử file; chưa đưa vào v3.1 để giữ single-binary nhẹ.
