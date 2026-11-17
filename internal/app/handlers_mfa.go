package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"

	"chinfon-kaizen/internal/mfa"
	"chinfon-kaizen/internal/security"
)

const mfaIssuer = "Chinfon Kaizen 360"

func (a *App) mfaSecret(userID int64) (string, bool, error) {
	var enc string
	var confirmed int
	if err := a.DB.QueryRow(`SELECT totp_secret_enc, totp_confirmed FROM app_user WHERE id=?`, userID).Scan(&enc, &confirmed); err != nil {
		return "", false, err
	}
	if strings.TrimSpace(enc) == "" {
		return "", confirmed == 1, nil
	}
	raw, err := mfa.Decrypt(a.MFAKey, enc)
	if err != nil {
		return "", confirmed == 1, err
	}
	return string(raw), confirmed == 1, nil
}

func (a *App) ensureMFASecret(userID int64) (string, error) {
	secret, confirmed, err := a.mfaSecret(userID)
	if err != nil {
		return "", err
	}
	if confirmed {
		return secret, nil
	}
	if secret != "" {
		return secret, nil
	}
	secret, err = mfa.NewSecret()
	if err != nil {
		return "", err
	}
	enc, err := mfa.Encrypt(a.MFAKey, []byte(secret))
	if err != nil {
		return "", err
	}
	if _, err := a.DB.Exec(`UPDATE app_user SET totp_secret_enc=?, totp_confirmed=0, last_totp_counter=0 WHERE id=?`, enc, userID); err != nil {
		return "", err
	}
	return secret, nil
}

func (a *App) authLocked(userID int64) (bool, time.Time) {
	var raw sql.NullString
	if err := a.DB.QueryRow(`SELECT locked_until FROM app_user WHERE id=?`, userID).Scan(&raw); err != nil || !raw.Valid || raw.String == "" {
		return false, time.Time{}
	}
	until, ok := parseAppTime(raw.String)
	if !ok {
		return false, time.Time{}
	}
	if until.After(time.Now()) {
		return true, until
	}
	_, _ = a.DB.Exec(`UPDATE app_user SET failed_login_count=0, locked_until=NULL WHERE id=?`, userID)
	return false, time.Time{}
}

func (a *App) recordAuthFailure(userID int64) (time.Time, error) {
	var count int
	if err := a.DB.QueryRow(`UPDATE app_user SET failed_login_count=failed_login_count+1 WHERE id=? RETURNING failed_login_count`, userID).Scan(&count); err != nil {
		return time.Time{}, err
	}
	if count < 5 {
		return time.Time{}, nil
	}
	step := count - 5
	if step > 4 {
		step = 4
	}
	minutes := 1 << step
	if minutes > 15 {
		minutes = 15
	}
	until := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	if _, err := a.DB.Exec(`UPDATE app_user SET locked_until=? WHERE id=?`, until.Format(time.RFC3339), userID); err != nil {
		return time.Time{}, err
	}
	return until, nil
}

func (a *App) clearAuthFailures(userID int64) {
	_, _ = a.DB.Exec(`UPDATE app_user SET failed_login_count=0, locked_until=NULL WHERE id=?`, userID)
}

func (a *App) mfaSetupPage(w http.ResponseWriter, u *User, secret, errMsg string) {
	uri := mfa.ProvisioningURI(mfaIssuer, u.EmployeeNo, secret)
	d := PageData{
		User:         u,
		Title:        "Thiết lập MFA",
		MFASecret:    secret,
		MFAURI:       uri,
		MFARemaining: mfa.RemainingSeconds(time.Now()),
		Error:        errMsg,
	}
	a.render(w, "mfa_setup", d)
}

