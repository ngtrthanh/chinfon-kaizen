package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"chinfon-kaizen/internal/mfa"
	"chinfon-kaizen/internal/security"
)

func mustAdmin(t *testing.T, a *App) {
	t.Helper()
	hash, err := security.HashPassword("Admin@12345")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.createUser("admin", hash, "CF-0001", "ALL", []string{RoleAdmin}); err != nil {
		t.Fatal(err)
	}
}

func TestSampleStaffCSVParses(t *testing.T) {
	rows, errs := parseStaffCSV([]byte(staffSampleCSV))
	if len(errs) != 0 {
		t.Fatalf("sample CSV has errors: %#v", errs)
	}
	if len(rows) != 10 {
		t.Fatalf("sample rows=%d", len(rows))
	}
	if rows[6].Roles[0] != RoleAdmin {
		t.Fatalf("sample admin row roles=%v", rows[6].Roles)
	}
}

func TestStaffPageTemplateAndSample(t *testing.T) {
	a := newTestApp(t, false)
	if _, err := a.DB.Exec(`INSERT INTO md_department(code,name,kind,sort_order,active) VALUES('P81','Phòng Kỹ thuật 81','PLANT',100,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(`INSERT INTO md_employee(employee_no,full_name,department_code,position_title,work_group,active)
		VALUES('P81-0001','Nguyễn Văn A','P81','Kỹ sư Cơ khí','Tổ Cơ khí',1)`); err != nil {
		t.Fatal(err)
	}
	u := testUser("ADMIN1", "ALL", RoleAdmin)
	u.CSRFToken = "csrf-test"
	d, err := a.adminPageData(u, httptest.NewRequest(http.MethodGet, "/admin?tab=staff", nil), "staff")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Staff) == 0 {
		t.Fatal("no staff rows")
	}
	var found *StaffRow
	for i := range d.Staff {
		if d.Staff[i].EmployeeNo == "P81-0001" {
			found = &d.Staff[i]
			break
		}
	}
	if found == nil || found.JobTitle != "Kỹ sư Cơ khí" || found.Group != "Tổ Cơ khí" {
		t.Fatalf("staff row not populated: %+v", found)
	}
	var buf bytes.Buffer
	if err := a.Templates.ExecuteTemplate(&buf, "admin", d); err != nil {
		t.Fatalf("render staff admin page: %v", err)
	}
	if !strings.Contains(buf.String(), "Tổ Cơ khí") || !strings.Contains(buf.String(), "Kỹ sư Cơ khí") {
		t.Fatal("staff page missing new job_title/group columns")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/staff/sample.csv", nil)
	a.staffSample(rec, req, u)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "emp_id,name,job_title") || !strings.Contains(rec.Body.String(), ",group") {
		t.Fatalf("sample csv missing new fields: %d %q", rec.Code, rec.Body.String()[:80])
	}
}

func TestParseStaffCSV(t *testing.T) {
	csv := "\uFEFFemp_id,name,job_title,dept_code,dept_name,report_to,e_mail,mobile_num,seed_pin,role,group\r\n" +
		"P81-0001,Nguyễn Văn A,Kỹ sư Cơ khí,P81,Phòng Kỹ thuật 81,P81-0000,a@chinfon.com,0912345678,,employee,Tổ Cơ khí\r\n" +
		"P81-0002,Trần Thị B,Giám sát,P81,Phòng Kỹ thuật 81,P81-0001,,0909876543,76543,manager|employee,Ca A\r\n" +
		"P81-0003,Lê Văn C,Thành viên HĐ,P81,Phòng Kỹ thuật 81,P81-0001,,0901112222,,council,Hội đồng ĐGKT\r\n"
	rows, errs := parseStaffCSV([]byte(csv))
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %#v", errs)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].Scope != "P81" {
		t.Fatalf("worker scope=%s", rows[0].Scope)
	}
	if rows[0].JobTitle != "Kỹ sư Cơ khí" || rows[0].Group != "Tổ Cơ khí" {
		t.Fatalf("job_title/group not parsed: %+v", rows[0])
	}
	if rows[2].Scope != "ALL" || !contains(rows[2].Roles, RoleCouncil) {
		t.Fatalf("council row: %+v", rows[2])
	}
	if !contains(rows[1].Roles, RoleManager) || !contains(rows[1].Roles, RoleWorker) {
		t.Fatalf("multi-role row: %+v", rows[1].Roles)
	}
}

func TestParseStaffCSVValidation(t *testing.T) {
	header := "emp_id,name,dept_code,dept_name,report_to,e_mail,mobile_num,seed_pin,role\n"
	cases := []string{
		header + "P81-0001,A,P81,,,a@b.com,0912345678,99999,employee\n",                                           // PIN mismatch
		header + "P81-0001,A,P81,,,a@b.com,0912345678,,wizard\n",                                                  // invalid role
		header + "P81-0001,A,P81,,,bad-email,0912345678,,employee\n",                                              // bad email
		header + "P81-0001,A,P81,,,a@b.com,0912345678,,employee\nP81-0001,B,P81,,,a@b.com,0912345678,,employee\n", // duplicate
	}
	for i, c := range cases {
		_, errs := parseStaffCSV([]byte(c))
		if len(errs) == 0 {
			t.Fatalf("case %d should fail, errs=%#v", i, errs)
		}
	}
}

func TestParseStaffCSVMissingMobileAccepted(t *testing.T) {
	csv := "emp_id,name,dept_code,dept_name,e_mail,mobile_num,role\n" +
		"P81-0001,Nguyễn Văn A,P81,Phòng Kỹ thuật 81,,,employee\n"
	rows, errs := parseStaffCSV([]byte(csv))
	if len(errs) != 0 || len(rows) != 1 {
		t.Fatalf("missing mobile should be accepted: rows=%d errs=%#v", len(rows), errs)
	}
	if rows[0].Mobile != "" {
		t.Fatalf("mobile should be empty: %+v", rows[0])
	}
}

func TestStaffImportWithoutMobileSkipsAccount(t *testing.T) {
	a := newTestApp(t, false)
	mustAdmin(t, a)
	csv := "emp_id,name,mobile_num,dept_code,dept_name\n" +
		"P81-0001,Nguyễn Văn A,,P81,Phòng Kỹ thuật 81\n" +
		"P81-0002,Trần Thị B,,P81,Phòng Kỹ thuật 81\n"
	rows, errs := parseStaffCSV([]byte(csv))
	if len(errs) != 0 {
		t.Fatalf("parse: %#v", errs)
	}
	res, err := a.applyStaffImport(rows, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.EmployeesUpserted != 2 || res.UsersCreated != 0 || res.AccountsSkipped != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	var n int
	_ = a.DB.QueryRow(`SELECT COUNT(*) FROM app_user WHERE employee_no='P81-0001'`).Scan(&n)
	if n != 0 {
		t.Fatalf("account should not exist without mobile, got %d", n)
	}

	// Adding the mobile later provisions the account on re-import.
	rows, errs = parseStaffCSV([]byte("emp_id,name,mobile_num,dept_code\nP81-0001,Nguyễn Văn A,0912345678,P81\n"))
	if len(errs) != 0 {
		t.Fatalf("parse2: %#v", errs)
	}
	res, err = a.applyStaffImport(rows, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.UsersCreated != 1 || res.AccountsSkipped != 0 {
		t.Fatalf("re-import should create account: %+v", res)
	}
	_ = a.DB.QueryRow(`SELECT COUNT(*) FROM app_user WHERE employee_no='P81-0001'`).Scan(&n)
	if n != 1 {
		t.Fatalf("account should exist after adding mobile, got %d", n)
	}
}

func TestPartialImportKeepsValidRows(t *testing.T) {
	a := newTestApp(t, false)
	mustAdmin(t, a)
	path := filepath.Join(t.TempDir(), "staff.csv")
	content := "emp_id,name,mobile_num,dept_code\n" +
		"P81-0001,Valid Row,0912345678,P81\n" +
		"P81-0002,,0901112222,P81\n" // missing name -> skipped
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := a.ImportStaffFile(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || res.SkippedRows != 1 || res.EmployeesUpserted != 1 || res.UsersCreated != 1 || len(res.Errors) != 1 {
		t.Fatalf("unexpected partial result: %+v", res)
	}
	var n int
	_ = a.DB.QueryRow(`SELECT COUNT(*) FROM md_employee WHERE employee_no='P81-0001'`).Scan(&n)
	if n != 1 {
		t.Fatalf("valid row not imported, got %d", n)
	}
	_ = a.DB.QueryRow(`SELECT COUNT(*) FROM md_employee WHERE employee_no='P81-0002'`).Scan(&n)
	if n != 0 {
		t.Fatalf("invalid row should be skipped, got %d", n)
	}
}

func TestStaffImportDisablesDemoAndIsIdempotent(t *testing.T) {
	a := newTestApp(t, true)
	mustAdmin(t, a)
	csv := "emp_id,name,job_title,dept_code,dept_name,report_to,e_mail,mobile_num,seed_pin,role,group\n" +
		"P81-0001,Nguyễn Văn A,Kỹ sư Cơ khí,P81,Phòng Kỹ thuật 81,P81-0000,a@chinfon.com,0912345678,,employee,Tổ Cơ khí\n" +
		"P81-0002,Trần Thị B,Giám sát,P81,Phòng Kỹ thuật 81,P81-0001,,0909876543,,manager|employee,Ca A\n" +
		"P81-0003,Lê Văn C,Thành viên HĐ,P81,Phòng Kỹ thuật 81,P81-0001,,0901112222,,council,Hội đồng ĐGKT\n"
	rows, errs := parseStaffCSV([]byte(csv))
	if len(errs) != 0 {
		t.Fatalf("parse: %#v", errs)
	}
	res, err := a.applyStaffImport(rows, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.DemoDisabled || res.UsersCreated != 3 || res.Departments != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if demoMode(a.DB) {
		t.Fatal("demo_mode should be off after import")
	}
	var hash string
	var must int
	if err := a.DB.QueryRow(`SELECT password_hash,must_change_password FROM app_user WHERE employee_no='P81-0001'`).Scan(&hash, &must); err != nil {
		t.Fatal(err)
	}
	if hash != "pending" || must != 1 {
		t.Fatalf("bootstrap account not pending: hash=%q must=%d", hash, must)
	}
	var scope string
	if err := a.DB.QueryRow(`SELECT dept_scope FROM app_user WHERE employee_no='P81-0003'`).Scan(&scope); err != nil || scope != "ALL" {
		t.Fatalf("council scope=%q err=%v", scope, err)
	}
	if err := a.DB.QueryRow(`SELECT dept_scope FROM app_user WHERE employee_no='P81-0002'`).Scan(&scope); err != nil || scope != "P81" {
		t.Fatalf("manager scope=%q err=%v", scope, err)
	}
	var jobTitle, workGroup string
	if err := a.DB.QueryRow(`SELECT COALESCE(position_title,''), work_group FROM md_employee WHERE employee_no='P81-0001'`).Scan(&jobTitle, &workGroup); err != nil || jobTitle != "Kỹ sư Cơ khí" || workGroup != "Tổ Cơ khí" {
		t.Fatalf("job/group not stored: %q/%q err=%v", jobTitle, workGroup, err)
	}

	// Demo records are hidden in production mode even for an imported worker.
	worker := testUser("P81-0001", "P81", RoleWorker)
	worker.DemoMode = false
	list, err := a.listInitiatives(worker, ListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("production worker should not see demo records, got %d", len(list))
	}
	demo, err := a.getInitiative(1)
	if err != nil {
		t.Fatal(err)
	}
	if worker.CanView(demo) {
		t.Fatal("production worker must not open a demo record by id")
	}

	// Re-import updates accounts instead of duplicating them.
	res2, err := a.applyStaffImport(rows, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if res2.UsersCreated != 0 || res2.UsersUpdated != 3 {
		t.Fatalf("re-import should update 3 users, got %+v", res2)
	}
}

func TestBootstrapPINLoginForcesPasswordChange(t *testing.T) {
	a := newTestApp(t, true)
	mustAdmin(t, a)
	csv := "emp_id,name,dept_code,dept_name,report_to,e_mail,mobile_num,seed_pin,role\n" +
		"P81-0001,Nguyễn Văn A,P81,Phòng Kỹ thuật 81,,,0912345678,,employee\n"
	rows, errs := parseStaffCSV([]byte(csv))
	if len(errs) != 0 {
		t.Fatalf("parse: %#v", errs)
	}
	if _, err := a.applyStaffImport(rows, false, true); err != nil {
		t.Fatal(err)
	}
	h := a.Handler()

	csrf, loginCookies := loginPageCSRF(t, h)
	rec := postForm(t, h, "/login", url.Values{
		"_csrf": {csrf}, "username": {"P81-0001"}, "password": {"45678"},
	}, loginCookies)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Header().Get("Location"), "/account") {
		t.Fatalf("bootstrap login should force account change, got %q", rec.Header().Get("Location"))
	}
	session := rec.Result().Cookies()
	if len(session) == 0 {
		t.Fatal("no session cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range session {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account" {
		t.Fatalf("must-change user should be redirected to /account, got %d %q", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/account", nil)
	for _, c := range session {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Bắt buộc đổi mật khẩu") {
		t.Fatalf("account page missing forced-change warning: %d", rec.Code)
	}
	csrf = regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())[1]

	rec = postForm(t, h, "/account/password", url.Values{
		"_csrf": {csrf}, "current_password": {"45678"}, "new_password": {"StrongPass123"},
	}, session)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/mfa/setup" {
		t.Fatalf("password change should lead to MFA setup: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	// Enrol TOTP using the secret shown on the setup page.
	req = httptest.NewRequest(http.MethodGet, "/mfa/setup", nil)
	for _, c := range session {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("mfa setup page status=%d body=%s", rec.Code, rec.Body.String())
	}
	secret := regexp.MustCompile(`class="secret">([A-Z2-7]+)<`).FindStringSubmatch(rec.Body.String())
	if len(secret) < 2 {
		t.Fatal("TOTP secret missing from setup page")
	}
	csrf = regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())[1]
	code, err := mfa.CodeAt(secret[1], time.Now().Unix()/mfa.Period)
	if err != nil {
		t.Fatal(err)
	}
	rec = postForm(t, h, "/mfa/setup", url.Values{"_csrf": {csrf}, "code": {code}}, session)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/") {
		t.Fatalf("MFA enrolment failed: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}

	// New login with the strong password must stop at the MFA challenge.
	csrf, loginCookies = loginPageCSRF(t, h)
	rec = postForm(t, h, "/login", url.Values{
		"_csrf": {csrf}, "username": {"P81-0001"}, "password": {"StrongPass123"},
	}, loginCookies)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/mfa/verify" {
		t.Fatalf("login with new password should require MFA: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	session = rec.Result().Cookies()

	req = httptest.NewRequest(http.MethodGet, "/mfa/verify", nil)
	for _, c := range session {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("mfa verify page status=%d", rec.Code)
	}
	csrf = regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())[1]
	nextCode, err := mfa.CodeAt(secret[1], time.Now().Add(30*time.Second).Unix()/mfa.Period)
	if err != nil {
		t.Fatal(err)
	}
	rec = postForm(t, h, "/mfa/verify", url.Values{"_csrf": {csrf}, "code": {nextCode}}, session)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("MFA verification failed: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range session {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("full session should reach dashboard, got %d", rec.Code)
	}

	// One-time code: a fresh partial session must not accept the code already used.
	dashCSRF := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())[1]
	rec = postForm(t, h, "/logout", url.Values{"_csrf": {dashCSRF}}, session)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("logout failed: %d", rec.Code)
	}
	csrf, loginCookies = loginPageCSRF(t, h)
	rec = postForm(t, h, "/login", url.Values{
		"_csrf": {csrf}, "username": {"P81-0001"}, "password": {"StrongPass123"},
	}, loginCookies)
	session = rec.Result().Cookies()
	req = httptest.NewRequest(http.MethodGet, "/mfa/verify", nil)
	for _, c := range session {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	csrf = regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())[1]
	rec = postForm(t, h, "/mfa/verify", url.Values{"_csrf": {csrf}, "code": {nextCode}}, session)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "đã được dùng") {
		t.Fatalf("replayed MFA code should be rejected, got %d", rec.Code)
	}
}

func loginPageCSRF(t *testing.T, h http.Handler) (string, []*http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login page status=%d", rec.Code)
	}
	m := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())
	if len(m) < 2 {
		t.Fatal("login CSRF missing")
	}
	return m[1], rec.Result().Cookies()
}

func postForm(t *testing.T, h http.Handler, path string, form url.Values, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
