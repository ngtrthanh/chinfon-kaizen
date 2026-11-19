package app

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type staffImportRow struct {
	Row      int
	EmpID    string
	Name     string
	JobTitle string
	DeptCode string
	DeptName string
	ReportTo string
	Email    string
	Mobile   string
	SeedPIN  string
	Group    string
	Roles    []string
	Scope    string
}

var staffHeaderAliases = map[string]string{
	"emp_id": "emp_id", "employee_id": "emp_id", "employee_no": "emp_id", "ma_nv": "emp_id",
	"name": "name", "full_name": "name", "ho_ten": "name",
	"job_title": "job_title", "position_title": "job_title", "title": "job_title", "chuc_danh": "job_title",
	"group": "group", "work_group": "group", "team": "group", "nhom": "group", "to": "group",
	"dept_code": "dept_code", "department_code": "dept_code", "ma_don_vi": "dept_code",
	"dept_name": "dept_name", "department_name": "dept_name", "ten_don_vi": "dept_name",
	"report_to": "report_to", "manager": "report_to", "quan_ly": "report_to",
	"e_mail": "email", "email": "email",
	"mobile_num": "mobile", "mobile": "mobile", "phone": "mobile", "dien_thoai": "mobile",
	"seed_pin": "seed_pin", "pin": "seed_pin",
	"role": "role", "roles": "role",
}

var staffRoleAliases = map[string]string{
	"employee": RoleWorker, "worker": RoleWorker, "nhanvien": RoleWorker, "nhan_vien": RoleWorker, "nhân viên": RoleWorker,
	"supervisor": RoleSupervisor, "quandoc": RoleSupervisor, "quan_doc": RoleSupervisor, "quản đốc": RoleSupervisor,
	"manager": RoleManager, "truongphong": RoleManager, "truong_phong": RoleManager, "trưởng phòng": RoleManager,
	"director": RoleDirector, "giamdoc": RoleDirector, "giam_doc": RoleDirector, "giám đốc": RoleDirector,
	"council": RoleCouncil, "hoidong": RoleCouncil, "hoi_dong": RoleCouncil, "hội đồng": RoleCouncil,
	"hr": RoleHRD, "hrd": RoleHRD, "nhansu": RoleHRD, "nhan_su": RoleHRD, "nhân sự": RoleHRD,
	"ceo": RoleCEOAudit, "ceo_audit": RoleCEOAudit, "tonggiamdoc": RoleCEOAudit, "tổng giám đốc": RoleCEOAudit,
	"admin": RoleAdmin,
}

var knownRoles = map[string]bool{
	RoleWorker: true, RoleSupervisor: true, RoleManager: true, RoleDirector: true,
	RoleCouncil: true, RoleHRD: true, RoleCEOAudit: true, RoleAdmin: true,
}

func normalizeHeader(s string) string {
	s = strings.TrimPrefix(s, "\uFEFF")
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "_", "-", "_", ".", "_").Replace(s)
	return s
}

func detectDelimiter(line string) rune {
	counts := map[rune]int{',': 0, ';': 0, '\t': 0}
	inQuotes := false
	for _, r := range line {
		if r == '"' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes {
			if _, ok := counts[r]; ok {
				counts[r]++
			}
		}
	}
	best := ','
	max := counts[',']
	for _, r := range []rune{';', '\t'} {
		if counts[r] > max {
			best, max = r, counts[r]
		}
	}
	return best
}

func validStaffEmail(s string) bool {
	if s == "" {
		return true
	}
	if len(s) > 200 || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	at := strings.Index(s, "@")
	if at <= 0 || at >= len(s)-1 {
		return false
	}
	return strings.Contains(s[at+1:], ".")
}

