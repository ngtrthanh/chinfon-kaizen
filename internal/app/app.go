package app

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

//go:embed all:web
var webFS embed.FS

type templateSet = template.Template

func New(cfg Config) (*App, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	if strings.TrimSpace(cfg.DBPath) == "" {
		return nil, fmt.Errorf("DBPath is required")
	}
	if strings.TrimSpace(cfg.UploadDir) == "" {
		cfg.UploadDir = "data/uploads"
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite pragma %q: %w", p, err)
		}
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	funcs := template.FuncMap{
		"hasPerm":    func(u *User, p string) bool { return u != nil && u.HasPerm(p) },
		"hasRole":    func(u *User, role string) bool { return u != nil && u.HasRole(role) },
		"money":      formatMoney,
		"shortMoney": formatShortMoney,
		"shortHash":  shortHash,
		"date":       formatDate,
		"dateTime":   formatDateTime,
		"bytes":      formatBytes,
		"initial":    firstRune,
		"icon":       icon,
		"add":        func(a, b int) int { return a + b },
		"contains": func(list []string, v string) bool {
			for _, x := range list {
				if x == v {
					return true
				}
			}
			return false
		},
		"stageName": StageName,
		"trackName": TrackName,
	}
	tpl, err := template.New("root").Funcs(funcs).ParseFS(webFS, "web/templates/*.html")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	absDB, _ := filepath.Abs(cfg.DBPath)
	mfaKey, mfaKeyPath, err := loadMFAKey(absDB)
	if err != nil {
		db.Close()
		return nil, err
	}
	sum := sha256.Sum256([]byte(absDB))
	prefix := "kz_" + hex.EncodeToString(sum[:4])
	app := &App{
		DB:              db,
		Templates:       tpl,
		UploadDir:       cfg.UploadDir,
		CookieSecure:    cfg.CookieSecure,
		SeedDemo:        cfg.SeedDemo,
		Logger:          logger,
		loginLimiter:    newLoginLimiter(),
		SessionCookie:   prefix + "_session",
		LoginCSRFCookie: prefix + "_login_csrf",
		MFAKey:          mfaKey,
		MFAKeyPath:      mfaKeyPath,
	}
	if cfg.SeedDemo && demoMode(db) {
		if err := seedDemoData(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("seed demo: %w", err)
		}
	}
	return app, nil
}

func loadMFAKey(absDBPath string) ([]byte, string, error) {
	if raw := strings.TrimSpace(os.Getenv("APP_MFA_KEY")); raw != "" {
		key, err := decodeMFAKey(raw)
		if err != nil {
			return nil, "", fmt.Errorf("APP_MFA_KEY: %w", err)
		}
		return key, "APP_MFA_KEY", nil
	}
	path := filepath.Join(filepath.Dir(absDBPath), "mfa.key")
	if b, err := os.ReadFile(path); err == nil {
		if len(b) != 32 {
			return nil, "", fmt.Errorf("MFA key file %s must be exactly 32 bytes (got %d)", path, len(b))
		}
		return b, path, nil
	} else if !os.IsNotExist(err) {
		return nil, "", err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, "", fmt.Errorf("write MFA key: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return key, path, nil
}

func decodeMFAKey(raw string) ([]byte, error) {
	if len(raw) == 64 {
		if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(raw); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("must be 32 bytes encoded as base64 or 64 hex characters")
}

func demoMode(db *sql.DB) bool {
	var v string
	if err := db.QueryRow(`SELECT value FROM app_setting WHERE key='demo_mode'`).Scan(&v); err != nil {
		return true
	}
	return v == "1"
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	)`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, err := fs.ReadFile(migrationFS, "migrations/"+name)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`, name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) Close() error { return a.DB.Close() }

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(webFS, "web/static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /setup", a.setupGet)
	mux.HandleFunc("POST /setup", a.setupPost)
	mux.HandleFunc("GET /login", a.loginGet)
	mux.HandleFunc("POST /login", a.loginPost)
	mux.HandleFunc("GET /mfa/setup", a.withUser(a.mfaSetupGet))
	mux.HandleFunc("POST /mfa/setup", a.withUser(a.mfaSetupPost))
	mux.HandleFunc("GET /mfa/qr.png", a.withUser(a.mfaQRImage))
	mux.HandleFunc("GET /mfa/verify", a.withUser(a.mfaVerifyGet))
	mux.HandleFunc("POST /mfa/verify", a.withUser(a.mfaVerifyPost))

	mux.HandleFunc("POST /logout", a.withUser(a.logoutPost))
	mux.HandleFunc("GET /", a.withUser(a.dashboard))
	mux.HandleFunc("GET /initiatives", a.withUser(a.initiativesList))
	mux.HandleFunc("GET /initiatives/new", a.withUser(a.initiativeNewGet))
	mux.HandleFunc("POST /initiatives", a.withUser(a.initiativeCreatePost))
	mux.HandleFunc("GET /initiatives/{id}", a.withUser(a.initiativeDetail))
	mux.HandleFunc("GET /initiatives/{id}/edit", a.withUser(a.initiativeEditGet))
	mux.HandleFunc("POST /initiatives/{id}/edit", a.withUser(a.initiativeEditPost))
	mux.HandleFunc("POST /initiatives/{id}/submit", a.withUser(a.initiativeSubmitPost))
	mux.HandleFunc("POST /initiatives/{id}/review", a.withUser(a.initiativeReviewPost))
	mux.HandleFunc("POST /initiatives/{id}/participants", a.withUser(a.participantPost))
	mux.HandleFunc("POST /initiatives/{id}/attachments", a.withUser(a.attachmentPost))
	mux.HandleFunc("GET /attachments/{id}", a.withUser(a.attachmentDownload))
	mux.HandleFunc("GET /print/{id}", a.withUser(a.initiativePrint))
	mux.HandleFunc("GET /opl", a.withUser(a.oplPage))
	mux.HandleFunc("GET /rewards", a.withUser(a.rewardsPage))
	mux.HandleFunc("GET /rewards/export.csv", a.withUser(a.rewardsExport))
	mux.HandleFunc("GET /admin", a.withUser(a.adminPage))
	mux.HandleFunc("POST /admin/departments", a.withUser(a.adminDepartmentPost))
	mux.HandleFunc("POST /admin/departments/{code}/toggle", a.withUser(a.adminDepartmentToggle))
	mux.HandleFunc("POST /admin/employees", a.withUser(a.adminEmployeePost))
	mux.HandleFunc("POST /admin/employees/{no}/toggle", a.withUser(a.adminEmployeeToggle))
	mux.HandleFunc("POST /admin/users", a.withUser(a.adminUserCreate))
	mux.HandleFunc("POST /admin/users/{id}/update", a.withUser(a.adminUserUpdate))
	mux.HandleFunc("POST /admin/users/{id}/password", a.withUser(a.adminUserPassword))
	mux.HandleFunc("POST /admin/users/{id}/mfa-reset", a.withUser(a.adminMFAReset))
	mux.HandleFunc("POST /admin/rbac", a.withUser(a.adminRBACToggle))
	mux.HandleFunc("GET /admin/staff/template.csv", a.withUser(a.staffTemplate))
	mux.HandleFunc("GET /admin/staff/sample.csv", a.withUser(a.staffSample))
	mux.HandleFunc("GET /admin/staff/export.csv", a.withUser(a.staffExport))
	mux.HandleFunc("POST /admin/staff/import", a.withUser(a.staffImportPost))
	mux.HandleFunc("POST /admin/staff/department", a.withUser(a.adminStaffDepartment))
	mux.HandleFunc("POST /admin/staff/council", a.withUser(a.adminCouncilCreate))
	mux.HandleFunc("GET /account", a.withUser(a.accountPage))
	mux.HandleFunc("POST /account/password", a.withUser(a.changePasswordPost))

	return a.securityHeaders(recoverer(a.Logger, mux))
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		if a.CookieSecure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; "+
				"img-src 'self' data:; style-src 'self'; script-src 'self'; object-src 'none'")
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			w.Header().Set("Cache-Control", "no-store, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		next.ServeHTTP(w, r)
	})
}

