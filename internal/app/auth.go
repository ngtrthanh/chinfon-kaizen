package app

import (
	"crypto/subtle"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"chinfon-kaizen/internal/security"
)

type loginAttempt struct {
	count int
	first time.Time
}

type loginLimiter struct {
	mu    sync.Mutex
	items map[string]loginAttempt
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{items: map[string]loginAttempt{}}
}

func (l *loginLimiter) allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.items[key]
	if !ok {
		return true
	}
	if time.Since(a.first) > 10*time.Minute {
		delete(l.items, key)
		return true
	}
	return a.count < 10
}

func (l *loginLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.items[key]
	if a.first.IsZero() || time.Since(a.first) > 10*time.Minute {
		a = loginAttempt{first: time.Now()}
	}
	a.count++
	l.items[key] = a
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.items, key)
}

func (a *App) usersExist() (bool, error) {
	var n int
	err := a.DB.QueryRow(`SELECT COUNT(*) FROM app_user`).Scan(&n)
	return n > 0, err
}

func (a *App) currentUser(r *http.Request) (*User, error) {
	c, err := r.Cookie(a.SessionCookie)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return nil, sql.ErrNoRows
	}
	now := nowUTC()
	row := a.DB.QueryRow(`
		SELECT u.id, u.username, u.employee_no, e.full_name, e.department_code, d.name, u.dept_scope, u.csrf_token,
		       u.must_change_password, u.totp_confirmed, s.mfa_passed,
		       COALESCE((SELECT value FROM app_setting WHERE key='demo_mode'),'1')
		FROM app_session s
		JOIN app_user u ON u.id = s.user_id AND u.active = 1
		JOIN md_employee e ON e.employee_no = u.employee_no AND e.active = 1
		JOIN md_department d ON d.code = e.department_code
		WHERE s.token_hash = ? AND s.expires_at > ?`,
		tokenHash(c.Value), now)
	var u User
	var mustChange, mfaEnrolled, mfaPassed int
	var demo string
	if err := row.Scan(&u.ID, &u.Username, &u.EmployeeNo, &u.FullName, &u.DepartmentCode, &u.DepartmentName, &u.Scope, &u.CSRFToken, &mustChange, &mfaEnrolled, &mfaPassed, &demo); err != nil {
		return nil, err
	}
	if u.CSRFToken == "" {
		token, err := randomToken(24)
		if err != nil {
			return nil, err
		}
		if _, err := a.DB.Exec(`UPDATE app_user SET csrf_token=? WHERE id=?`, token, u.ID); err != nil {
			return nil, err
		}
		u.CSRFToken = token
	}
	u.MustChangePassword = mustChange == 1
	u.MFAEnrolled = mfaEnrolled == 1
	u.MFAPassed = mfaPassed == 1
	u.DemoMode = demo == "1"
	u.Permissions = map[string]bool{}
	rows, err := a.DB.Query(`
		SELECT ur.role_code FROM app_user_role ur WHERE ur.user_id = ? ORDER BY ur.role_code`, u.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			rows.Close()
			return nil, err
		}
		u.Roles = append(u.Roles, role)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	permRows, err := a.DB.Query(`
		SELECT DISTINCT rp.permission_code
		FROM app_user_role ur
		JOIN rbac_role_permission rp ON rp.role_code = ur.role_code
		WHERE ur.user_id = ?`, u.ID)
	if err != nil {
		return nil, err
	}
	defer permRows.Close()
	for permRows.Next() {
		var p string
		if err := permRows.Scan(&p); err != nil {
			return nil, err
		}
		u.Permissions[p] = true
	}
	return &u, permRows.Err()
}

func (a *App) withUser(next func(http.ResponseWriter, *http.Request, *User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
		}
		u, err := a.currentUser(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if u.MustChangePassword {
			switch r.URL.Path {
			case "/account", "/account/password", "/logout":
			default:
				http.Redirect(w, r, "/account", http.StatusSeeOther)
				return
			}
		} else if !u.MFAPassed {
			switch r.URL.Path {
			case "/mfa/setup", "/mfa/qr.png", "/mfa/verify", "/logout":
			default:
				if u.MFAEnrolled {
					http.Redirect(w, r, "/mfa/verify", http.StatusSeeOther)
				} else {
					http.Redirect(w, r, "/mfa/setup", http.StatusSeeOther)
				}
				return
			}
		}
		if r.Method == http.MethodPost && !validCSRF(r, u.CSRFToken) {
			a.audit(r, u.EmployeeNo, "CSRF_REJECT", r.URL.Path, "403 DENIED", "CSRF token không hợp lệ hoặc đã hết hạn")
			http.Error(w, "403 — Phiên biểu mẫu đã hết hạn hoặc token không khớp. Nhấn F5 để tải lại trang rồi gửi lại.", http.StatusForbidden)
			return
		}
		next(w, r, u)
	}
}