func parseStaffRoles(cell string) ([]string, error) {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return []string{RoleWorker}, nil
	}
	parts := strings.FieldsFunc(cell, func(r rune) bool {
		return r == '|' || r == ';' || r == ',' || r == '/'
	})
	seen := map[string]bool{}
	var roles []string
	for _, p := range parts {
		key := strings.ToLower(strings.TrimSpace(p))
		if key == "" {
			continue
		}
		role, ok := staffRoleAliases[key]
		if !ok {
			up := strings.ToUpper(strings.TrimSpace(p))
			up = strings.ReplaceAll(up, " ", "_")
			if knownRoles[up] {
				role, ok = up, true
			}
		}
		if !ok {
			return nil, fmt.Errorf("vai trò không hợp lệ %q", p)
		}
		if !seen[role] {
			seen[role] = true
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		roles = []string{RoleWorker}
	}
	sort.Strings(roles)
	return roles, nil
}

func scopeForRoles(roles []string, deptCode string) string {
	for _, r := range roles {
		switch r {
		case RoleCouncil, RoleHRD, RoleDirector, RoleCEOAudit, RoleAdmin:
			return "ALL"
		}
	}
	return deptCode
}

func parseStaffCSV(data []byte) ([]staffImportRow, []ImportRowError) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	text := string(data)
	firstLine := text
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		firstLine = text[:i]
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = detectDelimiter(firstLine)
	r.TrimLeadingSpace = true
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, []ImportRowError{{Row: 0, Reason: "Không đọc được CSV: " + err.Error()}}
	}
	if len(records) == 0 {
		return nil, []ImportRowError{{Row: 0, Reason: "File CSV trống."}}
	}
	if len(records[0]) == 1 && strings.HasPrefix(strings.ToLower(strings.TrimSpace(records[0][0])), "sep=") {
		records = records[1:]
	}
	if len(records) == 0 {
		return nil, []ImportRowError{{Row: 0, Reason: "File CSV không có dòng tiêu đề."}}
	}
	if len(records)-1 > 5000 {
		return nil, []ImportRowError{{Row: 0, Reason: "File vượt 5.000 dòng dữ liệu."}}
	}

	idx := map[string]int{}
	for i, h := range records[0] {
		key := staffHeaderAliases[normalizeHeader(h)]
		if key != "" {
			idx[key] = i
		}
	}
	for _, required := range []string{"emp_id", "name", "dept_code"} {
		if _, ok := idx[required]; !ok {
			return nil, []ImportRowError{{Row: 0, Reason: "Thiếu cột bắt buộc: " + required}}
		}
	}
	cell := func(row []string, key string) string {
		i, ok := idx[key]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	var rows []staffImportRow
	var errs []ImportRowError
	seen := map[string]bool{}
	for n, raw := range records[1:] {
		rowNo := n + 2
		allEmpty := true
		for _, c := range raw {
			if strings.TrimSpace(c) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}
		row := staffImportRow{Row: rowNo}
		row.EmpID = cleanCode(cell(raw, "emp_id"))
		row.Name = cell(raw, "name")
		row.JobTitle = cell(raw, "job_title")
		row.DeptCode = cleanCode(cell(raw, "dept_code"))
		row.DeptName = cell(raw, "dept_name")
		row.ReportTo = strings.TrimSpace(cell(raw, "report_to"))
		row.Email = cell(raw, "email")
		row.Mobile = digitsOnly(cell(raw, "mobile"))
		row.SeedPIN = strings.TrimSpace(cell(raw, "seed_pin"))
		row.Group = strings.TrimSpace(cell(raw, "group"))

		rowErr := func(msg string) {
			errs = append(errs, ImportRowError{Row: rowNo, EmpID: row.EmpID, Reason: msg})
		}
		if row.EmpID == "" {
			rowErr("Thiếu emp_id.")
			continue
		}
		if len(row.EmpID) > 50 {
			rowErr("emp_id vượt 50 ký tự.")
			continue
		}
		if seen[row.EmpID] {
			rowErr("emp_id bị trùng trong file.")
			continue
		}
		if row.Name == "" || len([]rune(row.Name)) > 120 {
			rowErr("Thiếu name hoặc name vượt 120 ký tự.")
			continue
		}
		if row.DeptCode == "" {
			rowErr("Thiếu dept_code.")
			continue
		}
		if len(row.DeptCode) > 20 {
			rowErr("dept_code vượt 20 ký tự.")
			continue
		}
		if row.DeptName == "" {
			row.DeptName = row.DeptCode
		}
		if len([]rune(row.DeptName)) > 120 || len([]rune(row.ReportTo)) > 60 {
			rowErr("dept_name hoặc report_to quá dài.")
			continue
		}
		if len([]rune(row.JobTitle)) > 120 || len([]rune(row.Group)) > 120 {
			rowErr("job_title hoặc group vượt 120 ký tự.")
			continue
		}
		if !validStaffEmail(row.Email) {
			rowErr("e_mail không hợp lệ (có thể để trống).")
			continue
		}
		if row.Mobile != "" && (len(row.Mobile) < 5 || len(row.Mobile) > 15) {
			rowErr("mobile_num nếu có phải 5–15 chữ số.")
			continue
		}
		if row.Mobile == "" {
			// Accepted: import the employee without an account. The account can be
			// provisioned later by re-importing the same emp_id with a mobile.
			row.SeedPIN = ""
		} else if row.SeedPIN != "" {
			pin := last5Digits(row.Mobile)
			if digitsOnly(row.SeedPIN) != row.SeedPIN || len(row.SeedPIN) != 5 {
				rowErr("seed_pin phải là đúng 5 chữ số.")
				continue
			}
			if row.SeedPIN != pin {
				rowErr("seed_pin phải bằng 5 số cuối của mobile_num.")
				continue
			}
		}
		roles, err := parseStaffRoles(cell(raw, "role"))
		if err != nil {
			rowErr(err.Error())
			continue
		}
		row.Roles = roles
		row.Scope = scopeForRoles(roles, row.DeptCode)
		seen[row.EmpID] = true
		rows = append(rows, row)
	}
	if len(rows) == 0 && len(errs) == 0 {
		errs = append(errs, ImportRowError{Row: 0, Reason: "Không có dòng dữ liệu."})
	}
	return rows, errs
}

