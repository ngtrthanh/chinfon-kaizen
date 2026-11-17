package app

import (
	"net/http"
	"net/url"
	"strings"
)

// audit writes an append-only security event. It never fails the request:
// losing an audit row must not silently grant access, but the caller already
// made the authorization decision and the application logs the write failure.
func (a *App) audit(r *http.Request, actor, action, target, status, detail string) {
	if actor == "" {
		actor = "anonymous"
	}
	if _, err := a.DB.Exec(`
		INSERT INTO audit_log(at, actor, action_code, target, status, detail, ip)
		VALUES(?,?,?,?,?,?,?)`,
		nowUTC(), actor, action, target, status, detail, requestIP(r)); err != nil {
		a.Logger.Printf("audit write failed: %v", err)
	}
}

func applyFlash(pd *PageData, r *http.Request) {
	if v := strings.TrimSpace(r.URL.Query().Get("msg")); v != "" {
		pd.Message = v
	}
	if v := strings.TrimSpace(r.URL.Query().Get("err")); v != "" {
		pd.Error = v
	}
}

func flashRedirect(w http.ResponseWriter, r *http.Request, path, msg string) {
	if strings.TrimSpace(msg) == "" {
		http.Redirect(w, r, path, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, path+"?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}