func validCSRF(r *http.Request, expected string) bool {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(16 << 20); err != nil {
			return false
		}
	} else if err := r.ParseForm(); err != nil {
		return false
	}
	got := r.FormValue("_csrf")
	if got == "" || len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func (a *App) createSession(w http.ResponseWriter, userID int64, mfaPassed bool) error {
	token, err := randomToken(32)
	if err != nil {
		return err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return err
	}
	ttl := 12 * time.Hour
	if !mfaPassed {
		ttl = 10 * time.Minute
	}
	expires := time.Now().UTC().Add(ttl)
	mfaFlag := 0
	if mfaPassed {
		mfaFlag = 1
	}
	if _, err := a.DB.Exec(`INSERT INTO app_session(token_hash,user_id,csrf_token,expires_at,mfa_passed) VALUES(?,?,?,?,?)`,
		tokenHash(token), userID, csrf, expires.Format(time.RFC3339), mfaFlag); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: a.SessionCookie, Value: token, Path: "/",
		HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteStrictMode,
		Expires: expires,
	})
	return nil
}

func (a *App) markMFAPassed(w http.ResponseWriter, r *http.Request, u *User) error {
	c, err := r.Cookie(a.SessionCookie)
	if err != nil || c.Value == "" {
		return fmt.Errorf("missing session")
	}
	expires := time.Now().UTC().Add(12 * time.Hour)
	if _, err := a.DB.Exec(`UPDATE app_session SET mfa_passed=1, expires_at=? WHERE token_hash=?`,
		expires.Format(time.RFC3339), tokenHash(c.Value)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: a.SessionCookie, Value: c.Value, Path: "/",
		HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteStrictMode,
		Expires: expires,
	})
	u.MFAPassed = true
	return nil
}

func (a *App) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: a.SessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func (a *App) loginCSRF(w http.ResponseWriter, r *http.Request) string {
	token := ""
	if c, err := r.Cookie(a.LoginCSRFCookie); err == nil && len(c.Value) >= 32 {
		token = c.Value
	}
	if token == "" {
		var err error
		token, err = randomToken(24)
		if err != nil {
			return ""
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: a.LoginCSRFCookie, Value: token, Path: "/",
		HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteStrictMode,
		MaxAge: 7200,
	})
	return token
}

func (a *App) validLoginCSRF(r *http.Request) bool {
	c, err := r.Cookie(a.LoginCSRFCookie)
	if err != nil || c.Value == "" {
		return false
	}
	got := r.FormValue("_csrf")
	if got == "" || len(got) != len(c.Value) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(c.Value)) == 1
}