func (a *App) mfaQRImage(w http.ResponseWriter, r *http.Request, u *User) {
	secret, _, err := a.mfaSecret(u.ID)
	if err != nil || secret == "" {
		http.NotFound(w, r)
		return
	}
	uri := mfa.ProvisioningURI(mfaIssuer, u.EmployeeNo, secret)
	png, err := qrcode.Encode(uri, qrcode.Medium, 280)
	if err != nil {
		http.Error(w, "cannot generate QR", 500)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

func (a *App) mfaSetupGet(w http.ResponseWriter, r *http.Request, u *User) {
	if u.MFAPassed {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	secret, confirmed, err := a.mfaSecret(u.ID)
	if err != nil {
		http.Error(w, "Không đọc được cấu hình MFA. Liên hệ ADMIN.", 500)
		return
	}
	if confirmed {
		http.Redirect(w, r, "/mfa/verify", http.StatusSeeOther)
		return
	}
	if secret == "" {
		secret, err = a.ensureMFASecret(u.ID)
		if err != nil {
			http.Error(w, "Không tạo được MFA secret.", 500)
			return
		}
	}
	d := PageData{
		User:         u,
		Title:        "Thiết lập MFA",
		MFASecret:    secret,
		MFAURI:       mfa.ProvisioningURI(mfaIssuer, u.EmployeeNo, secret),
		MFARemaining: mfa.RemainingSeconds(time.Now()),
	}
	a.render(w, "mfa_setup", d)
}

func (a *App) mfaSetupPost(w http.ResponseWriter, r *http.Request, u *User) {
	if u.MFAPassed {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if locked, until := a.authLocked(u.ID); locked {
		a.audit(r, u.EmployeeNo, "MFA_LOCKED", u.Username, "429 DENIED", "Tài khoản đang tạm khóa")
		secret, _, _ := a.mfaSecret(u.ID)
		if secret == "" {
			secret, _ = a.ensureMFASecret(u.ID)
		}
		a.mfaSetupPage(w, u, secret, "Tài khoản tạm khóa đến "+until.Local().Format("15:04:05")+".")
		return
	}
	secret, confirmed, err := a.mfaSecret(u.ID)
	if err != nil {
		http.Error(w, "Không đọc được cấu hình MFA.", 500)
		return
	}
	if confirmed {
		http.Redirect(w, r, "/mfa/verify", http.StatusSeeOther)
		return
	}
	if secret == "" {
		secret, err = a.ensureMFASecret(u.ID)
		if err != nil {
			http.Error(w, "Không tạo được MFA secret.", 500)
			return
		}
	}
	code := strings.TrimSpace(r.FormValue("code"))
	counter, ok := mfa.Validate(secret, code, time.Now(), 1)
	if !ok {
		until, _ := a.recordAuthFailure(u.ID)
		msg := "Mã xác thực không đúng. Kiểm tra đồng hồ điện thoại và thử lại."
		if !until.IsZero() {
			msg = "Sai quá nhiều lần. Tạm khóa đến " + until.Local().Format("15:04:05") + "."
		}
		a.audit(r, u.EmployeeNo, "MFA_ENROLL_FAILED", u.Username, "401 DENIED", "Mã TOTP không đúng")
		a.mfaSetupPage(w, u, secret, msg)
		return
	}
	if _, err := a.DB.Exec(`UPDATE app_user SET totp_confirmed=1, last_totp_counter=?, failed_login_count=0, locked_until=NULL WHERE id=?`, counter, u.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := a.markMFAPassed(w, r, u); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	a.audit(r, u.EmployeeNo, "MFA_ENROLLED", u.Username, "200 OK", "Đã xác nhận TOTP")
	flashRedirect(w, r, "/", "Đã bật MFA. Từ nay mỗi lần đăng nhập cần mã 6 số.")
}

func (a *App) mfaVerifyPage(w http.ResponseWriter, u *User, errMsg string) {
	a.render(w, "mfa_verify", PageData{
		User:         u,
		Title:        "Xác thực MFA",
		MFARemaining: mfa.RemainingSeconds(time.Now()),
		Error:        errMsg,
	})
}

func (a *App) mfaVerifyGet(w http.ResponseWriter, r *http.Request, u *User) {
	if u.MFAPassed {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !u.MFAEnrolled {
		http.Redirect(w, r, "/mfa/setup", http.StatusSeeOther)
		return
	}
	a.mfaVerifyPage(w, u, "")
}

func (a *App) mfaVerifyPost(w http.ResponseWriter, r *http.Request, u *User) {
	if u.MFAPassed {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !u.MFAEnrolled {
		http.Redirect(w, r, "/mfa/setup", http.StatusSeeOther)
		return
	}
	if locked, until := a.authLocked(u.ID); locked {
		a.audit(r, u.EmployeeNo, "MFA_LOCKED", u.Username, "429 DENIED", "Tài khoản đang tạm khóa")
		a.mfaVerifyPage(w, u, "Tài khoản tạm khóa đến "+until.Local().Format("15:04:05")+".")
		return
	}
	secret, confirmed, err := a.mfaSecret(u.ID)
	if err != nil || !confirmed {
		http.Redirect(w, r, "/mfa/setup", http.StatusSeeOther)
		return
	}
	var lastCounter int64
	_ = a.DB.QueryRow(`SELECT last_totp_counter FROM app_user WHERE id=?`, u.ID).Scan(&lastCounter)
	code := strings.TrimSpace(r.FormValue("code"))
	counter, ok := mfa.Validate(secret, code, time.Now(), 1)
	if !ok || counter <= lastCounter {
		until, _ := a.recordAuthFailure(u.ID)
		msg := "Mã không đúng hoặc đã được dùng. Thử mã tiếp theo."
		if !until.IsZero() {
			msg = "Sai quá nhiều lần. Tạm khóa đến " + until.Local().Format("15:04:05") + "."
		}
		a.audit(r, u.EmployeeNo, "MFA_VERIFY_FAILED", u.Username, "401 DENIED", "Mã sai hoặc replay")
		a.mfaVerifyPage(w, u, msg)
		return
	}
	if _, err := a.DB.Exec(`UPDATE app_user SET last_totp_counter=?, failed_login_count=0, locked_until=NULL WHERE id=?`, counter, u.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := a.markMFAPassed(w, r, u); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	a.audit(r, u.EmployeeNo, "MFA_VERIFIED", u.Username, "200 OK", fmt.Sprintf("TOTP counter=%d", counter))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) adminMFAReset(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/users/mfa-reset") {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		adminRedirect(w, r, "users", "", "Tài khoản không hợp lệ.")
		return
	}
	if id == u.ID {
		adminRedirect(w, r, "users", "", "Không thể tự reset MFA của chính mình; dùng tài khoản ADMIN khác hoặc CLI.")
		return
	}
	var username string
	if err := a.DB.QueryRow(`SELECT username FROM app_user WHERE id=?`, id).Scan(&username); err != nil {
		adminRedirect(w, r, "users", "", "Không tìm thấy tài khoản.")
		return
	}
	if _, err := a.DB.Exec(`
		UPDATE app_user SET totp_secret_enc='', totp_confirmed=0, last_totp_counter=0,
			failed_login_count=0, locked_until=NULL WHERE id=?`, id); err != nil {
		adminRedirect(w, r, "users", "", err.Error())
		return
	}
	_, _ = a.DB.Exec(`DELETE FROM app_session WHERE user_id=?`, id)
	a.audit(r, u.EmployeeNo, "MFA_RESET", username, "200 OK", "ADMIN reset MFA; người dùng phải thiết lập lại")
	adminRedirect(w, r, "users", "Đã reset MFA cho "+username+". Người dùng phải thiết lập lại khi đăng nhập.", "")
}

// ResetMFA is the break-glass CLI recovery path when no Admin can sign in.
func (a *App) ResetMFA(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	var id int64
	if err := a.DB.QueryRow(`SELECT id FROM app_user WHERE username=? COLLATE NOCASE`, username).Scan(&id); err != nil {
		return fmt.Errorf("account %q not found", username)
	}
	if _, err := a.DB.Exec(`
		UPDATE app_user SET totp_secret_enc='', totp_confirmed=0, last_totp_counter=0,
			failed_login_count=0, locked_until=NULL WHERE id=?`, id); err != nil {
		return err
	}
	if _, err := a.DB.Exec(`DELETE FROM app_session WHERE user_id=?`, id); err != nil {
		return err
	}
	_, _ = a.DB.Exec(`INSERT INTO audit_log(actor,action_code,target,status,detail) VALUES('CLI','MFA_RESET',?, '200 OK','Break-glass CLI reset')`, username)
	return nil
}

// ResetPassword is the break-glass CLI path. It returns a one-time temporary
// password; the user must change it at next sign-in. MFA is kept.
func (a *App) ResetPassword(username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	var id int64
	if err := a.DB.QueryRow(`SELECT id FROM app_user WHERE username=? COLLATE NOCASE`, username).Scan(&id); err != nil {
		return "", fmt.Errorf("account %q not found", username)
	}
	token, err := randomToken(9)
	if err != nil {
		return "", err
	}
	temp := "Tmp-" + token[:12]
	hash, err := security.HashPassword(temp)
	if err != nil {
		return "", err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return "", err
	}
	if _, err := a.DB.Exec(`UPDATE app_user SET password_hash=?, must_change_password=1, csrf_token=?, failed_login_count=0, locked_until=NULL WHERE id=?`, hash, csrf, id); err != nil {
		return "", err
	}
	if _, err := a.DB.Exec(`DELETE FROM app_session WHERE user_id=?`, id); err != nil {
		return "", err
	}
	_, _ = a.DB.Exec(`INSERT INTO audit_log(actor,action_code,target,status,detail) VALUES('CLI','PASSWORD_RESET',?, '200 OK','Break-glass CLI temporary password')`, username)
	return temp, nil
}
