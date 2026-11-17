package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"chinfon-kaizen/internal/app"
)

func main() {
	logger := log.New(os.Stdout, "kaizen: ", log.LstdFlags|log.LUTC)

	addr := env("APP_ADDR", "127.0.0.1:8080")
	dataDir := env("APP_DATA_DIR", "data")
	dbPath := env("APP_DB", filepath.Join(dataDir, "kaizen.db"))
	uploadDir := env("APP_UPLOAD_DIR", filepath.Join(dataDir, "uploads"))
	cookieSecure := envBool("COOKIE_SECURE")
	seedDemo := envBoolDefault("APP_SEED_DEMO", false)

	cfg := app.Config{
		DBPath:       dbPath,
		UploadDir:    uploadDir,
		CookieSecure: cookieSecure,
		SeedDemo:     seedDemo,
		Logger:       logger,
	}

	if len(os.Args) > 1 && os.Args[1] == "issue-invite" {
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: kaizen issue-invite <username>")
			os.Exit(2)
		}
		cfg.SeedDemo = false
		application, err := app.New(cfg)
		if err != nil { logger.Fatal(err) }
		defer application.Close()
		token, err := application.IssueInvite(os.Args[2])
		if err != nil { logger.Fatal(err) }
		fmt.Printf("One-time bootstrap credential for %s (30 min, keep private): %s\n", os.Args[2], token)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "reset-mfa" {
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: kaizen reset-mfa <username>")
			os.Exit(2)
		}
		cfg.SeedDemo = false
		application, err := app.New(cfg)
		if err != nil {
			logger.Fatal(err)
		}
		defer application.Close()
		if err := application.ResetMFA(os.Args[2]); err != nil {
			logger.Fatal(err)
		}
		fmt.Printf("MFA reset for %s. The user must enrol again at next sign-in.\n", os.Args[2])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: kaizen reset-password <username>")
			os.Exit(2)
		}
		cfg.SeedDemo = false
		application, err := app.New(cfg)
		if err != nil {
			logger.Fatal(err)
		}
		defer application.Close()
		temp, err := application.ResetPassword(os.Args[2])
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Printf("Temporary password for %s: %s\nThe user must change it at next sign-in. MFA is unchanged.\n", os.Args[2], temp)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "import-staff" {
		resetPIN := false
		var path string
		var err error
		for _, arg := range os.Args[2:] {
			if arg == "--reset-pin" {
				resetPIN = true
			} else if strings.TrimSpace(arg) != "" {
				path = arg
			}
		}
		if path == "" {
			path, err = findStaffCSV(uploadDir)
			if err != nil {
				logger.Fatal(err)
			}
		}
		cfg.SeedDemo = false
		application, err := app.New(cfg)
		if err != nil {
			logger.Fatal(err)
		}
		defer application.Close()
		res, err := application.ImportStaffFile(path, resetPIN)
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Printf("File: %s\n", path)
		fmt.Printf("Rows: %d | skipped rows: %d | departments: %d | employees: %d | accounts created: %d | updated: %d | skipped (no mobile): %d\n",
			res.Total, res.SkippedRows, res.Departments, res.EmployeesUpserted, res.UsersCreated, res.UsersUpdated, res.AccountsSkipped)
		for _, e := range res.Errors {
			fmt.Printf("Row %d %s: %s\n", e.Row, e.EmpID, e.Reason)
		}
		if res.Notice != "" {
			fmt.Println(res.Notice)
		}
		if len(res.Errors) > 0 {
			os.Exit(1)
		}
		return
	}

	application, err := app.New(cfg)
	if err != nil {
		logger.Fatal(err)
	}
	defer application.Close()

	srv := &http.Server{
		Addr:              addr,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		logger.Printf("Chinfon Kaizen 360 listening on http://%s (db=%s)", addr, dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func findStaffCSV(dir string) (string, error) {
	for _, name := range []string{"staff.csv", "personnel.csv", "employees.csv", "nhan_su.csv", "nhansu.csv"} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read upload folder %s: %w", dir, err)
	}
	var csvs []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".csv") {
			csvs = append(csvs, e.Name())
		}
	}
	if len(csvs) == 1 {
		return filepath.Join(dir, csvs[0]), nil
	}
	if len(csvs) == 0 {
		return "", fmt.Errorf("no CSV found in %s; put staff.csv there or pass a path", dir)
	}
	return "", fmt.Errorf("multiple CSV files in %s (%s); pass a path", dir, strings.Join(csvs, ", "))
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool { return envBoolDefault(key, false) }

func envBoolDefault(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