func (a *App) loginGet(w http.ResponseWriter, r *http.Request) {
	exists, err := a.usersExist()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if !exists {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if _, err := a.currentUser(r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.render(w, "login", PageData{Title: "Đăng nhập", LoginCSRF: a.loginCSRF(w, r)})
}

func (a *App) loginPost(w http.ResponseWriter, r *http.Request) {
	exists, err := a.usersExist()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if !exists {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if !a.validLoginCSRF(r) {
		a.audit(r, "", "LOGIN_CSRF_REJECT", "/login", "403 DENIED", "Login CSRF không hợp lệ hoặc đã hết hạn")
		a.render(w, "login", PageData{Title: "Đăng nhập", Error: "Phiên biểu mẫu đã hết hạn. Vui lòng thử lại.", LoginCSRF: a.loginCSRF(w, r), HTTPStatus: http.StatusForbidden})
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	limitKey := requestIP(r) + "|" + strings.ToLower(username)
	if !a.loginLimiter.allowed(limitKey) {
		a.audit(r, username, "LOGIN_THROTTLED", "/login", "429 DENIED", "Quá nhiều lần đăng nhập sai")
		a.render(w, "login", PageData{Title: "Đăng nhập", Error: "Quá nhiều lần thử. Vui lòng đợi 10 phút.", LoginCSRF: a.loginCSRF(w, r), HTTPStatus: http.StatusTooManyRequests})
		return
	}
	var id int64
	var hash, bootstrapHash, bootstrapExpires, lockedRaw string
	var mustChange, mfaEnrolled int
	err = a.DB.QueryRow(`
		SELECT u.id, u.password_hash, u.must_change_password, u.totp_confirmed, COALESCE(u.bootstrap_token_hash,''), COALESCE(u.bootstrap_expires_at,''),
		       COALESCE(u.locked_until,'')
		FROM app_user u
		JOIN md_employee e ON e.employee_no = u.employee_no AND e.active = 1
		WHERE u.username = ? COLLATE NOCASE AND u.active = 1`, username).Scan(&id, &hash, &mustChange, &mfaEnrolled, &bootstrapHash, &bootstrapExpires, &lockedRaw)
	if err == nil && lockedRaw != "" {
		if until, ok := parseAppTime(lockedRaw); ok && until.After(time.Now()) {
			a.audit(r, username, "LOGIN_LOCKED", "/login", "429 DENIED", "Tài khoản đang tạm khóa")
			a.render(w, "login", PageData{Title: "Đăng nhập", Error: "Quá nhiều lần thử. Vui lòng thử lại sau.", LoginCSRF: a.loginCSRF(w, r)})
			return
		}
	}
	inviteLogin := false
	ok := false
	if err == nil {
		if hash == "pending" {
			// ASVS 5.0 V6.4.1: only high-entropy, time-limited bootstrap credentials.
			ok = bootstrapHash != "" && bootstrapExpires > nowUTC() &&
				subtle.ConstantTimeCompare([]byte(tokenHash(password)), []byte(bootstrapHash)) == 1
			inviteLogin = ok
		} else {
			ok = security.VerifyPassword(hash, password)
		}
	}
	if err != nil || !ok {
		a.loginLimiter.fail(limitKey)
		if err == nil {
			_, _ = a.recordAuthFailure(id)
		}
		time.Sleep(300 * time.Millisecond)
		a.audit(r, username, "LOGIN_FAILED", "/login", "401 DENIED", "Sai tài khoản hoặc thông tin xác thực")
		a.render(w, "login", PageData{Title: "Đăng nhập", Error: "Sai tài khoản hoặc mật khẩu.", LoginCSRF: a.loginCSRF(w, r)})
		return
	}
	a.loginLimiter.reset(limitKey)
	_, _ = a.DB.Exec(`DELETE FROM app_session WHERE expires_at <= ?`, nowUTC())
	if err := a.createSession(w, id, false); err != nil {
		http.Error(w, "cannot create session", 500)
		return
	}
	var empNo string
	_ = a.DB.QueryRow(`SELECT employee_no FROM app_user WHERE id=?`, id).Scan(&empNo)
	action := "LOGIN_SUCCESS"
	detail := "Đăng nhập thành công"
	if inviteLogin {
		action = "LOGIN_BOOTSTRAP_INVITE"
		detail = "Đăng nhập bằng mã kích hoạt ngẫu nhiên có thời hạn; bắt buộc đổi mật khẩu"
	}
	a.audit(r, empNo, action, "/", "200 OK", detail)
	if inviteLogin || mustChange == 1 {
		http.Redirect(w, r, "/account?msg="+url.QueryEscape("Bạn đang dùng PIN/mật khẩu tạm. Vui lòng đặt mật khẩu mới."), http.StatusSeeOther)
		return
	}
	if mfaEnrolled == 1 {
		http.Redirect(w, r, "/mfa/verify", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/mfa/setup", http.StatusSeeOther)
}

func (a *App) logoutPost(w http.ResponseWriter, r *http.Request, u *User) {
	if c, err := r.Cookie(a.SessionCookie); err == nil {
		_, _ = a.DB.Exec(`DELETE FROM app_session WHERE token_hash = ?`, tokenHash(c.Value))
	}
	a.audit(r, u.EmployeeNo, "LOGOUT", "/", "200 OK", "")
	a.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) setupGet(w http.ResponseWriter, r *http.Request) {
	exists, err := a.usersExist()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if exists {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", LoginCSRF: a.loginCSRF(w, r)})
}

func (a *App) setupPost(w http.ResponseWriter, r *http.Request) {
	exists, err := a.usersExist()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if exists {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if !a.validLoginCSRF(r) {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Phiên biểu mẫu đã hết hạn. Vui lòng thử lại.", LoginCSRF: a.loginCSRF(w, r), HTTPStatus: http.StatusForbidden})
		return
	}

	deptCode := cleanCode(r.FormValue("department_code"))
	deptName := strings.TrimSpace(r.FormValue("department_name"))
	employeeNo := cleanCode(r.FormValue("employee_no"))
	fullName := strings.TrimSpace(r.FormValue("full_name"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if deptCode == "" {
		deptCode = "BOD"
	}
	if deptName == "" {
		deptName = "Ban Giám đốc & Kiểm toán Nội bộ"
	}
	if employeeNo == "" || fullName == "" || username == "" {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Thiếu trường bắt buộc.", LoginCSRF: a.loginCSRF(w, r)})
		return
	}
	if len(username) < 3 || strings.ContainsAny(username, " \t\r\n") {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Tên đăng nhập tối thiểu 3 ký tự, không có khoảng trắng.", LoginCSRF: a.loginCSRF(w, r)})
		return
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: err.Error(), LoginCSRF: a.loginCSRF(w, r)})
		return
	}
	csrf, err := randomToken(24)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`INSERT OR IGNORE INTO md_department(code,name,kind,sort_order) VALUES(?,?,'BOD',10)`, deptCode, deptName); err != nil {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Không tạo được phòng ban: " + err.Error(), LoginCSRF: a.loginCSRF(w, r)})
		return
	}
	var empCount int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM md_employee WHERE employee_no=?`, employeeNo).Scan(&empCount)
	if empCount == 0 {
		if _, err = tx.Exec(`INSERT INTO md_employee(employee_no,full_name,department_code,position_title) VALUES(?,?,?,?)`,
			employeeNo, fullName, deptCode, "Quản trị hệ thống"); err != nil {
			a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Không tạo được nhân viên: " + err.Error(), LoginCSRF: a.loginCSRF(w, r)})
			return
		}
	}
	res, err := tx.Exec(`INSERT INTO app_user(username,password_hash,employee_no,dept_scope,active,csrf_token) VALUES(?,?,?,'ALL',1,?)`, username, hash, employeeNo, csrf)
	if err != nil {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Không tạo được user: " + err.Error(), LoginCSRF: a.loginCSRF(w, r)})
		return
	}
	userID, _ := res.LastInsertId()
	if _, err = tx.Exec(`INSERT INTO app_user_role(user_id,role_code) VALUES(?, 'ADMIN')`, userID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := a.createSession(w, userID, false); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	a.audit(r, employeeNo, "SETUP_ADMIN", username, "200 OK", "Tạo tài khoản quản trị đầu tiên; chờ thiết lập MFA")
	http.Redirect(w, r, "/mfa/setup", http.StatusSeeOther)
}

func (a *App) accountPage(w http.ResponseWriter, r *http.Request, u *User) {
	d := PageData{User: u, Title: "Tài khoản", Active: "account"}
	applyFlash(&d, r)
	a.render(w, "account", d)
}

func (a *App) changePasswordPost(w http.ResponseWriter, r *http.Request, u *User) {
	current := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	var hash, bootstrapHash, bootstrapExpires string
	if err := a.DB.QueryRow(`
		SELECT password_hash, bootstrap_token_hash, bootstrap_expires_at
		FROM app_user WHERE id=?`, u.ID).Scan(&hash, &bootstrapHash, &bootstrapExpires); err != nil {
		http.Error(w, "user not found", 404)
		return
	}
	verified := false
	if hash == "pending" {
		verified = bootstrapHash != "" && bootstrapExpires > nowUTC() &&
			subtle.ConstantTimeCompare([]byte(tokenHash(current)), []byte(bootstrapHash)) == 1
	} else {
		verified = security.VerifyPassword(hash, current)
	}
	if !verified {
		a.audit(r, u.EmployeeNo, "PASSWORD_CHANGE", u.Username, "403 DENIED", "Mật khẩu hiện tại không đúng")
		a.render(w, "account", PageData{User: u, Title: "Tài khoản", Active: "account", Error: "Mật khẩu hiện tại không đúng."})
		return
	}
	newHash, err := security.HashPassword(newPassword)
	if err != nil {
		a.render(w, "account", PageData{User: u, Title: "Tài khoản", Active: "account", Error: err.Error()})
		return
	}
	csrf, err := randomToken(24)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if _, err := a.DB.Exec(`UPDATE app_user SET password_hash=?, must_change_password=0, csrf_token=?, bootstrap_token_hash='', bootstrap_expires_at='' WHERE id=?`, newHash, csrf, u.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if u.MFAPassed {
		_, _ = a.DB.Exec(`DELETE FROM app_session WHERE user_id = ?`, u.ID)
		a.audit(r, u.EmployeeNo, "PASSWORD_CHANGE", u.Username, "200 OK", "Đổi mật khẩu, thu hồi toàn bộ phiên")
		a.clearSessionCookie(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	// Partial session: keep this session for MFA enrolment/verification, revoke all others.
	if c, err := r.Cookie(a.SessionCookie); err == nil {
		_, _ = a.DB.Exec(`DELETE FROM app_session WHERE user_id=? AND token_hash<>?`, u.ID, tokenHash(c.Value))
	}
	a.audit(r, u.EmployeeNo, "PASSWORD_CHANGE", u.Username, "200 OK", "Đổi mật khẩu; chờ MFA")
	u.MustChangePassword = false
	if u.MFAEnrolled {
		http.Redirect(w, r, "/mfa/verify", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/mfa/setup", http.StatusSeeOther)
	}
}