func (a *App) applyStaffImport(rows []staffImportRow, resetPIN, hideDemo bool) (StaffImportResult, error) {
	res := StaffImportResult{Total: len(rows)}
	tx, err := a.DB.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	var oldDemo string
	_ = tx.QueryRow(`SELECT value FROM app_setting WHERE key='demo_mode'`).Scan(&oldDemo)
	depts := map[string]bool{}

	for _, row := range rows {
		if _, err := tx.Exec(`
			INSERT INTO md_department(code,name,kind,sort_order,active,is_demo) VALUES(?,?,'PLANT',100,1,0)
			ON CONFLICT(code) DO UPDATE SET
				name = CASE WHEN excluded.name <> '' AND excluded.name <> excluded.code THEN excluded.name ELSE md_department.name END,
				active = 1, is_demo = 0`, row.DeptCode, row.DeptName); err != nil {
			return res, fmt.Errorf("dòng %d (%s): lưu đơn vị: %w", row.Row, row.EmpID, err)
		}
		depts[row.DeptCode] = true

		if _, err := tx.Exec(`
			INSERT INTO md_employee(employee_no,full_name,department_code,position_title,email,mobile_num,report_to,work_group,active,is_demo)
			VALUES(?,?,?,?,?,?,?,?,1,0)
			ON CONFLICT(employee_no) DO UPDATE SET
				full_name=excluded.full_name,
				department_code=excluded.department_code,
				position_title=CASE WHEN excluded.position_title <> '' THEN excluded.position_title ELSE md_employee.position_title END,
				email=CASE WHEN excluded.email <> '' THEN excluded.email ELSE md_employee.email END,
				mobile_num=excluded.mobile_num,
				report_to=CASE WHEN excluded.report_to <> '' THEN excluded.report_to ELSE md_employee.report_to END,
				work_group=CASE WHEN excluded.work_group <> '' THEN excluded.work_group ELSE md_employee.work_group END,
				active=1, is_demo=0`,
			row.EmpID, row.Name, row.DeptCode, row.JobTitle, row.Email, row.Mobile, row.ReportTo, row.Group); err != nil {
			return res, fmt.Errorf("dòng %d (%s): lưu nhân viên: %w", row.Row, row.EmpID, err)
		}
		res.EmployeesUpserted++

		var userID int64
		mustCSRFToken, tokenErr := randomToken(24)
		if tokenErr != nil {
			return res, tokenErr
		}
		err := tx.QueryRow(`SELECT id FROM app_user WHERE employee_no=?`, row.EmpID).Scan(&userID)
		switch err {
		case sql.ErrNoRows:
			// No phone or company email is required. First access uses a
			// short-lived random invite issued by a verified administrator.
			var conflict int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM app_user WHERE username=? COLLATE NOCASE`, row.EmpID).Scan(&conflict); err != nil {
				return res, err
			}
			if conflict > 0 {
				return res, fmt.Errorf("dòng %d (%s): username %s đã thuộc nhân viên khác", row.Row, row.EmpID, row.EmpID)
			}
			out, err := tx.Exec(`INSERT INTO app_user(username,password_hash,employee_no,dept_scope,active,must_change_password,csrf_token)
				VALUES(?, 'pending', ?, ?, 1, 1, ?)`, row.EmpID, row.EmpID, row.Scope, mustCSRFToken)
			if err != nil {
				return res, fmt.Errorf("dòng %d (%s): tạo tài khoản: %w", row.Row, row.EmpID, err)
			}
			userID, _ = out.LastInsertId()
			res.UsersCreated++
		case nil:
			if resetPIN {
				if _, err := tx.Exec(`UPDATE app_user SET dept_scope=?, active=1, password_hash='pending', must_change_password=1, csrf_token=? WHERE id=?`, row.Scope, mustCSRFToken, userID); err != nil {
					return res, err
				}
				_, _ = tx.Exec(`DELETE FROM app_session WHERE user_id=?`, userID)
			} else {
				if _, err := tx.Exec(`UPDATE app_user SET dept_scope=?, active=1 WHERE id=?`, row.Scope, userID); err != nil {
					return res, err
				}
			}
			res.UsersUpdated++
		default:
			return res, fmt.Errorf("dòng %d (%s): đọc tài khoản: %w", row.Row, row.EmpID, err)
		}

		if _, err := tx.Exec(`DELETE FROM app_user_role WHERE user_id=?`, userID); err != nil {
			return res, err
		}
		for _, role := range row.Roles {
			if _, err := tx.Exec(`INSERT INTO app_user_role(user_id,role_code) VALUES(?,?)`, userID, role); err != nil {
				return res, fmt.Errorf("dòng %d (%s): gán vai trò %s: %w", row.Row, row.EmpID, role, err)
			}
		}
	}

	// Never let an import remove the last active administrator.
	var admins int
	if err := tx.QueryRow(`
		SELECT COUNT(DISTINCT u.id) FROM app_user u
		JOIN app_user_role ur ON ur.user_id=u.id
		WHERE ur.role_code='ADMIN' AND u.active=1`).Scan(&admins); err != nil {
		return res, err
	}
	if admins == 0 {
		return res, fmt.Errorf("import sẽ không còn tài khoản ADMIN hoạt động; đã hủy toàn bộ")
	}

	res.Departments = len(depts)
	if _, err := tx.Exec(`INSERT INTO app_setting(key,value) VALUES('demo_mode','0')
		ON CONFLICT(key) DO UPDATE SET value='0'`); err != nil {
		return res, err
	}
	res.DemoDisabled = oldDemo == "1"
	if hideDemo && oldDemo == "1" {
		if _, err := tx.Exec(`UPDATE initiative SET is_demo=1 WHERE is_demo=0`); err != nil {
			return res, err
		}
	}
	if err := tx.Commit(); err != nil {
		return res, err
	}
	if res.DemoDisabled {
		res.Notice = "Đã tắt chế độ demo. Tài khoản mới cần mã kích hoạt ngẫu nhiên có thời hạn cấp bởi quản trị, sau đó đổi mật khẩu và thiết lập MFA."
	} else {
		res.Notice = "Đã cập nhật nhân sự. Chế độ vận hành đang bật; dữ liệu demo vẫn bị ẩn."
	}
	res.Notice += " Mọi nhân viên, kể cả không có email/SĐT, được tạo tài khoản PENDING và chỉ đăng nhập sau khi quản trị cấp mã mời ngẫu nhiên 30 phút."
	return res, nil
}

func (a *App) ImportStaffFile(path string, resetPIN bool) (StaffImportResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StaffImportResult{}, fmt.Errorf("read %s: %w", path, err)
	}
	rows, errs := parseStaffCSV(data)
	return a.applyParsedStaffImport(rows, errs, resetPIN, false)
}

func hasGlobalImportError(errs []ImportRowError) bool {
	for _, e := range errs {
		if e.Row <= 0 {
			return true
		}
	}
	return false
}

func countRowErrors(errs []ImportRowError) int {
	n := 0
	for _, e := range errs {
		if e.Row > 0 {
			n++
		}
	}
	return n
}

// applyParsedStaffImport commits every valid row. Row-level errors are returned
// in the result and do not roll back the valid rows; file/header errors do.
func (a *App) applyParsedStaffImport(rows []staffImportRow, errs []ImportRowError, resetPIN, hideDemo bool) (StaffImportResult, error) {
	res := StaffImportResult{
		Total:       len(rows) + countRowErrors(errs),
		SkippedRows: countRowErrors(errs),
		Errors:      errs,
	}
	if hasGlobalImportError(errs) {
		res.Notice = "Chưa thay đổi dữ liệu: lỗi file hoặc dòng tiêu đề."
		return res, nil
	}
	if len(rows) == 0 {
		res.Notice = "Không có dòng hợp lệ để import."
		return res, nil
	}
	applied, err := a.applyStaffImport(rows, resetPIN, hideDemo)
	if err != nil {
		return res, err
	}
	applied.Total = res.Total
	applied.SkippedRows = res.SkippedRows
	applied.Errors = res.Errors
	if applied.SkippedRows > 0 {
		applied.Notice = fmt.Sprintf("Đã import %d/%d dòng hợp lệ; bỏ qua %d dòng lỗi. ", applied.EmployeesUpserted, applied.Total, applied.SkippedRows) + applied.Notice
	}
	return applied, nil
}

func (a *App) staffRows() ([]StaffRow, error) {
	rows, err := a.DB.Query(`
		SELECT e.employee_no, e.full_name, COALESCE(e.position_title,''), e.department_code, d.name,
		       COALESCE(e.report_to,''), COALESCE(e.email,''), COALESCE(e.mobile_num,''), COALESCE(e.work_group,''), e.active,
		       COALESCE(u.username,''), COALESCE(u.dept_scope,''), COALESCE(u.must_change_password,0),
		       COALESCE(u.totp_confirmed,0),
		       COALESCE(GROUP_CONCAT(ur.role_code,'|'),'')
		FROM md_employee e
		JOIN md_department d ON d.code=e.department_code
		LEFT JOIN app_user u ON u.employee_no=e.employee_no
		LEFT JOIN app_user_role ur ON ur.user_id=u.id
		WHERE e.is_demo=0
		GROUP BY e.employee_no, e.full_name, e.position_title, e.department_code, d.name, e.report_to, e.email, e.mobile_num,
		         e.work_group, e.active, u.username, u.dept_scope, u.must_change_password, u.totp_confirmed
		ORDER BY d.sort_order, e.employee_no
		LIMIT 5000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StaffRow
	for rows.Next() {
		var s StaffRow
		var active, must, mfaOn int
		var roles string
		if err := rows.Scan(&s.EmployeeNo, &s.FullName, &s.JobTitle, &s.DepartmentCode, &s.DepartmentName,
			&s.ReportTo, &s.Email, &s.MobileNum, &s.Group, &active, &s.Username, &s.Scope, &must, &mfaOn, &roles); err != nil {
			return nil, err
		}
		s.Active = active == 1
		s.MustChange = must == 1
		s.MFAEnrolled = mfaOn == 1
		s.HasAccount = s.Username != ""
		if roles != "" {
			s.Roles = strings.Split(roles, "|")
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (a *App) staffTemplate(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/staff/template.csv") {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="chinfon_staff_template.csv"`)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	_ = cw.Write([]string{"emp_id", "name", "job_title", "dept_code", "dept_name", "report_to", "e_mail", "mobile_num", "seed_pin", "role", "group"})
	cw.Flush()
}

const staffSampleCSV = `emp_id,name,job_title,dept_code,dept_name,report_to,e_mail,mobile_num,seed_pin,role,group
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
`

func (a *App) staffSample(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/staff/sample.csv") {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="chinfon_staff_P81_sample.csv"`)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	_, _ = w.Write([]byte(staffSampleCSV))
}

func (a *App) staffExport(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/staff/export.csv") {
		return
	}
	rows, err := a.staffRows()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	filename := fmt.Sprintf("Chinfon_Staff_%s.csv", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	_ = cw.Write([]string{"emp_id", "name", "job_title", "dept_code", "dept_name", "report_to", "e_mail", "mobile_num", "role", "group", "username", "scope", "account_status", "mfa"})
	for _, s := range rows {
		status := "no_account"
		if s.HasAccount {
			status = "active"
			if s.MustChange {
				status = "must_change_password"
			}
		}
		mfaStatus := "no_account"
		if s.HasAccount {
			mfaStatus = "not_enrolled"
			if s.MFAEnrolled {
				mfaStatus = "enrolled"
			}
		}
		_ = cw.Write([]string{
			csvSafe(s.EmployeeNo), csvSafe(s.FullName), csvSafe(s.JobTitle), csvSafe(s.DepartmentCode), csvSafe(s.DepartmentName),
			csvSafe(s.ReportTo), csvSafe(s.Email), csvSafe(s.MobileNum), csvSafe(strings.Join(s.Roles, "|")),
			csvSafe(s.Group), csvSafe(s.Username), csvSafe(s.Scope), status, mfaStatus,
		})
	}
	cw.Flush()
	a.audit(r, u.EmployeeNo, "STAFF_EXPORT", filename, "200 OK", fmt.Sprintf("rows=%d", len(rows)))
}

func (a *App) staffImportPost(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/staff/import") {
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		a.renderStaffPage(w, r, u, &StaffImportResult{Errors: []ImportRowError{{Reason: "File quá lớn hoặc biểu mẫu không hợp lệ."}}})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		a.renderStaffPage(w, r, u, &StaffImportResult{Errors: []ImportRowError{{Reason: "Vui lòng chọn file CSV."}}})
		return
	}
	defer file.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(io.LimitReader(file, 8<<20)); err != nil {
		a.renderStaffPage(w, r, u, &StaffImportResult{Errors: []ImportRowError{{Reason: "Không đọc được file."}}})
		return
	}
	rows, errs := parseStaffCSV(buf.Bytes())
	applied, err := a.applyParsedStaffImport(rows, errs, r.FormValue("reset_pin") == "1", r.FormValue("hide_demo") == "1")
	if err != nil {
		result := &StaffImportResult{
			Total:       len(rows) + countRowErrors(errs),
			SkippedRows: countRowErrors(errs),
			Errors:      append(errs, ImportRowError{Reason: err.Error()}),
			Notice:      "Import thất bại; toàn bộ thay đổi đã rollback.",
		}
		a.renderStaffPage(w, r, u, result)
		return
	}
	if applied.DemoDisabled {
		u.DemoMode = false
	}
	if applied.EmployeesUpserted > 0 {
		a.audit(r, u.EmployeeNo, "STAFF_IMPORT", "staff", "200 OK",
			fmt.Sprintf("rows=%d skipped=%d depts=%d created=%d updated=%d demo_off=%v",
				applied.Total, applied.SkippedRows, applied.Departments, applied.UsersCreated, applied.UsersUpdated, applied.DemoDisabled))
	}
	a.renderStaffPage(w, r, u, &applied)
}

func (a *App) renderStaffPage(w http.ResponseWriter, r *http.Request, u *User, result *StaffImportResult) {
	d, err := a.adminPageData(u, r, "staff")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d.ImportResult = result
	applyFlash(&d, r)
	a.render(w, "admin", d)
}

func (a *App) adminStaffDepartment(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/staff/department") {
		return
	}
	code := cleanCode(r.FormValue("dept_code"))
	name := strings.TrimSpace(r.FormValue("dept_name"))
	if code == "" || name == "" {
		adminRedirect(w, r, "staff", "", "Mã và tên đơn vị là bắt buộc.")
		return
	}
	if _, err := a.DB.Exec(`INSERT INTO md_department(code,name,kind,sort_order,active,is_demo) VALUES(?,?,'PLANT',100,1,0)
		ON CONFLICT(code) DO UPDATE SET name=excluded.name, active=1, is_demo=0`, code, name); err != nil {
		adminRedirect(w, r, "staff", "", "Không tạo được đơn vị: "+err.Error())
		return
	}
	a.audit(r, u.EmployeeNo, "STAFF_DEPARTMENT_SEED", code, "200 OK", name)
	adminRedirect(w, r, "staff", "Đã tạo/cập nhật đơn vị "+code, "")
}

func (a *App) adminCouncilCreate(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/staff/council") {
		return
	}
	empNo := cleanCode(r.FormValue("employee_no"))
	var active int
	err := a.DB.QueryRow(`SELECT active FROM md_employee WHERE employee_no=? AND active=1`, empNo).Scan(&active)
	if err != nil {
		adminRedirect(w, r, "staff", "", "Không tìm thấy nhân viên hoạt động.")
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		adminRedirect(w, r, "staff", "", err.Error())
		return
	}
	defer tx.Rollback()
	var uid int64
	err = tx.QueryRow(`SELECT id FROM app_user WHERE employee_no=?`, empNo).Scan(&uid)
	if err == sql.ErrNoRows {
		// Council members without personal phones can be provisioned too.
		// CLI issue-invite must be used for their first login.
		var conflict int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM app_user WHERE username=? COLLATE NOCASE`, empNo).Scan(&conflict)
		if conflict > 0 {
			adminRedirect(w, r, "staff", "", "Username đã thuộc nhân viên khác.")
			return
		}
		mustCSRFToken, terr := randomToken(24)
		if terr != nil {
			adminRedirect(w, r, "staff", "", terr.Error())
			return
		}
		out, err := tx.Exec(`INSERT INTO app_user(username,password_hash,employee_no,dept_scope,active,must_change_password,csrf_token)
			VALUES(?, 'pending', ?, 'ALL', 1, 1, ?)`, empNo, empNo, mustCSRFToken)
		if err != nil {
			adminRedirect(w, r, "staff", "", "Không tạo được tài khoản: "+err.Error())
			return
		}
		uid, _ = out.LastInsertId()
	} else if err == nil {
		if _, err := tx.Exec(`UPDATE app_user SET dept_scope='ALL', active=1 WHERE id=?`, uid); err != nil {
			adminRedirect(w, r, "staff", "", err.Error())
			return
		}
	} else {
		adminRedirect(w, r, "staff", "", err.Error())
		return
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO app_user_role(user_id,role_code) VALUES(?, 'COUNCIL')`, uid); err != nil {
		adminRedirect(w, r, "staff", "", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		adminRedirect(w, r, "staff", "", err.Error())
		return
	}
	a.audit(r, u.EmployeeNo, "COUNCIL_USER_PROVISION", empNo, "200 OK", "Tạo/cập nhật tài khoản Hội đồng")
	adminRedirect(w, r, "staff", "Đã tạo/cập nhật tài khoản Hội đồng cho "+empNo, "")
}
