-- Chinfon Kaizen 360 / PN-B0000-02 (2026)
-- Canonical initiative record + controlled master data + immutable history.
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS md_department (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'PLANT' CHECK(kind IN ('PLANT','SUPPORT','HRD','BOD','BRANCH')),
    sort_order INTEGER NOT NULL DEFAULT 100,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_employee (
    employee_no TEXT PRIMARY KEY,
    full_name TEXT NOT NULL,
    department_code TEXT NOT NULL REFERENCES md_department(code),
    position_title TEXT,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_track (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    sla_hours INTEGER NOT NULL,
    description TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS md_stage (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS md_implementation_status (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS md_document_type (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS md_reward_tier (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    basis TEXT NOT NULL CHECK(basis IN ('FINANCIAL','NONFINANCIAL')),
    min_net_vnd INTEGER,
    max_net_vnd INTEGER,
    base_amount_vnd INTEGER NOT NULL CHECK(base_amount_vnd >= 0),
    rule_note TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS rbac_role (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 100
);

CREATE TABLE IF NOT EXISTS rbac_permission (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 100
);

CREATE TABLE IF NOT EXISTS rbac_role_permission (
    role_code TEXT NOT NULL REFERENCES rbac_role(code) ON DELETE CASCADE,
    permission_code TEXT NOT NULL REFERENCES rbac_permission(code) ON DELETE CASCADE,
    PRIMARY KEY(role_code, permission_code)
);

CREATE TABLE IF NOT EXISTS app_user (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    employee_no TEXT NOT NULL UNIQUE REFERENCES md_employee(employee_no),
    dept_scope TEXT NOT NULL DEFAULT 'ALL',
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE IF NOT EXISTS app_user_role (
    user_id INTEGER NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    role_code TEXT NOT NULL REFERENCES rbac_role(code),
    PRIMARY KEY(user_id, role_code)
);

CREATE TABLE IF NOT EXISTS app_session (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    csrf_token TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE IF NOT EXISTS initiative_sequence (
    year INTEGER PRIMARY KEY,
    value INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS initiative (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    track_code TEXT NOT NULL REFERENCES md_track(code),
    stage_code TEXT NOT NULL REFERENCES md_stage(code),
    record_status TEXT NOT NULL DEFAULT 'OPEN' CHECK(record_status IN ('OPEN','REJECTED','CLOSED')),
    title TEXT NOT NULL CHECK(length(title) BETWEEN 5 AND 240),
    department_code TEXT NOT NULL REFERENCES md_department(code),
    submitter_employee_no TEXT NOT NULL REFERENCES md_employee(employee_no),
    equipment_code TEXT NOT NULL DEFAULT '',
    problem_summary TEXT NOT NULL CHECK(length(problem_summary) <= 500),
    problem_detail TEXT NOT NULL,
    solution_summary TEXT NOT NULL CHECK(length(solution_summary) <= 500),
    solution_detail TEXT NOT NULL,
    benefit_summary TEXT NOT NULL DEFAULT '',
    implementation_status_code TEXT NOT NULL DEFAULT 'IDEA' REFERENCES md_implementation_status(code),
    gross_value_vnd INTEGER NOT NULL DEFAULT 0 CHECK(gross_value_vnd >= 0),
    invest_cost_vnd INTEGER NOT NULL DEFAULT 0 CHECK(invest_cost_vnd >= 0),
    net_benefit_vnd INTEGER NOT NULL DEFAULT 0 CHECK(net_benefit_vnd >= 0),
    annex03 INTEGER NOT NULL DEFAULT 0 CHECK(annex03 IN (0,1)),
    reward_basis TEXT NOT NULL DEFAULT 'FINANCIAL' CHECK(reward_basis IN ('FINANCIAL','NONFINANCIAL')),
    nonfinancial_level INTEGER NOT NULL DEFAULT 0 CHECK(nonfinancial_level BETWEEN 0 AND 4),
    reward_tier_code TEXT REFERENCES md_reward_tier(code),
    base_reward_vnd INTEGER NOT NULL DEFAULT 0 CHECK(base_reward_vnd >= 0),
    reward_amount_vnd INTEGER NOT NULL DEFAULT 0 CHECK(reward_amount_vnd >= 0),
    seed_granted INTEGER NOT NULL DEFAULT 0 CHECK(seed_granted IN (0,1)),
    anonymous INTEGER NOT NULL DEFAULT 0 CHECK(anonymous IN (0,1)),
    split_ratio TEXT NOT NULL DEFAULT '',
    due_at TEXT,
    submitted_at TEXT,
    paid_at TEXT,
    paid_by TEXT REFERENCES md_employee(employee_no),
    reject_reason TEXT,
    created_by TEXT NOT NULL REFERENCES md_employee(employee_no),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE IF NOT EXISTS initiative_participant (
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE CASCADE,
    employee_no TEXT NOT NULL REFERENCES md_employee(employee_no),
    share_percent REAL NOT NULL DEFAULT 0 CHECK(share_percent >= 0 AND share_percent <= 100),
    role_note TEXT NOT NULL DEFAULT 'Đồng tác giả',
    PRIMARY KEY(initiative_id, employee_no)
);

CREATE TABLE IF NOT EXISTS initiative_review (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE CASCADE,
    stage_code TEXT NOT NULL REFERENCES md_stage(code),
    reviewer_employee_no TEXT NOT NULL REFERENCES md_employee(employee_no),
    decision TEXT NOT NULL CHECK(decision IN ('APPROVE','REJECT','NEED_INFO')),
    comment TEXT NOT NULL DEFAULT '',
    assessed_gross_vnd INTEGER,
    assessed_invest_vnd INTEGER,
    assessed_net_vnd INTEGER,
    reward_tier_code TEXT REFERENCES md_reward_tier(code),
    reward_amount_vnd INTEGER,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE IF NOT EXISTS initiative_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE CASCADE,
    from_stage_code TEXT REFERENCES md_stage(code),
    to_stage_code TEXT NOT NULL REFERENCES md_stage(code),
    action_code TEXT NOT NULL,
    changed_by TEXT NOT NULL REFERENCES md_employee(employee_no),
    note TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE IF NOT EXISTS initiative_attachment (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE CASCADE,
    document_type_code TEXT NOT NULL REFERENCES md_document_type(code),
    original_filename TEXT NOT NULL,
    storage_key TEXT NOT NULL UNIQUE,
    mime_type TEXT,
    size_bytes INTEGER NOT NULL CHECK(size_bytes >= 0),
    sha256_hex TEXT NOT NULL,
    uploaded_by TEXT NOT NULL REFERENCES md_employee(employee_no),
    uploaded_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    actor TEXT NOT NULL,
    action_code TEXT NOT NULL,
    target TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_initiative_department ON initiative(department_code);
CREATE INDEX IF NOT EXISTS idx_initiative_submitter ON initiative(submitter_employee_no);
CREATE INDEX IF NOT EXISTS idx_initiative_stage ON initiative(stage_code, record_status);
CREATE INDEX IF NOT EXISTS idx_initiative_track ON initiative(track_code);
CREATE INDEX IF NOT EXISTS idx_review_initiative ON initiative_review(initiative_id, id);
CREATE INDEX IF NOT EXISTS idx_history_initiative ON initiative_history(initiative_id, id);
CREATE INDEX IF NOT EXISTS idx_attachment_initiative ON initiative_attachment(initiative_id);
CREATE INDEX IF NOT EXISTS idx_audit_at ON audit_log(at);

-- ---------------------------------------------------------------------------
-- Controlled master data
-- ---------------------------------------------------------------------------
INSERT OR IGNORE INTO md_department(code, name, kind, sort_order) VALUES
('BOD',       'Ban Giám đốc & Kiểm toán Nội bộ',              'BOD',      10),
('MINE',      '1. Mỏ đá & Đập đá',                            'PLANT',    20),
('KILN',      '2. Lò nung, Nghiền thô & WHR',                 'PLANT',    30),
('GRIND',     '3. Nghiền XM, Đóng bao & Hiệp Phước',          'PLANT',    40),
('LOGISTICS', '4. Kho vận, Cảng & Trạm trộn',                 'PLANT',    50),
('QHSE',      '5. Khối QHSE & Môi trường',                    'SUPPORT',  60),
('SALES',     '6. Khối Kinh doanh & Xuất khẩu',               'SUPPORT',  70),
('SUPPORT',   '7. Khối Phụ trợ & Chuyển đổi số',              'SUPPORT',  80),
('HRD',       'Phòng Nhân lực',                                'HRD',      90);

INSERT OR IGNORE INTO md_track(code, name, sla_hours, description, sort_order) VALUES
('FAST48',     'Luồng 1 — Thưởng nóng 48h', 48,
 'Ngăn sự cố, vượt tiến độ đại tu, Kaizen < 20 triệu/năm hoặc thưởng khuyến khích ý tưởng.', 10),
('STANDARD',   'Luồng 2 — Tiêu chuẩn', 120,
 'Làm lợi ròng ≥ 20 triệu/năm, nghiệm thu tối thiểu 30 ngày, chốt sổ ngày 25.', 20),
('CONFIDENTIAL','Luồng 3 — Bảo mật tố giác', 24,
 'Chuyển thẳng Tổng Giám đốc & Kiểm toán Nội bộ, bỏ qua Trưởng phòng/Giám đốc khối.', 30);

INSERT OR IGNORE INTO md_stage(code, name, sort_order) VALUES
('DRAFT',     'Nháp',                                10),
('SCREEN',    'Xác nhận hiện trường',                20),
('REVIEW',    'Thẩm định & nghiệm thu',              30),
('APPROVE',   'Phê duyệt quyết định thưởng',         40),
('IMPLEMENT', 'Triển khai & xác nhận kết quả',       50),
('APPLIED',   'Đã chi trả & vinh danh',              60);

INSERT OR IGNORE INTO md_implementation_status(code, name, sort_order) VALUES
('IDEA',        'Ý tưởng / chưa triển khai', 10),
('PILOT',       'Đang thử nghiệm',           20),
('IMPLEMENTED', 'Đã áp dụng thực tế',         30);

INSERT OR IGNORE INTO md_document_type(code, name) VALUES
('PHOTO',       'Hình ảnh hiện trường'),
('EVIDENCE',    'Bằng chứng / log dữ liệu'),
('CALCULATION', 'Bảng tính lợi ích'),
('DRAWING',     'Bản vẽ / sơ đồ'),
('OTHER',       'Tài liệu khác');

INSERT OR IGNORE INTO md_reward_tier(code, name, basis, min_net_vnd, max_net_vnd, base_amount_vnd, rule_note, sort_order) VALUES
('T1_IDEA', 'Mức 1 — Thưởng khuyến khích ý tưởng', 'FINANCIAL', NULL, 0, 300000,
 'Tặng đứt khi ý tưởng được duyệt thử nghiệm; không trừ vào thưởng nghiệm thu.', 10),
('T2', 'Mức 2 — Khen ngợi Cấp 1', 'FINANCIAL', 0, 19999999, 1000000,
 'Làm lợi ròng < 20 triệu VNĐ/năm + thư khen.', 20),
('T3', 'Mức 3 — Khen ngợi Cấp 2', 'FINANCIAL', 20000000, 49999999, 2500000,
 'Từ 20 đến < 50 triệu VNĐ/năm + điểm thi đua.', 30),
('T4', 'Mức 4 — Ghi công Nhỏ Cấp 1', 'FINANCIAL', 50000000, 149999999, 5000000,
 'Từ 50 đến < 150 triệu VNĐ/năm + giấy khen.', 40),
('T5', 'Mức 5 — Ghi công Nhỏ Cấp 2', 'FINANCIAL', 150000000, 299999999, 10000000,
 'Từ 150 đến < 300 triệu VNĐ/năm + vinh danh.', 50),
('T6', 'Mức 6 — Ghi công Lớn', 'FINANCIAL', 300000000, 499999999, 20000000,
 'Từ 300 đến < 500 triệu VNĐ/năm + kỷ niệm chương.', 60),
('T7', 'Mức 7 — Giải thưởng Đột phá', 'FINANCIAL', 500000000, NULL, 0,
 '5%–10% lợi ích ròng năm đầu, tính chuẩn 6%, tối đa 100 triệu VNĐ.', 70),
('NF1', 'Mục 5.2 — Mức 1 (cục bộ)', 'NONFINANCIAL', NULL, NULL, 1000000,
 'Rò rỉ dầu/khí, kẹt con lăn, ảnh hưởng cục bộ.', 80),
('NF2', 'Mục 5.2 — Mức 2 (dây chuyền)', 'NONFINANCIAL', NULL, NULL, 3000000,
 'Ngăn dừng lò/nghiền < 24h, vượt tiến độ đại tu.', 90),
('NF3', 'Mục 5.2 — Mức 3 (nghiêm trọng)', 'NONFINANCIAL', NULL, NULL, 8000000,
 'Ngăn cháy nổ trạm điện/kho than, gian lận vật tư.', 100),
('NF4', 'Mục 5.2 — Mức 4 (đặc biệt)', 'NONFINANCIAL', NULL, NULL, 25000000,
 'Phá đường dây tiêu cực lớn, cứu hộ; TGĐ quyết định.', 110);

INSERT OR IGNORE INTO rbac_role(code, name, description, sort_order) VALUES
('CEO_AUDIT',  'Tổng Giám đốc & Kiểm toán Nội bộ', 'Toàn công ty + giải mật Luồng 3.', 10),
('COUNCIL',    'Hội đồng Đánh giá Khen thưởng',     'Họp định kỳ ngày 26–30 hàng tháng.', 20),
('DIRECTOR',   'Giám đốc khối (Director)',          'Phê duyệt cấp khối & xác nhận Phụ lục 03.', 30),
('MANAGER',    'Trưởng phòng (Manager)',            'Thẩm định thực tế trong phạm vi đơn vị.', 40),
('SUPERVISOR', 'Quản đốc (Supervisor)',             'Xác nhận hiện trường trong phạm vi đơn vị.', 50),
('HRD',        'Phòng Nhân lực (HRD)',              'Tổng hợp, chi trả và quản trị cổng.', 60),
('ADMIN',      'Quản trị hệ thống',                 'Toàn quyền cấu hình & RBAC.', 70),
('WORKER',     'Nhân viên / Công nhân',             'Nộp Phụ lục 01 và theo dõi hồ sơ của mình.', 80);

INSERT OR IGNORE INTO rbac_permission(code, name, description, sort_order) VALUES
('idea:create',              'Nộp Phụ lục 01',                  'Tạo và nộp hồ sơ sáng kiến.', 10),
('idea:screen:dept',         'Xác nhận hiện trường',             'Xác nhận thực tế theo phạm vi đơn vị.', 20),
('idea:review:roi',          'Thẩm định Phụ lục 02',             'Tính Net Benefit và khung thưởng.', 30),
('idea:approve:final',       'Phê duyệt cuối cùng',              'Quyết định thưởng và hệ số 1.2x.', 40),
('track:confidential:read',  'Giải mật Luồng 3',                 'Chỉ TGĐ & Kiểm toán Nội bộ.', 50),
('hr:payroll:export',        'Chi trả & xuất bảng lương',        'Đánh dấu chi trả, xuất CSV HRD.', 60),
('rbac:admin',               'Quản trị hệ thống & RBAC',         'Cấu hình quyền, người dùng, master data.', 70);

INSERT OR IGNORE INTO rbac_role_permission(role_code, permission_code) VALUES
('CEO_AUDIT','idea:create'),('CEO_AUDIT','idea:screen:dept'),('CEO_AUDIT','idea:review:roi'),
('CEO_AUDIT','idea:approve:final'),('CEO_AUDIT','track:confidential:read'),('CEO_AUDIT','hr:payroll:export'),
('COUNCIL','idea:create'),('COUNCIL','idea:screen:dept'),('COUNCIL','idea:review:roi'),('COUNCIL','idea:approve:final'),
('DIRECTOR','idea:create'),('DIRECTOR','idea:screen:dept'),('DIRECTOR','idea:review:roi'),('DIRECTOR','idea:approve:final'),
('MANAGER','idea:create'),('MANAGER','idea:screen:dept'),('MANAGER','idea:review:roi'),
('SUPERVISOR','idea:create'),('SUPERVISOR','idea:screen:dept'),
('HRD','idea:create'),('HRD','hr:payroll:export'),
('ADMIN','idea:create'),('ADMIN','idea:screen:dept'),('ADMIN','idea:review:roi'),('ADMIN','idea:approve:final'),
('ADMIN','track:confidential:read'),('ADMIN','hr:payroll:export'),('ADMIN','rbac:admin'),
('WORKER','idea:create');

-- Demo employees (no demo passwords; /setup creates the first real ADMIN account).
INSERT OR IGNORE INTO md_employee(employee_no, full_name, department_code, position_title) VALUES
('CF-0001','Lai Lai',                 'BOD',       'Tổng Giám đốc & Kiểm toán Nội bộ'),
('CF-0005','Vũ Hữu Kháng',            'BOD',       'Giám đốc khối'),
('CF-0010','Trần Thị Hồng',           'BOD',       'Chủ tịch Hội đồng ĐGKT'),
('CF-0018','Nguyễn Tuấn Anh',         'KILN',      'Trưởng phòng Kỹ thuật & Vận hành'),
('CF-0118','Vũ Quang Huy',            'KILN',      'Quản đốc Xưởng Lò & Nghiền'),
('CF-0088','Hồ Thị Thảo',             'HRD',       'Trưởng phòng Nhân lực'),
('CF-0412','Trần Văn Hùng',           'KILN',      'Tổ trưởng Tổ Cơ khí Lò'),
('CF-0519','Lê Minh Khôi',            'GRIND',     'Kỹ sư Công nghệ Hiệp Phước'),
('CF-0611','Nguyễn Thu Hà',           'SUPPORT',   'Chuyên viên Kế toán / MIS'),
('CF-0711','Đỗ Minh Tuấn',            'QHSE',      'Chuyên viên QHSE'),
('CF-0811','Lê Thanh Tùng',           'SALES',     'Chuyên viên Kinh doanh'),
('CF-0911','Phạm Văn Dũng',           'LOGISTICS', 'Giám sát trạm cân');
