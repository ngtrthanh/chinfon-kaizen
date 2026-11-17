package app

import (
    "fmt"
    "strings"
    "time"
)

// IssueInvite replaces predictable phone-derived PINs with a short-lived
// cryptographically random bootstrap credential. It is never stored in plaintext.
// Run from the machine console with administrator access; distribute privately.
func (a *App) IssueInvite(username string) (string, error) {
    username = strings.TrimSpace(username)
    if username == "" { return "", fmt.Errorf("username is required") }
    token, err := randomToken(24)
    if err != nil { return "", err }
    expiry := time.Now().UTC().Add(30 * time.Minute).Format(time.RFC3339)
    res, err := a.DB.Exec(`UPDATE app_user
        SET bootstrap_token_hash=?, bootstrap_expires_at=?, failed_login_count=0, locked_until=NULL
        WHERE username=? COLLATE NOCASE AND password_hash='pending' AND active=1`,
        tokenHash(token), expiry, username)
    if err != nil { return "", err }
    n, err := res.RowsAffected()
    if err != nil { return "", err }
    if n != 1 { return "", fmt.Errorf("no active pending account for %q", username) }
    _, _ = a.DB.Exec(`DELETE FROM app_session WHERE user_id=(SELECT id FROM app_user WHERE username=? COLLATE NOCASE)`,username)
    _, _ = a.DB.Exec(`INSERT INTO audit_log(actor,action_code,target,status,detail)
        VALUES('CLI','BOOTSTRAP_ISSUED',?,'200 OK','Random credential, expiry 30 min; plaintext not stored')`,username)
    return token, nil
}