func recoverer(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Printf("panic %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 2*time.Second)
	defer cancel()
	if err := a.DB.PingContext(ctx); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (a *App) render(w http.ResponseWriter, name string, data PageData) {
	var buf bytes.Buffer
	if err := a.Templates.ExecuteTemplate(&buf, name, data); err != nil {
		a.Logger.Printf("render %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data.HTTPStatus != 0 {
		w.WriteHeader(data.HTTPStatus)
	}
	_, _ = w.Write(buf.Bytes())
}

func (a *App) forbidden(w http.ResponseWriter, r *http.Request, u *User, action, target, detail string) {
	actor := ""
	if u != nil {
		actor = u.EmployeeNo
	}
	a.audit(r, actor, action, target, "403 DENIED", detail)
	http.Error(w, "403 — không có quyền hoặc ngoài phạm vi dữ liệu", http.StatusForbidden)
}

func icon(name string) template.HTML {
	paths := map[string]string{
		"dashboard": `<rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/>`,
		"flow":      `<circle cx="6" cy="6" r="3"/><circle cx="18" cy="18" r="3"/><path d="M6 9v3a3 3 0 0 0 3 3h3"/><path d="M15 12h3"/>`,
		"book":      `<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>`,
		"coins":     `<circle cx="8" cy="8" r="6"/><path d="M18.09 10.37A6 6 0 1 1 10.34 18"/><path d="M7 6h1v4"/><path d="m16.71 13.88.7.71-2.82 2.82"/>`,
		"settings":  `<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.38a2 2 0 0 0-.73-2.73l-.15-.09a2 2 0 0 1-1-1.74v-.51a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>`,
		"plus":      `<path d="M12 5v14M5 12h14"/>`,
		"search":    `<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`,
		"shield":    `<path d="M20 13c0 5-3.5 7.5-8 9-4.5-1.5-8-4-8-9V6l8-3 8 3z"/><path d="m9 12 2 2 4-4"/>`,
		"alert":     `<path d="m21.73 18-8-14a2 2 0 0 0-3.46 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3z"/><path d="M12 9v4M12 17h.01"/>`,
		"check":     `<path d="M20 6 9 17l-5-5"/>`,
		"x":         `<path d="M18 6 6 18M6 6l12 12"/>`,
		"clock":     `<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>`,
		"users":     `<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>`,
		"file":      `<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/><path d="M16 13H8M16 17H8M10 9H8"/>`,
		"download":  `<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m7 10 5 5 5-5"/><path d="M12 15V3"/>`,
		"arrow":     `<path d="M5 12h14M12 5l7 7-7 7"/>`,
		"logout":    `<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="m16 17 5-5-5-5"/><path d="M21 12H9"/>`,
		"eye":       `<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/>`,
		"lock":      `<rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>`,
		"trophy":    `<path d="M8 21h8M12 17v4"/><path d="M7 4h10v5a5 5 0 0 1-10 0z"/><path d="M7 5H4v2a3 3 0 0 0 3 3M17 5h3v2a3 3 0 0 1-3 3"/>`,
		"edit":      `<path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4z"/>`,
		"filter":    `<path d="M22 3H2l8 9.46V19l4 2v-8.54z"/>`,
	}
	body, ok := paths[name]
	if !ok {
		body = paths["file"]
	}
	return template.HTML(`<svg class="ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` + body + `</svg>`)
}
